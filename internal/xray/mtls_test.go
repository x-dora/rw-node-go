package xray

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
)

func TestInternalMTLSBundleVerifiesServerAndClientCertificates(t *testing.T) {
	bundle, err := NewInternalMTLSBundle()
	if err != nil {
		t.Fatalf("NewInternalMTLSBundle() error = %v", err)
	}

	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM([]byte(bundle.CACertPEM)); !ok {
		t.Fatalf("AppendCertsFromPEM() = false")
	}

	serverCert, err := tls.X509KeyPair([]byte(bundle.ServerCertPEM), []byte(bundle.ServerKeyPEM))
	if err != nil {
		t.Fatalf("load server keypair: %v", err)
	}
	serverLeaf, err := x509.ParseCertificate(serverCert.Certificate[0])
	if err != nil {
		t.Fatalf("parse server cert: %v", err)
	}
	if _, err := serverLeaf.Verify(x509.VerifyOptions{
		DNSName:     InternalServerName,
		Roots:       roots,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		CurrentTime: serverLeaf.NotBefore.Add(serverLeaf.NotAfter.Sub(serverLeaf.NotBefore) / 2),
	}); err != nil {
		t.Fatalf("verify server cert: %v", err)
	}

	clientCert, err := tls.X509KeyPair([]byte(bundle.ClientCertPEM), []byte(bundle.ClientKeyPEM))
	if err != nil {
		t.Fatalf("load client keypair: %v", err)
	}
	clientLeaf, err := x509.ParseCertificate(clientCert.Certificate[0])
	if err != nil {
		t.Fatalf("parse client cert: %v", err)
	}
	if _, err := clientLeaf.Verify(x509.VerifyOptions{
		Roots:       roots,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		CurrentTime: clientLeaf.NotBefore.Add(clientLeaf.NotAfter.Sub(clientLeaf.NotBefore) / 2),
	}); err != nil {
		t.Fatalf("verify client cert: %v", err)
	}
}
