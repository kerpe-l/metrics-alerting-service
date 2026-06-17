package crypto

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// writeKeys генерирует пару ключей, пишет их в PEM-файлы во временной директории
// и возвращает пути (приватный, публичный).
func writeKeys(t *testing.T) (privPath, pubPath string) {
	t.Helper()
	key := genKey(t)
	dir := t.TempDir()

	privPath = filepath.Join(dir, "priv.pem")
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal priv: %v", err)
	}
	writePEMFile(t, privPath, "PRIVATE KEY", privDER)

	pubPath = filepath.Join(dir, "pub.pem")
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	writePEMFile(t, pubPath, "PUBLIC KEY", pubDER)

	return privPath, pubPath
}

func writePEMFile(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	data := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadKeys(t *testing.T) {
	privPath, pubPath := writeKeys(t)

	if _, err := LoadPrivateKey(privPath); err != nil {
		t.Errorf("LoadPrivateKey: %v", err)
	}
	if _, err := LoadPublicKey(pubPath); err != nil {
		t.Errorf("LoadPublicKey: %v", err)
	}
}

func TestLoadKeysErrors(t *testing.T) {
	t.Run("несуществующий файл", func(t *testing.T) {
		if _, err := LoadPrivateKey("/no/such/file"); err == nil {
			t.Error("ожидалась ошибка")
		}
	})
	t.Run("не PEM", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "junk")
		if err := os.WriteFile(p, []byte("not a pem"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPublicKey(p); err == nil {
			t.Error("ожидалась ошибка")
		}
	})
}

func TestMiddleware(t *testing.T) {
	key := genKey(t)
	plaintext := []byte("encrypted body")

	// Конечный обработчик возвращает прочитанное тело — так проверяем расшифровку.
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_, _ = w.Write(body)
	})

	t.Run("расшифровывает помеченный запрос", func(t *testing.T) {
		enc, err := Encrypt(&key.PublicKey, plaintext)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(enc))
		req.Header.Set(HeaderEncrypted, "1")

		Middleware(key)(echo).ServeHTTP(rec, req)

		if got := rec.Body.String(); got != string(plaintext) {
			t.Errorf("получено %q, ожидалось %q", got, plaintext)
		}
	})

	t.Run("без заголовка проходит насквозь", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(plaintext))

		Middleware(key)(echo).ServeHTTP(rec, req)

		if got := rec.Body.String(); got != string(plaintext) {
			t.Errorf("получено %q, ожидалось %q", got, plaintext)
		}
	})

	t.Run("nil-ключ проходит насквозь", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(plaintext))
		req.Header.Set(HeaderEncrypted, "1")

		Middleware(nil)(echo).ServeHTTP(rec, req)

		if got := rec.Body.String(); got != string(plaintext) {
			t.Errorf("получено %q, ожидалось %q", got, plaintext)
		}
	})

	t.Run("битое тело отвергается", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("garbage")))
		req.Header.Set(HeaderEncrypted, "1")

		Middleware(key)(echo).ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("получен код %d, ожидался 400", rec.Code)
		}
	})
}
