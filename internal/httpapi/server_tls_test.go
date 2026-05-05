package httpapi

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/testkit"
)

func TestSecureServerRequiresMTLSAndJWT(t *testing.T) {
	bundle := testkit.NewCertBundle(t)
	raw, err := json.Marshal(bundle.Payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	cfg := config.Config{
		SecretKey:             base64.StdEncoding.EncodeToString(raw),
		XrayBin:               "xray",
		XrayConfigPath:        t.TempDir() + "/config.json",
		XTLSAPIPort:           61000,
		RequestBodyLimitBytes: 1 << 20,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := NewServer(cfg, Handlers{
		Xray:     tlsTestHandlers{},
		Handler:  tlsTestHandlers{},
		Stats:    tlsTestHandlers{},
		Vision:   tlsTestHandlers{},
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

	if _, err := ts.Client().Get(ts.URL + "/node/xray/healthcheck"); err == nil {
		t.Fatalf("request without client certificate unexpectedly succeeded")
	}

	client := ts.Client()
	client.Transport = &http.Transport{TLSClientConfig: clientTLSConfig(t, bundle)}
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/node/xray/healthcheck", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status without JWT = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/node/xray/healthcheck", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testkit.NewRS256Token(t, bundle.JWTPrivateKey))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request with JWT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status with JWT = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

type tlsTestHandlers struct{}

func (tlsTestHandlers) Start(c *gin.Context)                        { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) Stop(c *gin.Context)                         { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) Healthcheck(c *gin.Context)                  { c.Status(http.StatusOK) }
func (tlsTestHandlers) AddUser(c *gin.Context)                      { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) AddUsers(c *gin.Context)                     { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) RemoveUser(c *gin.Context)                   { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) RemoveUsers(c *gin.Context)                  { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetInboundUsers(c *gin.Context)              { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetInboundUsersCount(c *gin.Context)         { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) DropUsersConnections(c *gin.Context)         { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) DropIPs(c *gin.Context)                      { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetSystemStats(c *gin.Context)               { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetUsersStats(c *gin.Context)                { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetUserOnlineStatus(c *gin.Context)          { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetUserIPList(c *gin.Context)                { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetUsersIPList(c *gin.Context)               { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetInboundStats(c *gin.Context)              { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetOutboundStats(c *gin.Context)             { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetAllInboundsStats(c *gin.Context)          { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetAllOutboundsStats(c *gin.Context)         { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetCombinedStats(c *gin.Context)             { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) BlockIP(c *gin.Context)                      { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) UnblockIP(c *gin.Context)                    { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) Sync(c *gin.Context)                         { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) CollectTorrentBlockerReports(c *gin.Context) { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) BlockIPs(c *gin.Context)                     { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) UnblockIPs(c *gin.Context)                   { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) RecreateTables(c *gin.Context)               { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) GetConfig(c *gin.Context)                    { c.Status(http.StatusNoContent) }
func (tlsTestHandlers) Webhook(c *gin.Context)                      { c.Status(http.StatusNoContent) }

func clientTLSConfig(t *testing.T, bundle testkit.CertBundle) *tls.Config {
	t.Helper()
	cert, err := tls.X509KeyPair([]byte(bundle.PanelCertPEM), []byte(bundle.PanelKeyPEM))
	if err != nil {
		t.Fatalf("load panel certificate: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(bundle.Payload.CACertPEM)) {
		t.Fatalf("append CA")
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		RootCAs:      roots,
	}
}
