package testkit

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

type CertBundle struct {
	Payload       config.NodePayload
	PanelCertPEM  string
	PanelKeyPEM   string
	JWTPrivateKey *rsa.PrivateKey
}

func NewCertBundle(t *testing.T) CertBundle {
	t.Helper()

	caKey := newRSAKey(t)
	caTemplate := certTemplate(t, "rw-node-go test CA", true)
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}

	nodeKey := newRSAKey(t)
	nodeTemplate := certTemplate(t, "rw-node-go node", false)
	nodeTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	nodeTemplate.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	nodeDER, err := x509.CreateCertificate(rand.Reader, nodeTemplate, caTemplate, &nodeKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create node certificate: %v", err)
	}

	panelKey := newRSAKey(t)
	panelTemplate := certTemplate(t, "rw-node-go panel", false)
	panelTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	panelDER, err := x509.CreateCertificate(rand.Reader, panelTemplate, caTemplate, &panelKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create panel certificate: %v", err)
	}

	jwtKey := newRSAKey(t)
	jwtPubDER, err := x509.MarshalPKIXPublicKey(&jwtKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal JWT public key: %v", err)
	}

	return CertBundle{
		Payload: config.NodePayload{
			CACertPEM:    encodePEM("CERTIFICATE", caDER),
			JWTPublicKey: encodePEM("PUBLIC KEY", jwtPubDER),
			NodeCertPEM:  encodePEM("CERTIFICATE", nodeDER),
			NodeKeyPEM:   encodePEM("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(nodeKey)),
		},
		PanelCertPEM:  encodePEM("CERTIFICATE", panelDER),
		PanelKeyPEM:   encodePEM("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(panelKey)),
		JWTPrivateKey: jwtKey,
	}
}

func certTemplate(t *testing.T, commonName string, isCA bool) *x509.Certificate {
	t.Helper()
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}
	return &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
}

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

func encodePEM(blockType string, bytes []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: bytes}))
}
