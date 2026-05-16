package logview

import (
	"strings"
	"testing"

	"github.com/x-dora/rw-node-go/internal/config"
)

func TestStartupSummaryRendersRuntimeMetadataWithoutSecret(t *testing.T) {
	cfg := config.Config{
		NodePort:              2222,
		InternalRESTPort:      61001,
		SecretKey:             "secret-key-value",
		RequestBodyLimitBytes: 1024,
	}

	output := StartupSummary(cfg, "2.8.0")

	for _, want := range []string{"rw-node-go starting", "Project Version", "Panel Node Version", "2.8.0", "embedded xray-core", "0.0.0.0:2222", "127.0.0.1:61001", "TLS Enabled", "TLS Client Auth", "mtls", "JWT Enabled"} {
		if !strings.Contains(output, want) {
			t.Fatalf("StartupSummary() missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "secret-key-value") {
		t.Fatalf("StartupSummary() leaked SECRET_KEY:\n%s", output)
	}
}

func TestListenSummary(t *testing.T) {
	output := ListenSummary("Main API listening", "0.0.0.0:2222", "https")
	for _, want := range []string{"Main API listening", "0.0.0.0:2222", "https"} {
		if !strings.Contains(output, want) {
			t.Fatalf("ListenSummary() missing %q:\n%s", want, output)
		}
	}
}
