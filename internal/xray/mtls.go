package xray

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

const InternalServerName = "internal.remnawave.local"

type InternalMTLSBundle struct {
	CACertPEM     string
	CAKeyPEM      string
	ServerCertPEM string
	ServerKeyPEM  string
	ClientCertPEM string
	ClientKeyPEM  string
}

func NewInternalMTLSBundle() (InternalMTLSBundle, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return InternalMTLSBundle{}, fmt.Errorf("generate internal CA key: %w", err)
	}
	caTemplate, err := certTemplate("Remnawave Internal CA", true, nil)
	if err != nil {
		return InternalMTLSBundle{}, err
	}
	caTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	caTemplate.ExtKeyUsage = nil
	caTemplate.NotAfter = time.Now().AddDate(10, 0, 0)

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return InternalMTLSBundle{}, fmt.Errorf("create internal CA certificate: %w", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return InternalMTLSBundle{}, fmt.Errorf("generate internal server key: %w", err)
	}
	serverTemplate, err := certTemplate(InternalServerName, false, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	if err != nil {
		return InternalMTLSBundle{}, err
	}
	serverTemplate.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		return InternalMTLSBundle{}, fmt.Errorf("create internal server certificate: %w", err)
	}

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return InternalMTLSBundle{}, fmt.Errorf("generate internal client key: %w", err)
	}
	clientTemplate, err := certTemplate(InternalServerName, false, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	if err != nil {
		return InternalMTLSBundle{}, err
	}
	clientTemplate.KeyUsage = x509.KeyUsageDigitalSignature
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		return InternalMTLSBundle{}, fmt.Errorf("create internal client certificate: %w", err)
	}

	return InternalMTLSBundle{
		CACertPEM:     encodeCertificatePEM(caDER),
		CAKeyPEM:      encodePrivateKeyPEM(caKey),
		ServerCertPEM: encodeCertificatePEM(serverDER),
		ServerKeyPEM:  encodePrivateKeyPEM(serverKey),
		ClientCertPEM: encodeCertificatePEM(clientDER),
		ClientKeyPEM:  encodePrivateKeyPEM(clientKey),
	}, nil
}

func certTemplate(commonName string, isCA bool, usages []x509.ExtKeyUsage) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}
	return &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		ExtKeyUsage:           usages,
		DNSNames:              []string{commonName},
	}, nil
}

func encodeCertificatePEM(der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func encodePrivateKeyPEM(key *rsa.PrivateKey) string {
	bytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: bytes}))
}
