package httpapi

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/x-dora/rw-node-go/internal/config"
)

func TLSConfigFromSecret(payload config.NodePayload) (*tls.Config, error) {
	cert, err := tls.X509KeyPair([]byte(payload.NodeCertPEM), []byte(payload.NodeKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("load node keypair: %w", err)
	}

	clientCAs := x509.NewCertPool()
	if ok := clientCAs.AppendCertsFromPEM([]byte(payload.CACertPEM)); !ok {
		return nil, fmt.Errorf("load client CA")
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}
