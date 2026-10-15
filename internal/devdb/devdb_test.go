package devdb

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestKindAndRedact(t *testing.T) {
	for in, want := range map[string]string{
		"postgres://u:p@h:5432/db": "postgres", "postgresql://h/db": "postgres",
		"mongodb://h:27017/app": "mongodb", "mongodb+srv://u:p@c.x.net/app": "mongodb",
	} {
		if got, err := Kind(in); err != nil || got != want {
			t.Errorf("Kind(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := Kind("mysql://h/db"); err == nil {
		t.Error("mysql must be rejected")
	}
	got := Redact("postgres://app:s3cret@db.dev:5432/app?sslmode=disable&password=x")
	if strings.Contains(got, "s3cret") || strings.Contains(got, "password=") || !strings.Contains(got, "app:xxxxx@db.dev") {
		t.Errorf("Redact leaked or mangled: %s", got)
	}
}

func TestPipelineRejectsWrites(t *testing.T) {
	if _, err := pipeline(`[{"$match":{"a":1}},{"$out":"x"}]`); err == nil {
		t.Error("$out must be rejected")
	}
	if _, err := pipeline(`[{"$merge":{"into":"x"}}]`); err == nil {
		t.Error("$merge must be rejected")
	}
	st, err := pipeline(`[{"$match":{"_id":{"$oid":"64b7f0000000000000000000"}}},{"$group":{"_id":"$s","n":{"$sum":1}}}]`)
	if err != nil || len(st) != 2 {
		t.Fatalf("valid pipeline: %v %v", st, err)
	}
	if mongoDBName("mongodb://u:p@h1,h2:27017/app?replicaSet=rs") != "app" || mongoDBName("mongodb://h") != "" {
		t.Error("mongoDBName")
	}
}

type fakeEngine struct{ calls []string }

func (f *fakeEngine) Tools() []Tool { return []Tool{{Name: "sql_query", InputSchema: schema(nil)}} }
func (f *fakeEngine) Close()        {}
func (f *fakeEngine) Call(_ context.Context, tool string, args json.RawMessage) (string, error) {
	f.calls = append(f.calls, tool+" "+string(args))
	return `{"n":1}`, nil
}

func TestServerProtocol(t *testing.T) {
	fe := &fakeEngine{}
	s := &Server{tools: fe.Tools(), open: func(context.Context) (Engine, error) { return fe, nil }}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sql_query","arguments":{"sql":"select 1"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"nope"}`,
	}, "\n")
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 { // the notification gets no reply
		t.Fatalf("want 4 replies, got %d:\n%s", len(lines), out.String())
	}
	for i, want := range []string{`"protocolVersion":"2025-06-18"`, `"name":"sql_query"`, `{\"n\":1}`, `"code":-32601`} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("reply %d missing %s: %s", i, want, lines[i])
		}
	}
	if len(fe.calls) != 1 || fe.calls[0] != `sql_query {"sql":"select 1"}` {
		t.Errorf("calls: %v", fe.calls)
	}
}
