package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

type certMaterial struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
	pem  string
}

type payloadOptions struct {
	caNotBefore time.Time
	caNotAfter  time.Time
	// wrongSigner issues the node certificate with a second CA instead.
	wrongSigner bool
	// keyMismatch pairs the node certificate with a different node key.
	keyMismatch bool
	// jwtKeyAsPEM overrides the JWT public key PEM.
	jwtKeyAsPEM string
}

func defaultPayloadOptions() payloadOptions {
	return payloadOptions{
		caNotBefore: time.Now().Add(-time.Hour),
		caNotAfter:  time.Now().Add(time.Hour),
	}
}

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func newCA(t *testing.T, key *rsa.PrivateKey, notBefore, notAfter time.Time) certMaterial {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA: %v", err)
	}
	return certMaterial{key: key, cert: cert, pem: encodeTestPEM("CERTIFICATE", der)}
}

func issueNodeCert(t *testing.T, key *rsa.PrivateKey, ca certMaterial) certMaterial {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-node"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("issue node cert: %v", err)
	}
	return certMaterial{key: key, pem: encodeTestPEM("CERTIFICATE", der)}
}

func encodeTestPEM(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}

func buildPayload(t *testing.T, opts payloadOptions) config.NodePayload {
	t.Helper()
	caKey := newKey(t)
	ca := newCA(t, caKey, opts.caNotBefore, opts.caNotAfter)

	nodeKey := newKey(t)
	node := issueNodeCert(t, nodeKey, ca)

	if opts.wrongSigner {
		otherCAKey := newKey(t)
		otherCA := newCA(t, otherCAKey, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
		node = issueNodeCert(t, nodeKey, otherCA)
	}

	payloadKey := nodeKey
	if opts.keyMismatch {
		payloadKey = newKey(t)
	}

	jwtKey := newKey(t)
	jwtPubDER, err := x509.MarshalPKIXPublicKey(&jwtKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal jwt public key: %v", err)
	}
	jwtPEM := encodeTestPEM("PUBLIC KEY", jwtPubDER)
	if opts.jwtKeyAsPEM != "" {
		jwtPEM = opts.jwtKeyAsPEM
	}

	return config.NodePayload{
		CACertPEM:    ca.pem,
		JWTPublicKey: jwtPEM,
		NodeCertPEM:  node.pem,
		NodeKeyPEM:   encodeTestPEM("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(payloadKey)),
	}
}

func TestValidateNodePayloadAcceptsValidBundle(t *testing.T) {
	payload := buildPayload(t, defaultPayloadOptions())

	checks, err := config.ValidateNodePayload(payload)
	if err != nil {
		t.Fatalf("err = %v; checks = %#v", err, checks)
	}
	for _, check := range checks {
		if !check.OK {
			t.Fatalf("check %s failed: %s", check.Name, check.Detail)
		}
	}
}

func TestValidateNodePayloadRejectsExpiredCA(t *testing.T) {
	opts := defaultPayloadOptions()
	opts.caNotAfter = time.Now().Add(-time.Minute)
	payload := buildPayload(t, opts)

	_, err := config.ValidateNodePayload(payload)
	if err == nil {
		t.Fatalf("err = nil, want expired CA failure")
	}
	assertCheckFails(t, payload, "CA not expired")
}

func TestValidateNodePayloadRejectsNodeCertFromOtherCA(t *testing.T) {
	opts := defaultPayloadOptions()
	opts.wrongSigner = true
	payload := buildPayload(t, opts)

	if _, err := config.ValidateNodePayload(payload); err == nil {
		t.Fatalf("err = nil, want foreign CA failure")
	}
	assertCheckFails(t, payload, "node signed by CA")
}

func TestValidateNodePayloadRejectsKeyMismatch(t *testing.T) {
	opts := defaultPayloadOptions()
	opts.keyMismatch = true
	payload := buildPayload(t, opts)

	if _, err := config.ValidateNodePayload(payload); err == nil {
		t.Fatalf("err = nil, want key mismatch failure")
	}
	assertCheckFails(t, payload, "node key matches cert")
}

func TestValidateNodePayloadRejectsNonRSAJWTKey(t *testing.T) {
	opts := defaultPayloadOptions()
	opts.jwtKeyAsPEM = "not a pem"
	payload := buildPayload(t, opts)

	if _, err := config.ValidateNodePayload(payload); err == nil {
		t.Fatalf("err = nil, want jwt key failure")
	}
	assertCheckFails(t, payload, "jwt public key")
}

func TestValidateNodePayloadAcceptsECDSANodeKey(t *testing.T) {
	// 面板签发的节点密钥可能是 ECDSA（P-256）PKCS8；历史上
	// verifyKeyMatchesCert 只匹配 interface{ Public() any }，而
	// *ecdsa.PrivateKey 实现的是 Public() crypto.PublicKey，导致误报
	// "has no extractable public key"。
	caKey := newKey(t)
	ca := newCA(t, caKey, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	nodeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate node key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "test-node-ec"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &nodeKey.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("issue node cert: %v", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(nodeKey)
	if err != nil {
		t.Fatalf("marshal node key: %v", err)
	}

	jwtKey := newKey(t)
	jwtPubDER, err := x509.MarshalPKIXPublicKey(&jwtKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal jwt public key: %v", err)
	}

	payload := config.NodePayload{
		CACertPEM:    ca.pem,
		JWTPublicKey: encodeTestPEM("PUBLIC KEY", jwtPubDER),
		NodeCertPEM:  encodeTestPEM("CERTIFICATE", der),
		NodeKeyPEM:   encodeTestPEM("PRIVATE KEY", keyDER),
	}

	checks, err := config.ValidateNodePayload(payload)
	if err != nil {
		t.Fatalf("err = %v; checks = %#v", err, checks)
	}
	for _, check := range checks {
		if !check.OK {
			t.Fatalf("check %s failed: %s", check.Name, check.Detail)
		}
	}
}

func assertCheckFails(t *testing.T, payload config.NodePayload, name string) {
	t.Helper()
	checks, err := config.ValidateNodePayload(payload)
	if err == nil {
		t.Fatalf("err = nil, want failure")
	}
	for _, check := range checks {
		if check.Name == name && !check.OK {
			return
		}
	}
	t.Fatalf("check %q did not fail; checks = %#v err = %v", name, checks, err)
}
