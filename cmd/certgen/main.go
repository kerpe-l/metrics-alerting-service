// Command certgen генерирует самоподписанный CA и серверный TLS-сертификат для gRPC.
//
// CA-сертификат (ca.crt) отдаётся агенту для проверки сервера, серверные cert/key
// (server.crt, server.key) — серверу. Пример:
//
//	go run ./cmd/certgen -ca ca.crt -cert server.crt -key server.key -host localhost,127.0.0.1
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	caPath := flag.String("ca", "ca.crt", "путь для записи CA-сертификата")
	certPath := flag.String("cert", "server.crt", "путь для записи серверного сертификата")
	keyPath := flag.String("key", "server.key", "путь для записи серверного приватного ключа")
	hosts := flag.String("host", "localhost,127.0.0.1", "SAN-имена (hostname/IP) через запятую")
	days := flag.Int("days", 365, "срок действия сертификатов в днях")
	flag.Parse()

	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, *days)

	// Самоподписанный CA.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("генерация ключа CA: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "metrics-alerting CA"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		log.Fatalf("создание CA-сертификата: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		log.Fatalf("разбор CA-сертификата: %v", err)
	}

	// Серверный сертификат, подписанный CA.
	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("генерация ключа сервера: %v", err)
	}
	srvTemplate := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: "metrics-alerting server"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range strings.Split(*hosts, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			srvTemplate.IPAddresses = append(srvTemplate.IPAddresses, ip)
		} else {
			srvTemplate.DNSNames = append(srvTemplate.DNSNames, h)
		}
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTemplate, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		log.Fatalf("создание серверного сертификата: %v", err)
	}

	if err := writePEM(*caPath, "CERTIFICATE", caDER, 0o644); err != nil {
		log.Fatalf("запись CA-сертификата: %v", err)
	}
	if err := writePEM(*certPath, "CERTIFICATE", srvDER, 0o644); err != nil {
		log.Fatalf("запись серверного сертификата: %v", err)
	}
	srvKeyDER, err := x509.MarshalPKCS8PrivateKey(srvKey)
	if err != nil {
		log.Fatalf("сериализация ключа сервера: %v", err)
	}
	if err := writePEM(*keyPath, "PRIVATE KEY", srvKeyDER, 0o600); err != nil {
		log.Fatalf("запись ключа сервера: %v", err)
	}

	log.Printf("сертификаты записаны: %s (CA), %s + %s (сервер)", *caPath, *certPath, *keyPath)
}

// serial генерирует случайный 128-битный серийный номер сертификата.
func serial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		log.Fatalf("генерация серийного номера: %v", err)
	}
	return n
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
