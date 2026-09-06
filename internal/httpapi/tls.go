package httpapi

import (
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/x-dora/rw-node-go/internal/config"
)

func TLSConfigFromSecret(payload config.NodePayload) (*tls.Config, error) {
	return TLSConfigFromSecretWithOptions(payload, config.DefaultNodeTLSClientAuth, false)
}

func TLSConfigFromSecretWithClientAuth(payload config.NodePayload, clientAuthMode string) (*tls.Config, error) {
	return TLSConfigFromSecretWithOptions(payload, clientAuthMode, false)
}

// TLSConfigFromSecretWithOptions builds the main API TLS config. When
// sniVerification is enabled the handshake is gated behind the derived SNI,
// mirroring the official SNI_VERIFICATION behavior (off by default).
func TLSConfigFromSecretWithOptions(payload config.NodePayload, clientAuthMode string, sniVerification bool) (*tls.Config, error) {
	cert, err := tls.X509KeyPair([]byte(payload.NodeCertPEM), []byte(payload.NodeKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("load node keypair: %w", err)
	}

	clientCAs := x509.NewCertPool()
	if ok := clientCAs.AppendCertsFromPEM([]byte(payload.CACertPEM)); !ok {
		return nil, fmt.Errorf("load client CA")
	}

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    clientCAs,
		ClientAuth:   tlsClientAuth(clientAuthMode),
	}
	if sniVerification {
		expected, err := DeriveSNI(payload)
		if err != nil {
			return nil, err
		}
		tlsConfig.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			if subtle.ConstantTimeCompare([]byte(hello.ServerName), []byte(expected)) != 1 {
				return nil, errors.New("unknown sni")
			}
			return tlsConfig, nil
		}
	}
	return tlsConfig, nil
}

// sniTLDs mirrors the official decode-servername.util.ts TLDS list.
var sniTLDs = []string{"com", "net", "org", "io", "dev", "app"}

// DeriveSNI derives the SNI hostname that gates the main API when
// SNI_VERIFICATION is enabled, mirroring the official deriveSni:
// HKDF-SHA256 over canon(jwtPublicKey) || canon(caCertPem) with empty salt and
// info "rw-v1", producing <32 hex>.<10 hex>.<tld>.
func DeriveSNI(payload config.NodePayload) (string, error) {
	ikm := append(
		[]byte(canonBase64(payload.JWTPublicKey)),
		[]byte(canonBase64(payload.CACertPEM))...,
	)
	okm, err := hkdf.Key(sha256.New, ikm, nil, "rw-v1", 22)
	if err != nil {
		return "", fmt.Errorf("derive sni: %w", err)
	}
	host := hex.EncodeToString(okm[0:16])
	label := hex.EncodeToString(okm[16:21])
	tld := sniTLDs[int(okm[21])%len(sniTLDs)]
	return host + "." + label + "." + tld, nil
}

// canonBase64 mirrors the official canon helper: strip PEM header/footer
// markers and keep only base64 characters.
func canonBase64(pemValue string) string {
	var builder strings.Builder
	for _, line := range strings.Split(pemValue, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-----") {
			continue
		}
		for _, c := range line {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
				builder.WriteRune(c)
			}
		}
	}
	return builder.String()
}

func tlsClientAuth(mode string) tls.ClientAuthType {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "optional":
		return tls.VerifyClientCertIfGiven
	case "none":
		return tls.NoClientCert
	default:
		return tls.RequireAndVerifyClientCert
	}
}
