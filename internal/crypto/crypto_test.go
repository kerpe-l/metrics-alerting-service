package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

// genKey генерирует тестовый ключ. 2048 бит достаточно и быстро для тестов.
func genKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("генерация ключа: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := genKey(t)

	cases := []struct {
		name string
		data []byte
	}{
		{"пустые данные", []byte{}},
		{"короткие данные", []byte("hello")},
		{"данные больше размера RSA-блока", bytes.Repeat([]byte("metric"), 1000)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := Encrypt(&key.PublicKey, tc.data)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			dec, err := Decrypt(key, enc)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(dec, tc.data) {
				t.Errorf("получено %q, ожидалось %q", dec, tc.data)
			}
		})
	}
}

func TestDecryptWrongKey(t *testing.T) {
	enc, err := Encrypt(&genKey(t).PublicKey, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt(genKey(t), enc); err == nil {
		t.Error("ожидалась ошибка при расшифровке чужим ключом")
	}
}

func TestDecryptMalformed(t *testing.T) {
	key := genKey(t)
	cases := []struct {
		name string
		data []byte
	}{
		{"слишком короткое", []byte{0x01}},
		{"длина ключа больше данных", []byte{0xff, 0xff, 0x00}},
		{"мусор", bytes.Repeat([]byte{0x00}, 64)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decrypt(key, tc.data); err == nil {
				t.Error("ожидалась ошибка")
			}
		})
	}
}
