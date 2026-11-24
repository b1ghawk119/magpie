package gateway

import (
	"context"
	"encoding/json"
	"github.com/yetone/magpie/internal/codexsummary"
	"github.com/yetone/magpie/internal/provider"
	"net/http"
	"strings"
)

// exporterOnly marks a summary call that can only go to the recorded writer.
// It inherits the caller's gateway identity and ordinary scheduling limits.
type exporterOnly struct{}

// exportCheckpoint asks the writer to describe just its own opaque item. Later
// user turns and tool results stay on the chosen route. Multiple seals require
// separate provenance and are left intact rather than folded speculatively.
func (s *Server) exportCheckpoint(r *http.Request, body []byte, writer checkpointWriter) ([]byte, bool) {
	if writer.Model == "" || len(sealedCompactions(body)) != 1 {
		return nil, false
	}
	input := compactInput(body)
	for i, item := range input {
		if len(sealedCompactions(withFields([]byte(`{}`), map[string]any{"input": []json.RawMessage{item}}))) == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(r.Context(), compactWithin)
		defer cancel()
		ctx = context.WithValue(ctx, exporterOnly{}, true)
		ctx = context.WithValue(ctx, unaryCompactKey{}, false)
		req := r.Clone(ctx)
		req.URL.Path = "/v1/responses"
		req.URL.RawQuery = ""
		req.Header.Del("X-Codex-Turn-State")
		req.Header.Del("previous_response_id")
		source, _ := json.Marshal(map[string]any{"model": writer.Provider + "/" + writer.Model, "stream": true, "store": false, "tools": []any{}, "instructions": codexCompactPrompt, "input": []any{item, userMessage(codexCompactPrompt)}})
		rec := &recorder{header: http.Header{}, status: 200}
		s.serve(rec, askWhole(req), provider.Responses, source)
		if rec.status >= 400 {
			return nil, false
		}
		reply, err := compactReply(rec.body.Bytes())
		text := strings.TrimSpace(reply.summary())
		if err != nil || reply.Status != "completed" || reply.Error != nil || calledTool(reply.Output) || text == "" {
			return nil, false
		}
		input[i], _ = json.Marshal(codexsummary.UserMessage(codexsummary.Prefix + "\n" + text))
		return withFields(body, map[string]any{"input": input}), true
	}
	return nil, false
}
