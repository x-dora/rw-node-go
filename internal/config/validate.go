package config

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PayloadCheck is one item of the SECRET_KEY integrity report, mirroring the
// official assertPayloadIntegrity checks introduced in node 3.3.1.
type PayloadCheck struct {
	Name   string
	OK     bool
	Detail string
}

// ValidateNodePayload verifies the SECRET_KEY payload end to end: the CA must
// parse, be currently valid and self-signed; the node certificate must be
// signed by that CA and match the node key; the JWT public key must parse as
// an RSA key (the JWT middleware enforces RS256).
func ValidateNodePayload(payload NodePayload) ([]PayloadCheck, error) {
	now := time.Now()
	checks := make([]PayloadCheck, 0, 7)
	add := func(name string, detail string, err error) {
		if err != nil {
			checks = append(checks, PayloadCheck{Name: name, OK: false, Detail: firstLine(err.Error())})
			return
		}
		checks = append(checks, PayloadCheck{Name: name, OK: true, Detail: detail})
	}

	caCert, err := parseCertificate(payload.CACertPEM)
	add("CA parses", shortFingerprint(caCert), err)

	if caCert != nil {
		if caCert.NotBefore.After(now) {
			add("CA not expired", "", errors.New("not yet valid"))
		} else if now.After(caCert.NotAfter) {
			add("CA not expired", "", errors.New("expired"))
		} else {
			add("CA not expired", fmt.Sprintf("until %s", caCert.NotAfter.UTC().Format(time.RFC3339)), nil)
		}
		add("CA self-signature", "valid", caCert.CheckSignatureFrom(caCert))
	} else {
		add("CA not expired", "", errors.New("CA unavailable"))
		add("CA self-signature", "", errors.New("CA unavailable"))
	}

	nodeCert, err := parseCertificate(payload.NodeCertPEM)
	add("node cert parses", shortFingerprint(nodeCert), err)

	if caCert != nil && nodeCert != nil {
		add("node signed by CA", "valid", nodeCert.CheckSignatureFrom(caCert))
	} else {
		add("node signed by CA", "", errors.New("cert unavailable"))
	}

	add("node key matches cert", "valid", verifyKeyMatchesCert(payload.NodeKeyPEM, nodeCert))

	add("jwt public key", "ok", verifyRSAPublicKey(payload.JWTPublicKey))

	failures := make([]string, 0)
	for _, check := range checks {
		if !check.OK {
			failures = append(failures, fmt.Sprintf("%s (%s)", check.Name, check.Detail))
		}
	}
	if len(failures) > 0 {
		return checks, fmt.Errorf("SECRET_KEY payload validation failed: %s", strings.Join(failures, "; "))
	}
	return checks, nil
}

func parseCertificate(pemValue string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func shortFingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])[:24]
}

func verifyKeyMatchesCert(keyPEM string, cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("cert unavailable")
	}
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return errors.New("no PEM block")
	}
	var public any
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			public = &k.PublicKey
		case *ecdsa.PrivateKey:
			public = &k.PublicKey
		case *ed25519.PrivateKey:
			public = k.Public()
		default:
			if signer, ok := key.(interface{ Public() crypto.PublicKey }); ok {
				public = signer.Public()
			} else {
				return fmt.Errorf("node key type %T has no extractable public key", key)
			}
		}
	} else if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		public = &key.PublicKey
	} else if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		public = &key.PublicKey
	} else {
		return errors.New("unsupported node key format")
	}

	certKeyDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal cert public key: %w", err)
	}
	keyKeyDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return fmt.Errorf("marshal node key public key: %w", err)
	}
	if !equalBytes(certKeyDER, keyKeyDER) {
		return errors.New("key does not match cert")
	}
	return nil
}

func verifyRSAPublicKey(pemValue string) error {
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return errors.New("no PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse jwt public key: %w", err)
	}
	if _, ok := key.(*rsa.PublicKey); !ok {
		return fmt.Errorf("jwt public key is %T, want RSA for RS256", key)
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func firstLine(value string) string {
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return value[:idx]
	}
	return value
}
