package logview

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestTableRendersTitleAndRows(t *testing.T) {
	output := Table("Xray started",
		Field("Version", "25.1.1"),
		Field("Internal Status", true),
	)

	if !strings.HasPrefix(output, "\n") {
		t.Fatalf("Table() should start with a newline:\n%s", output)
	}
	for _, want := range []string{"Xray started", "Version", "25.1.1", "Internal Status", "true"} {
		if !strings.Contains(output, want) {
			t.Fatalf("Table() missing %q:\n%s", want, output)
		}
	}
}

func TestValueFormatsEmptyBoolDurationAndLongString(t *testing.T) {
	if got := Value(""); got != EmptyValue {
		t.Fatalf("Value(empty) = %q, want %q", got, EmptyValue)
	}
	if got := Bool(true); got != "true" {
		t.Fatalf("Bool(true) = %q", got)
	}
	if got := Duration(1250 * time.Millisecond); got != "1.25s" {
		t.Fatalf("Duration() = %q, want 1.25s", got)
	}
	long := strings.Repeat("a", DefaultMaxValueLength+10)
	got := Value(long)
	if len(got) != DefaultMaxValueLength || !strings.HasSuffix(got, "...") {
		t.Fatalf("Value(long) = len %d suffix %q", len(got), got[len(got)-3:])
	}
}

func TestShortHash(t *testing.T) {
	if got := ShortHash(""); got != EmptyValue {
		t.Fatalf("ShortHash(empty) = %q, want %q", got, EmptyValue)
	}
	if got := ShortHash("abcdef"); got != "abcdef" {
		t.Fatalf("ShortHash(short) = %q", got)
	}
	if got := ShortHash("1234567890abcdef"); got != "1234567890ab" {
		t.Fatalf("ShortHash(long) = %q", got)
	}
}

func TestSecretLikeValuesAreOnlyRenderedWhenExplicitlyPassed(t *testing.T) {
	output := Table("startup",
		Field("SECRET_KEY", ""),
		Field("JWT Enabled", true),
	)
	if strings.Contains(output, "token-secret-value") {
		t.Fatalf("Table leaked unexpected secret: %s", output)
	}
	if !strings.Contains(output, "SECRET_KEY") || !strings.Contains(output, EmptyValue) {
		t.Fatalf("Table should render explicit empty secret-like field safely:\n%s", output)
	}
}

func TestInboundTableRendersInboundSummary(t *testing.T) {
	output := InboundTable("inbounds", []InboundRow{{
		Tag:        "VLESS_INBOUND",
		UsersCount: 2,
		Hash:       "1234567890abcdef",
	}})

	for _, want := range []string{"VLESS_INBOUND", "users=2", "hash=1234567890ab"} {
		if !strings.Contains(output, want) {
			t.Fatalf("InboundTable() missing %q:\n%s", want, output)
		}
	}
}

func TestRedactTextMasksSensitiveFragments(t *testing.T) {
	input := `parse failed: password: "trojan-password" privateKey=super-private-key shortId: abc123 Authorization: Bearer abc.def.ghi id":"client-secret"`
	got := RedactText(input)
	for _, leaked := range []string{"trojan-password", "super-private-key", "abc123", "abc.def.ghi", "client-secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("RedactText() leaked %q in %s", leaked, got)
		}
	}
	if !strings.Contains(got, "parse failed") {
		t.Fatalf("RedactText() removed safe context: %s", got)
	}
}

func TestInfoTableLogsTableAsLeadingNewlineBlock(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(NewTextHandler(&buffer, nil))

	InfoTable(logger, "summary", Table("demo", Field("Value", "ok")))

	logs := buffer.String()
	for _, want := range []string{"msg=summary", "\n+---------------+", "|     demo      |", "| Value | ok    |"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
	for _, oldLineLog := range []string{"msg=+---------------+", "msg=\"|     demo      |\"", "msg=\"| Value | ok    |\""} {
		if strings.Contains(logs, oldLineLog) {
			t.Fatalf("table should not be logged as separate slog lines %q:\n%s", oldLineLog, logs)
		}
	}
	if strings.Contains(logs, `summary="`) {
		t.Fatalf("table should not be logged as a summary attribute:\n%s", logs)
	}
}

func TestInfoTableWithStandardTextHandlerEscapesTableRecord(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buffer, nil))

	InfoTable(logger, "summary", Table("demo", Field("Value", "ok")))

	logs := buffer.String()
	for _, want := range []string{"msg=summary", `msg="\n+---------------+`, tableRecordAttr + "=true"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("standard handler logs missing %q:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "\n+---------------+") {
		t.Fatalf("standard handler should escape table newlines instead of rendering raw blocks:\n%s", logs)
	}
}
