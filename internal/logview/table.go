package logview

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMaxValueLength = 96
	EmptyValue            = "-"
)

type Row struct {
	Key   string
	Value string
}

func Field(key string, value any) Row {
	return Row{Key: key, Value: Value(value)}
}

func Table(title string, rows ...Row) string {
	title = strings.TrimSpace(title)
	keyWidth := len("Field")
	valueWidth := len("Value")
	normalized := make([]Row, 0, len(rows))
	for _, row := range rows {
		key := strings.TrimSpace(row.Key)
		value := strings.TrimSpace(row.Value)
		if key == "" {
			continue
		}
		if value == "" {
			value = EmptyValue
		}
		normalized = append(normalized, Row{Key: key, Value: value})
		keyWidth = max(keyWidth, len(key))
		valueWidth = max(valueWidth, len(value))
	}

	contentWidth := keyWidth + valueWidth + 5
	if title != "" {
		contentWidth = max(contentWidth, len(title)+2)
	}
	border := "+" + strings.Repeat("-", contentWidth) + "+"

	var b strings.Builder
	b.WriteByte('\n')
	b.WriteString(border)
	b.WriteByte('\n')
	if title != "" {
		b.WriteString("|")
		b.WriteString(center(title, contentWidth))
		b.WriteString("|")
		b.WriteByte('\n')
		b.WriteString(border)
		b.WriteByte('\n')
	}
	b.WriteString("| ")
	b.WriteString(padRight("Field", keyWidth))
	b.WriteString(" | ")
	b.WriteString(padRight("Value", valueWidth))
	b.WriteString(" |")
	b.WriteByte('\n')
	b.WriteString(border)
	for _, row := range normalized {
		b.WriteByte('\n')
		b.WriteString("| ")
		b.WriteString(padRight(row.Key, keyWidth))
		b.WriteString(" | ")
		b.WriteString(padRight(row.Value, valueWidth))
		b.WriteString(" |")
	}
	b.WriteByte('\n')
	b.WriteString(border)
	return b.String()
}

func InboundTable(title string, inbounds []InboundRow) string {
	rows := make([]Row, 0, len(inbounds))
	for _, inbound := range inbounds {
		label := inbound.Tag
		if strings.TrimSpace(label) == "" {
			label = "untagged"
		}
		rows = append(rows, Row{
			Key:   label,
			Value: fmt.Sprintf("users=%d hash=%s", inbound.UsersCount, ShortHash(inbound.Hash)),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, Field("Inbounds", "none"))
	}
	return Table(title, rows...)
}

func InfoTable(logger *slog.Logger, message string, table string, args ...any) {
	LogTable(logger, slog.LevelInfo, message, table, args...)
}

func WarnTable(logger *slog.Logger, message string, table string, args ...any) {
	LogTable(logger, slog.LevelWarn, message, table, args...)
}

func ErrorTable(logger *slog.Logger, message string, table string, args ...any) {
	LogTable(logger, slog.LevelError, message, table, args...)
}

func LogTable(logger *slog.Logger, level slog.Level, message string, table string, args ...any) {
	if logger == nil {
		return
	}
	logger.Log(context.Background(), level, message, args...)
	if strings.TrimSpace(table) == "" {
		return
	}
	logger.Log(context.Background(), level, ensureLeadingNewline(table), slog.Bool(tableRecordAttr, true))
}

type InboundRow struct {
	Tag        string
	UsersCount int
	Hash       string
}

func Value(value any) string {
	switch typed := value.(type) {
	case nil:
		return EmptyValue
	case string:
		return Truncate(typed, DefaultMaxValueLength)
	case *string:
		if typed == nil {
			return EmptyValue
		}
		return Truncate(*typed, DefaultMaxValueLength)
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case time.Duration:
		return Duration(typed)
	default:
		return Truncate(fmt.Sprint(value), DefaultMaxValueLength)
	}
}

func Bool(value bool) string {
	return strconv.FormatBool(value)
}

func Duration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	return duration.Round(time.Millisecond).String()
}

func ShortHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return EmptyValue
	}
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func Truncate(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return EmptyValue
	}
	if maxLength <= 0 || len(value) <= maxLength {
		return value
	}
	if maxLength <= 1 {
		return value[:maxLength]
	}
	if maxLength <= 3 {
		return value[:maxLength]
	}
	return value[:maxLength-3] + "..."
}

func RedactText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = pemBlockPattern.ReplaceAllString(value, "[REDACTED_PEM]")
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = jwtPattern.ReplaceAllString(value, "[REDACTED_JWT]")
	value = keyValuePattern.ReplaceAllString(value, `${1}${2}[REDACTED]`)
	value = jsonSensitivePattern.ReplaceAllString(value, `${1}[REDACTED]${3}`)
	return value
}

func center(value string, width int) string {
	if len(value) >= width {
		return value
	}
	left := (width - len(value)) / 2
	right := width - len(value) - left
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func ensureLeadingNewline(value string) string {
	if strings.HasPrefix(value, "\n") {
		return value
	}
	return "\n" + value
}

var (
	pemBlockPattern      = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]+-----.*?-----END [A-Z0-9 ]+-----`)
	bearerPattern        = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/\-]+=*`)
	jwtPattern           = regexp.MustCompile(`\b[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	keyValuePattern      = regexp.MustCompile(`(?i)\b(secret[_-]?key|api[_-]?key|apikey|token|password|private[_-]?key|node[_-]?key|cert[_-]?pem|authorization|short[_-]?id|id|uuid)(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s"',;}&]+)`)
	jsonSensitivePattern = regexp.MustCompile(`(?i)("?(?:id|uuid|password|privateKey|private_key|shortId|short_id|secretKey|secret_key|token|authorization)"?\s*:\s*")([^"]*)(")`)
)
