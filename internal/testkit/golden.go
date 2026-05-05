package testkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type PanelAPIFixture struct {
	Version string         `json:"version"`
	Routes  []RouteFixture `json:"routes"`
}

type RouteFixture struct {
	Name     string          `json:"name"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Status   string          `json:"status"`
	Request  json.RawMessage `json:"request,omitempty"`
	Response json.RawMessage `json:"response"`
}

func LoadPanelAPIFixture(t testing.TB) PanelAPIFixture {
	t.Helper()

	path := filepath.Join(ProjectRoot(t), "testdata", "contracts", "official-2.7.0", "panel-api.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read panel api fixture: %v", err)
	}

	var fixture PanelAPIFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal panel api fixture: %v", err)
	}
	if fixture.Version == "" {
		t.Fatalf("panel api fixture version is empty")
	}
	if len(fixture.Routes) == 0 {
		t.Fatalf("panel api fixture has no routes")
	}
	return fixture
}

func ProjectRoot(t testing.TB) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find project root from %q", dir)
		}
		dir = parent
	}
}

func StrictDecode[T any](data json.RawMessage) (T, error) {
	var value T
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return value, fmt.Errorf("unexpected extra JSON data")
	}
	return value, nil
}

func MustStrictDecode[T any](t testing.TB, data json.RawMessage) T {
	t.Helper()

	value, err := StrictDecode[T](data)
	if err != nil {
		t.Fatalf("strict decode %s: %v", reflect.TypeOf(value), err)
	}
	return value
}

func CanonicalJSON(t testing.TB, value any) []byte {
	t.Helper()

	var decoded any
	switch typed := value.(type) {
	case json.RawMessage:
		if err := json.Unmarshal(typed, &decoded); err != nil {
			t.Fatalf("unmarshal JSON: %v", err)
		}
	case []byte:
		if err := json.Unmarshal(typed, &decoded); err != nil {
			t.Fatalf("unmarshal JSON: %v", err)
		}
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			t.Fatalf("marshal JSON: %v", err)
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal marshaled JSON: %v", err)
		}
	}

	output, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal canonical JSON: %v", err)
	}
	return output
}

func AssertCanonicalJSONEqual(t testing.TB, got any, want any) {
	t.Helper()

	gotJSON := CanonicalJSON(t, got)
	wantJSON := CanonicalJSON(t, want)
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("JSON mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}
