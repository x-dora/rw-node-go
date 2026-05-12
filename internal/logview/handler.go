package logview

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
)

const tableRecordAttr = "__logview_table"

type textHandler struct {
	inner slog.Handler
	out   io.Writer
	mu    *sync.Mutex
}

// NewTextHandler preserves slog's text output for ordinary records and renders
// LogTable records as raw newline-prefixed table blocks.
func NewTextHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &textHandler{
		inner: slog.NewTextHandler(w, opts),
		out:   w,
		mu:    &sync.Mutex{},
	}
}

func (h *textHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *textHandler) Handle(ctx context.Context, record slog.Record) error {
	if isTableRecord(record) {
		h.mu.Lock()
		defer h.mu.Unlock()

		if _, err := io.WriteString(h.out, record.Message); err != nil {
			return err
		}
		if !strings.HasSuffix(record.Message, "\n") {
			_, err := io.WriteString(h.out, "\n")
			return err
		}
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	return h.inner.Handle(ctx, record)
}

func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &textHandler{
		inner: h.inner.WithAttrs(attrs),
		out:   h.out,
		mu:    h.mu,
	}
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	return &textHandler{
		inner: h.inner.WithGroup(name),
		out:   h.out,
		mu:    h.mu,
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
