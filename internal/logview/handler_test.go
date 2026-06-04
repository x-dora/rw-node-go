package logview

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestColorTextHandlerColorsOrdinaryRecordsByLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(NewColorTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}, true))

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	logs := buffer.String()
	for _, want := range []string{
		ansiGray + "time=",
		ansiGreen + "time=",
		ansiYellow + "time=",
		ansiRed + "time=",
		"level=DEBUG",
		"level=INFO",
		"level=WARN",
		"level=ERROR",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("colored logs missing %q:\n%s", want, logs)
		}
	}
	if got, want := strings.Count(logs, ansiReset), 4; got != want {
		t.Fatalf("reset count = %d, want %d:\n%s", got, want, logs)
	}
}

func TestColorTextHandlerCanDisableColor(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(NewColorTextHandler(&buffer, nil, false))

	logger.Info("info message")

	logs := buffer.String()
	if strings.Contains(logs, "\x1b[") {
		t.Fatalf("disabled color logs contain ANSI escape:\n%s", logs)
	}
	if !strings.Contains(logs, "msg=\"info message\"") {
		t.Fatalf("logs missing message:\n%s", logs)
	}
}

func TestColorTextHandlerPreservesAttrsAndGroups(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(NewColorTextHandler(&buffer, nil, true)).
		With("component", "test").
		WithGroup("request")

	logger.Info("grouped message", "id", 123)

	logs := buffer.String()
	for _, want := range []string{"component=test", "request.id=123", ansiGreen} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
}

func TestColorTextHandlerColorsTableBlockWithoutChangingShape(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(NewColorTextHandler(&buffer, nil, true))

	InfoTable(logger, "summary", Table("demo", Field("Value", "ok")))

	logs := buffer.String()
	for _, want := range []string{ansiGreen + "time=", "level=INFO", ansiGreen + "\n+---------------+", "|     demo      |", "| Value | ok    |"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("colored table logs missing %q:\n%s", want, logs)
		}
	}
	for _, oldLineLog := range []string{"msg=+---------------+", "msg=\"|     demo      |\"", "msg=\"| Value | ok    |\""} {
		if strings.Contains(logs, oldLineLog) {
			t.Fatalf("table should not be logged as separate slog lines %q:\n%s", oldLineLog, logs)
		}
	}
	if !strings.HasSuffix(logs, ansiReset+"\n") {
		t.Fatalf("colored table logs should end with reset and newline:\n%s", logs)
	}
}
