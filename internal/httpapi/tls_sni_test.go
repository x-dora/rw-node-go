package httpapi

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/testkit"
)

// Golden vector generated with the official deriveSni implementation
// (node:crypto hkdfSync, empty salt, info "rw-v1").
const (
	vectorJWTPublicKey = "-----BEGIN PUBLIC KEY-----\n" +
		"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwoowdrPTaT62WuElFANe\n" +
		"TQOKAfE2uvbrPjqL1I+cae5YKoiE4PEMzYRaYPb4cYUO99fzv1zqHOn8//hCH5Ry\n" +
		"baS2FCBEne26s4TfL48/C9/Cp+TKaR1zzzWABvFTWdIrGPbZhrnL2IMh01Nfkf6L\n" +
		"v8RZ3aji+6NUTGdlGsbgg1Z9HWb4WNNGNv4E26A7jR5o8ymQSILbvHBK151LgLwK\n" +
		"vR1RHk1vhurBMxfCKLpwLdLg8dUh4pdVI6vmBCXGWfeqyf8LY9yCHKoDCdQJxXjD\n" +
		"3A6rQd/g87DVjUHqdDbrmL4SKcKRF7Jb0UgxLQmK5IZp5E2eTi8ZTCSoTkx1kniw\n" +
		"mwIDAQAB\n" +
		"-----END PUBLIC KEY-----\n"
	vectorCACertPEM = "-----BEGIN CERTIFICATE-----\r\n" +
		"MIIDCTCCAfGgAwIBAgIUc6aE6tuAOSwGej3EcHeviJRng40wDQYJKoZIhvcNAQEL\r\n" +
		"BQAwFDESMBAGA1UEAwwJdmVjdG9yLWNhMB4XDTI2MDkwNjA2MTgzN1oXDTI2MTAw\r\n" +
		"NjA2MTgzN1owFDESMBAGA1UEAwwJdmVjdG9yLWNhMIIBIjANBgkqhkiG9w0BAQEF\r\n" +
		"AAOCAQ8AMIIBCgKCAQEAsmx33JTKjZZ8LrltOmq+xIGObsqTChLAYN6GGRck8pyb\r\n" +
		"SPy0blq657XWLp+GpeRSD2qnyZXRJJ8iHxtcIxQDnUjs1o649QX7xOOwrDlff82r\r\n" +
		"nBymLnAIWnYYTPQwbpGkwYfg+tjafdYM9lYPQbPSPAnCXgRNAEO2TtSQdEcfrq3L\r\n" +
		"Yw7r9MvzTGt7bGzcUhxBD2qsTCx2QMM7KkqQD5S/IFS41tdokSzKo9BaiHY/Kfww\r\n" +
		"dlHdaRa62H5f6W+6JrRMDoT3+vsyyyu+1t3aqLJRRXgrAZTvAHN08K/SaNJTFxCT\r\n" +
		"XmsN+wfkkmN6lBdqCTAH17nzIPld84l/0+quOX8rdwIDAQABo1MwUTAdBgNVHQ4E\r\n" +
		"FgQU0OjR6YM0KKtFpHmsK6YlnEpvdYgwHwYDVR0jBBgwFoAU0OjR6YM0KKtFpHms\r\n" +
		"K6YlnEpvdYgwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAQc0S\r\n" +
		"3B5aLYruXBaM0MXxmCROhDxsxBtGKapyZEwArJ9Pp0oi9xC6TrJC7ZFxYhlCni9m\r\n" +
		"NS2/SA2pv1LXGh07OCl+in3War3WR191GtYZ3Q8V9AceWuTd3XeSP4e3H3bdbY7Y\r\n" +
		"+9uMqrLmbqDZVKr4EDnHqlYl8/qgiLM3EGkYj1alJjPCuoR9wXx9FlTCrURU82jF\r\n" +
		"eY5lXM2LnDSXtSQ3CQxOOX+jZjJgIawfiRydJ749JqNo1pyBIHX1hJWQOk/yGdQO\r\n" +
		"CSKnZuFyYCr6AZudvh441RzM+7YCYWyrO7uKEIuK3PENGYLBMLS8FT9tmUVYzOeH\r\n" +
		"d0g16If3dUtDUlIt7w==\r\n" +
		"-----END CERTIFICATE-----\r\n"
	vectorSNI = "509d286ef126df278627b62ed7ebd9ec.77b3f673ca.com"
)

