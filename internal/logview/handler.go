package logview

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
)

const tableRecordAttr = "__logview_table"

const (
	ansiReset  = "\x1b[0m"
	ansiGray   = "\x1b[90m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
)

type textHandler struct {
	inner  slog.Handler
	out    io.Writer
	opts   *slog.HandlerOptions
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
	color  bool
}

// NewTextHandler preserves slog's text output for ordinary records and renders
// LogTable records as raw newline-prefixed table blocks.
func NewTextHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return newTextHandler(w, opts, false)
}

// NewColorTextHandler renders the same log shape as NewTextHandler with
// level-based ANSI coloring for interactive and Docker logs.
func NewColorTextHandler(w io.Writer, opts *slog.HandlerOptions, color bool) slog.Handler {
	return newTextHandler(w, opts, color)
}

func newTextHandler(w io.Writer, opts *slog.HandlerOptions, color bool) slog.Handler {
	return &textHandler{
		inner: slog.NewTextHandler(w, opts),
		out:   w,
		opts:  opts,
		mu:    &sync.Mutex{},
		color: color,
	}
}

func (h *textHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *textHandler) Handle(ctx context.Context, record slog.Record) error {
	if isTableRecord(record) {
		h.mu.Lock()
		defer h.mu.Unlock()

		message := record.Message
		if h.color {
			message = colorize(record.Level, message)
		}
		if _, err := io.WriteString(h.out, message); err != nil {
			return err
		}
		if !strings.HasSuffix(message, "\n") {
			_, err := io.WriteString(h.out, "\n")
			return err
		}
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.color {
		return h.inner.Handle(ctx, record)
	}

	var buffer bytes.Buffer
	var handler slog.Handler = slog.NewTextHandler(&buffer, h.opts)
	if len(h.attrs) > 0 {
		handler = handler.WithAttrs(h.attrs)
	}
	for _, group := range h.groups {
		handler = handler.WithGroup(group)
	}
	if err := handler.Handle(ctx, record); err != nil {
		return err
	}
	_, err := io.WriteString(h.out, colorize(record.Level, buffer.String()))
	return err
}

func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &textHandler{
		inner:  h.inner.WithAttrs(attrs),
		out:    h.out,
		opts:   h.opts,
		attrs:  appendAttrs(h.attrs, attrs),
		groups: append([]string(nil), h.groups...),
		mu:     h.mu,
		color:  h.color,
	}
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	return &textHandler{
		inner:  h.inner.WithGroup(name),
		out:    h.out,
		opts:   h.opts,
		attrs:  append([]slog.Attr(nil), h.attrs...),
		groups: appendGroup(h.groups, name),
		mu:     h.mu,
		color:  h.color,
	}
}

func isTableRecord(record slog.Record) bool {
	found := false
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == tableRecordAttr && attr.Value.Kind() == slog.KindBool && attr.Value.Bool() {
			found = true
			return false
		}
		return true
	})
	return found
}

func colorize(level slog.Level, value string) string {
	color := levelColor(level)
	if color == "" || value == "" {
		return value
	}
	if strings.HasSuffix(value, "\n") {
		return color + strings.TrimSuffix(value, "\n") + ansiReset + "\n"
	}
	return color + value + ansiReset
}

func levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return ansiRed
	case level >= slog.LevelWarn:
		return ansiYellow
	case level <= slog.LevelDebug:
		return ansiGray
	default:
		return ansiGreen
	}
}

func appendAttrs(existing []slog.Attr, attrs []slog.Attr) []slog.Attr {
	out := make([]slog.Attr, 0, len(existing)+len(attrs))
	out = append(out, existing...)
	out = append(out, attrs...)
	return out
}

func appendGroup(existing []string, group string) []string {
	out := make([]string, 0, len(existing)+1)
	out = append(out, existing...)
	out = append(out, group)
	return out
}
