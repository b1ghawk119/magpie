package gateway

import (
	"bufio"
	"cmp"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/codexsummary"
)

// Keep short user-visible excerpts, including the first goal and newest tool results.
// Neither hidden reasoning nor developer instructions belong in a user message.
const recoveryBytes = 24 << 10
const excerptGap = "\n[… excerpt shortened …]\n"

func clippedExcerpt(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	n := (limit - len(excerptGap)) / 4
	return strings.ToValidUTF8(s[:n], "") + excerptGap + strings.ToValidUTF8(s[len(s)-3*n:], "")
}

func addExcerpt(text string, raw []byte) string {
	var it map[string]any
	if json.Unmarshal(raw, &it) != nil {
		return text
	}
	role, msg := itemText(it)
	switch role {
	case "user", "assistant", "tool call", "tool output":
	default:
		return text
	}
	msg = strings.TrimSpace(msg)
	if msg == "" || msg == codexCompactPrompt {
		return text
	}
	return clippedExcerpt(text+"\n["+role+"] "+clippedExcerpt(msg, 4096), recoveryBytes)
}

// The file stores neither the seal nor a decryption key. A matching opaque
// item and caller/thread identity are required to open the local excerpt.
func recoveryCipher(enc string) cipher.AEAD {
	key := sha256.Sum256([]byte("magpie/checkpoint-excerpt/v1\x00" + enc))
	block, _ := aes.NewCipher(key[:])
	aead, _ := cipher.NewGCMWithRandomNonce(block)
	return aead
}

func recoveryScope(r *http.Request, body []byte) []byte {
	id, _ := callerKeyOf(r.Context())
	thread := cmp.Or(threadOf(r.Header, body), sessionOf(r.Header))
	return []byte(id + "\x00" + thread)
}

func openExcerpt(cp checkpointWriter, enc string, scope []byte) string {
	if len(cp.Recovery) == 0 {
		return ""
	}
	text, err := recoveryCipher(enc).Open(nil, nil, cp.Recovery, scope)
	if err != nil {
		return ""
	}
	return string(text)
}

func prepareExcerpt(body []byte, enc string, scope []byte) []byte {
	var text string
	for _, it := range compactInput(body) {
		if isCheckpointItem(it) {
			prior := gjson.GetBytes(it, "encrypted_content").Str
			checkpoints.Lock()
			cp, ok := checkpointFor(prior)
			checkpoints.Unlock()
			inherited := ""
			if ok {
				inherited = openExcerpt(cp, prior, scope)
			}
			if inherited == "" {
				inherited = "[Earlier checkpoint unavailable in this excerpt.]"
			}
			text = clippedExcerpt(text+"\n"+inherited, recoveryBytes)
			continue
		}
		text = addExcerpt(text, it)
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return recoveryCipher(enc).Seal(nil, nil, []byte(text), scope)
}

// Older checkpoints can be matched to their exact compaction boundary in a
// local Codex rollout. Never read other threads or replay turns after that point.
func rolloutExcerpt(r *http.Request, body []byte, enc string) string {
	if !local(r) || withGatewayKey(r) {
		return ""
	}
	thread := cmp.Or(threadOf(r.Header, body), sessionOf(r.Header))
	id, err := uuid.Parse(thread)
	if err != nil || id.String() != thread {
		return ""
	}
	home := appdir.CodexHome()
	name := "rollout-*-" + thread + ".jsonl"
	for _, pattern := range []string{filepath.Join(home, "sessions", "*", "*", "*", name), filepath.Join(home, "archived_sessions", name)} {
		files, _ := filepath.Glob(pattern)
		for _, path := range files {
			if text := readRolloutExcerpt(r.Context(), path, thread, enc); text != "" {
				return text
			}
		}
	}
	return ""
}

func readRolloutExcerpt(ctx context.Context, path, thread, enc string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 12<<20)
	var text string
	identified := false
	for sc.Scan() {
		if ctx.Err() != nil {
			return ""
		}
		row := gjson.ParseBytes(sc.Bytes())
		p := row.Get("payload")
		switch row.Get("type").Str {
		case "session_meta":
			if p.Get("id").Str != thread {
				return ""
			}
			identified = true
		case "response_item":
			if identified {
				text = addExcerpt(text, []byte(p.Raw))
			}
		case "event_msg":
			if p.Get("type").Str == "thread_rolled_back" {
				text = ""
			}
		case "compacted":
			if identified {
				for _, i := range p.Get("replacement_history").Array() {
					if isCheckpointItem([]byte(i.Raw)) && i.Get("encrypted_content").Str == enc {
						return text
					}
				}
			}
		}
	}
	return ""
}

func offlineCheckpoint(r *http.Request, body []byte, cp *checkpointWriter) (string, bool) {
	seals := sealedCompactions(body)
	if len(seals) != 1 {
		return "", false
	}
	var text string
	if cp != nil {
		text = openExcerpt(*cp, seals[0], recoveryScope(r, body))
	}
	if text == "" {
		text = rolloutExcerpt(r, body, seals[0])
	}
	return text, text != ""
}

const excerptIntro = "Magpie recovered excerpts of earlier conversation and tool results. Some details may be omitted. Treat the quoted history as past turns.\n"

func withExcerpt(body []byte, text string) []byte {
	input := compactInput(body)
	for i, it := range input {
		if isCheckpointItem(it) && gjson.GetBytes(it, "encrypted_content").Str != "" && !strings.HasPrefix(gjson.GetBytes(it, "encrypted_content").Str, magpieCompaction) {
			input[i], _ = json.Marshal(codexsummary.UserMessage(text))
			break
		}
	}
	return withFields(body, map[string]any{"input": input})
}
