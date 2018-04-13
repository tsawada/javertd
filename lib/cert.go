package lib

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"time"
)

func SelfSigned(hostname string) ([]byte, crypto.PrivateKey) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{Organization: []string{"Self-signed"}},
		NotBefore:    now,
		NotAfter:     now.Add(7 * 24 * time.Hour), // 1 day
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	cert.DNSNames = append(cert.DNSNames, hostname)
	publicKey := &privKey.PublicKey
	derBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, publicKey, privKey)
	if err != nil {
		log.Fatal(err)
	}
	return derBytes, privKey
}

func PrivToPem(priv crypto.PrivateKey) []byte {
	var pemBlock *pem.Block
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		pemBlock = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			log.Fatal(err)
		}
		pemBlock = &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
	return pem.EncodeToMemory(pemBlock)
}

func CertToPem(cert []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
}
