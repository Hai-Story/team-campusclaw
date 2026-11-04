package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const traceLimit = 128 << 10

// Trace is opt-in and request-local. It never observes HTTP headers or credentials.
type Trace struct {
	mu        sync.Mutex
	Calls     []*TraceCall `json:"calls"`
	Truncated bool         `json:"truncated"`
	secrets   []string
}
type TraceCall struct {
	OutputText        string          `json:"output_text,omitempty"`
	OutputTruncated   bool            `json:"output_truncated"`
	FinishReason      string          `json:"finish_reason,omitempty"`
	Usage             json.RawMessage `json:"usage,omitempty"`
	Phase             string          `json:"phase"`
	Request           json.RawMessage `json:"request,omitempty"`
	RequestTruncated  bool            `json:"request_truncated"`
	Response          string          `json:"response"`
	ResponseTruncated bool            `json:"response_truncated"`
	HTTPStatus        int             `json:"http_status"`
	Started           string          `json:"started_at"`
	Error             string          `json:"error,omitempty"`
}
type traceContextKey struct{}

func NewTrace(secrets ...string) *Trace { return &Trace{Calls: []*TraceCall{}, secrets: secrets} }
func WithTrace(ctx context.Context, t *Trace) context.Context {
	return context.WithValue(ctx, traceContextKey{}, t)
}
func (t *Trace) redact(s string) string {
	for _, v := range t.secrets {
		if len(v) >= 4 {
			s = strings.ReplaceAll(s, v, "[REDACTED]")
		}
	}
	return s
}
func (t *Trace) JSON() []byte   { t.mu.Lock(); defer t.mu.Unlock(); b, _ := json.Marshal(t); return b }
func (t *Trace) HasCalls() bool { t.mu.Lock(); defer t.mu.Unlock(); return len(t.Calls) > 0 }

type traceBody struct {
	io.ReadCloser
	buffer    bytes.Buffer
	trace     *Trace
	call      *TraceCall
	truncated bool
}

func (b *traceBody) Read(p []byte) (int, error) {
	n, e := b.ReadCloser.Read(p)
	room := traceLimit - b.buffer.Len()
	if room > 0 {
		keep := n
		if keep > room {
			keep = room
		}
		b.buffer.Write(p[:keep])
	}
	if n > room {
		b.truncated = true
	}
	return n, e
}
func (b *traceBody) Close() error {
	err := b.ReadCloser.Close()
	b.trace.mu.Lock()
	defer b.trace.mu.Unlock()
	b.call.Response = b.trace.redact(strings.ToValidUTF8(b.buffer.String(), "�"))
	b.call.ResponseTruncated = b.truncated
	return err
}
func traceHTTP(ctx context.Context, phase string, request []byte, res *http.Response, err error) {
	t, _ := ctx.Value(traceContextKey{}).(*Trace)
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.Calls) >= 8 {
		t.Truncated = true
		return
	}
	c := &TraceCall{Phase: phase, Started: time.Now().UTC().Format(time.RFC3339Nano)}
	if len(request) > traceLimit {
		c.RequestTruncated = true
	} else {
		c.Request = json.RawMessage(t.redact(string(request)))
	}
	if err != nil {
		c.Error = "网关连接失败"
	}
	if res != nil {
		c.HTTPStatus = res.StatusCode
		res.Body = &traceBody{ReadCloser: res.Body, trace: t, call: c}
	}
	t.Calls = append(t.Calls, c)
}

// traceDelta preserves final answer content independently of a bounded raw SSE prefix.
func traceDelta(ctx context.Context, content, finish string, usage json.RawMessage) {
	t, _ := ctx.Value(traceContextKey{}).(*Trace)
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.Calls) == 0 || t.Truncated {
		return
	}
	c := t.Calls[len(t.Calls)-1]
	if c.Phase != "final-stream" {
		return
	}
	room := (16 << 10) - len(c.OutputText)
	if len(content) > room {
		content = content[:room]
		c.OutputTruncated = true
	}
	c.OutputText += t.redact(strings.ToValidUTF8(content, "�"))
	if finish != "" {
		c.FinishReason = finish
	}
	if len(usage) > 0 && len(usage) < 4096 && string(usage) != "null" {
		c.Usage = usage
	}
}