func TestDeriveSNIMatchesOfficialVector(t *testing.T) {
	payload := config.NodePayload{
		JWTPublicKey: vectorJWTPublicKey,
		CACertPEM:    vectorCACertPEM,
	}

	got, err := DeriveSNI(payload)
	if err != nil {
		t.Fatalf("DeriveSNI err = %v", err)
	}
	if got != vectorSNI {
		t.Fatalf("sni = %q, want %q", got, vectorSNI)
	}
}

func TestSNIVerificationGatesHandshake(t *testing.T) {
	bundle := testkit.NewCertBundle(t)
	tlsConfig, err := TLSConfigFromSecretWithOptions(bundle.Payload, "mtls", true)
	if err != nil {
		t.Fatalf("TLSConfigFromSecretWithOptions err = %v", err)
	}
	if tlsConfig.GetConfigForClient == nil {
		t.Fatalf("GetConfigForClient not set with SNI verification enabled")
	}

	expected, err := DeriveSNI(bundle.Payload)
	if err != nil {
		t.Fatalf("DeriveSNI err = %v", err)
	}

	if _, err := tlsConfig.GetConfigForClient(&tls.ClientHelloInfo{ServerName: expected}); err != nil {
		t.Fatalf("matching servername rejected: %v", err)
	}
	if _, err := tlsConfig.GetConfigForClient(&tls.ClientHelloInfo{ServerName: "evil.example.com"}); err == nil {
		t.Fatalf("unknown servername accepted")
	}
	if _, err := tlsConfig.GetConfigForClient(&tls.ClientHelloInfo{ServerName: expected + "x"}); err == nil {
		t.Fatalf("padded servername accepted")
	}
	if _, err := tlsConfig.GetConfigForClient(&tls.ClientHelloInfo{}); err == nil {
		t.Fatalf("empty servername accepted")
	}
}

func TestSNIVerificationDisabledByDefault(t *testing.T) {
	bundle := testkit.NewCertBundle(t)
	tlsConfig, err := TLSConfigFromSecretWithOptions(bundle.Payload, "mtls", false)
	if err != nil {
		t.Fatalf("TLSConfigFromSecretWithOptions err = %v", err)
	}
	if tlsConfig.GetConfigForClient != nil {
		t.Fatalf("GetConfigForClient set with SNI verification disabled")
	}
}

func TestSecureServerSNIVerificationGatesHandshake(t *testing.T) {
	bundle := testkit.NewCertBundle(t)
	raw, err := json.Marshal(bundle.Payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	cfg := config.Config{
		SecretKey:             base64.StdEncoding.EncodeToString(raw),
		InternalRESTPort:      61001,
		RequestBodyLimitBytes: 1 << 20,
		SNIVerification:       true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := NewServer(cfg, Handlers{
		Xray:     tlsTestHandlers{},
		Handler:  tlsTestHandlers{},
		Stats:    tlsTestHandlers{},
		Plugin:   tlsTestHandlers{},
		Internal: tlsTestHandlers{},
	}, logger)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	ts := httptest.NewUnstartedServer(server.httpServer.Handler)
	ts.TLS = server.httpServer.TLSConfig
	ts.StartTLS()
	defer ts.Close()

	expected, err := DeriveSNI(bundle.Payload)
	if err != nil {
		t.Fatalf("DeriveSNI err = %v", err)
	}
	token := "Bearer " + testkit.NewRS256Token(t, bundle.JWTPrivateKey)

	doRequest := func(serverName string) error {
		tlsClientConfig := clientTLSConfig(t, bundle)
		// The SNI gate is under test, not hostname verification of the node
		// certificate, which has no SAN for the derived name.
		tlsClientConfig.InsecureSkipVerify = true
		tlsClientConfig.ServerName = serverName
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsClientConfig}}
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/node/xray/healthcheck", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Authorization", token)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status with serverName %q = %d, want %d", serverName, resp.StatusCode, http.StatusOK)
		}
		return nil
	}

	if err := doRequest("wrong.example.com"); err == nil {
		t.Fatalf("handshake with unknown servername unexpectedly succeeded")
	}
	if err := doRequest(expected); err != nil {
		t.Fatalf("handshake with derived servername failed: %v", err)
	}
}
