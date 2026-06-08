// Package crypto реализует гибридное шифрование трафика агент→сервер.
//
// RSA шифрует данные не больше размера ключа, поэтому тело запроса шифруется
// симметрично (AES-256-GCM) случайным сеансовым ключом, а сам сеансовый ключ —
// асимметрично (RSA-OAEP) публичным ключом сервера. Формат зашифрованного тела:
//
//	uint16(len(encKey)) || encKey || nonce||ciphertext
//
// где encKey — сеансовый AES-ключ под RSA-OAEP, а nonce||ciphertext — результат
// aes-gcm Seal (nonce записан в начало по соглашению Seal).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// aesKeySize — размер сеансового ключа AES-256 в байтах.
const aesKeySize = 32

// errMalformed — общая ошибка для повреждённого или неполного зашифрованного тела.
var errMalformed = errors.New("malformed encrypted payload")

// LoadPublicKey читает RSA-публичный ключ из PEM-файла (формат PKIX или PKCS1).
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}

	// Сначала пробуем PKIX (BEGIN PUBLIC KEY), затем PKCS1 (BEGIN RSA PUBLIC KEY).
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("ключ %s не является RSA", path)
		}
		return rsaPub, nil
	}
	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("разбор публичного ключа %s: %w", path, err)
	}
	return rsaPub, nil
}

// LoadPrivateKey читает RSA-приватный ключ из PEM-файла (формат PKCS8 или PKCS1).
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}

	// Сначала пробуем PKCS8 (BEGIN PRIVATE KEY), затем PKCS1 (BEGIN RSA PRIVATE KEY).
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("ключ %s не является RSA", path)
		}
		return rsaKey, nil
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("разбор приватного ключа %s: %w", path, err)
	}
	return rsaKey, nil
}

// readPEM читает файл и декодирует первый PEM-блок.
func readPEM(path string) (*pem.Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение ключа %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("файл %s не содержит PEM-блок", path)
	}
	return block, nil
}

// Encrypt шифрует plaintext гибридной схемой публичным ключом pub.
func Encrypt(pub *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	// Случайный сеансовый ключ AES-256.
	aesKey := make([]byte, aesKeySize)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("генерация сеансового ключа: %w", err)
	}

	gcm, err := newGCM(aesKey)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("генерация nonce: %w", err)
	}
	// Seal дописывает ciphertext к nonce: на выходе nonce||ciphertext.
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)

	// Шифруем сеансовый ключ RSA-OAEP.
	encKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("шифрование сеансового ключа: %w", err)
	}

	out := make([]byte, 2+len(encKey)+len(sealed))
	binary.BigEndian.PutUint16(out, uint16(len(encKey)))
	n := copy(out[2:], encKey)
	copy(out[2+n:], sealed)
	return out, nil
}

// Decrypt расшифровывает данные, сформированные Encrypt, приватным ключом priv.
func Decrypt(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	if len(data) < 2 {
		return nil, errMalformed
	}
	keyLen := int(binary.BigEndian.Uint16(data))
	if len(data) < 2+keyLen {
		return nil, errMalformed
	}
	encKey := data[2 : 2+keyLen]
	sealed := data[2+keyLen:]

	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, encKey, nil)
	if err != nil {
		return nil, fmt.Errorf("расшифровка сеансового ключа: %w", err)
	}

	gcm, err := newGCM(aesKey)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errMalformed
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("расшифровка тела: %w", err)
	}
	return plaintext, nil
}

// newGCM создаёт AES-GCM для сеансового ключа.
func newGCM(aesKey []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("инициализация AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("инициализация GCM: %w", err)
	}
	return gcm, nil
}
