// Command keygen генерирует пару RSA-ключей для шифрования трафика агент→сервер.
//
// Приватный ключ (PKCS8) передаётся серверу через -crypto-key, публичный (PKIX) —
// агенту. Пример:
//
//	go run ./cmd/keygen -priv private.pem -pub public.pem -bits 4096
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"log"
	"os"
)

func main() {
	privPath := flag.String("priv", "private.pem", "путь для записи приватного ключа")
	pubPath := flag.String("pub", "public.pem", "путь для записи публичного ключа")
	bits := flag.Int("bits", 4096, "размер ключа RSA в битах")
	flag.Parse()

	key, err := rsa.GenerateKey(rand.Reader, *bits)
	if err != nil {
		log.Fatalf("генерация ключа: %v", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		log.Fatalf("сериализация приватного ключа: %v", err)
	}
	if err := writePEM(*privPath, "PRIVATE KEY", privDER, 0o600); err != nil {
		log.Fatalf("запись приватного ключа: %v", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		log.Fatalf("сериализация публичного ключа: %v", err)
	}
	if err := writePEM(*pubPath, "PUBLIC KEY", pubDER, 0o644); err != nil {
		log.Fatalf("запись публичного ключа: %v", err)
	}

	log.Printf("ключи записаны: %s (приватный), %s (публичный)", *privPath, *pubPath)
}

// writePEM кодирует der в PEM-блок типа blockType и пишет в файл с правами perm.
func writePEM(path, blockType string, der []byte, perm os.FileMode) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: der})
}
