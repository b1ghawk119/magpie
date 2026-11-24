package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// grokUpstream is a SuperGrok-like Responses upstream that can decrypt
// only what it sealed itself, in reasoning and in a compaction alike, and
// says so in the stream as xAI does.
type grokUpstream struct {
	mu  sync.Mutex
	got [][]string // the sealed content each request carried, by type
}

func (g *grokUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	var q struct {
		Input []sealedItem `json:"input"`
	}
	json.Unmarshal(b, &q)
	var got []string
	foreign := false
	for _, it := range q.Input {
		if it.Enc != "" {
			got = append(got, it.Type+":"+it.Enc)
			foreign = foreign || it.Enc != "sealed-by-grok"
		}
	}
	g.mu.Lock()
	g.got = append(g.got, got)
	g.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	if foreign {
		io.WriteString(w, sse(
			`data: {"type":"response.created","response":{"id":"r0","model":"grok-4"}}`,
			`data: {"type":"response.failed","response":{"id":"r0","error":{"code":"invalid_request_error","message":"`+xaiUndecryptable+`"}}}`))
		return
	}
	io.WriteString(w, sse(
		`data: {"type":"response.created","response":{"id":"r1","model":"grok-4"}}`,
		`data: {"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"rs_g","summary":[],"encrypted_content":"sealed-by-grok"}}`,
		`data: {"type":"response.output_text.delta","delta":"from grok"}`,
		`data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":7,"output_tokens":1}}}`))
}

func (g *grokUpstream) sent() [][]string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := g.got
	g.got = nil
	return out
}

// An imported conversation has no local writer record. Retry removable
// reasoning state, but retain the only checkpoint even if the reader refuses it.
func TestForeignCompactionToGrok(t *testing.T) {
	fresh(t)
	refusedSeals.Lock()
	refusedSeals.m = map[string]sealsRefused{}
	refusedSeals.Unlock()
	g := &grokUpstream{}
	up := httptest.NewServer(g)
	t.Cleanup(up.Close)
	if err := provider.Save(provider.Provider{ID: "grok1", Name: "Grok (SuperGrok)", Key: "k", Models: []string{"grok-4"}, Responses: up.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	s := New()
	user := func(text string) string {
		return `{"type":"message","role":"user","content":[{"type":"input_text","text":"` + text + `"}]}`
	}
	turn := func(items ...string) (int, string) {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", CodexPath+"/responses", strings.NewReader(`{"model":"grok1/grok-4","stream":true,"input":[`+strings.Join(items, ",")+`]}`))
		req.Header.Set("session_id", "thread-compacted")
		s.Handler().ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	compaction := `{"type":"compaction","id":"cmp_1","encrypted_content":"sealed-by-openai-compaction"}`
	reasoning := func(by string) string {
		return `{"type":"reasoning","id":"rs_` + by + `","summary":[],"encrypted_content":"sealed-by-` + by + `"}`
	}
	call := func(id string) string {
		return `{"type":"function_call","call_id":"` + id + `","name":"shell","arguments":"{}"},{"type":"function_call_output","call_id":"` + id + `","output":"ok"}`
	}

	history := []string{user("start"), compaction, user("go on"), reasoning("openai"), call("c1")}
	code, body := turn(history...)
	if strings.Contains(body, "from grok") || !strings.Contains(body, "decrypt") {
		t.Fatalf("missing refusal: %d %s", code, body)
	}
	got := g.sent()
	if len(got) != 2 || len(got[0]) != 2 || strings.Join(got[1], ",") != "compaction:sealed-by-openai-compaction" {
		t.Fatalf("grok was sent %q", got)
	}
}
