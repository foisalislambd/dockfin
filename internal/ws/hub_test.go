package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestHubDropsSlowSubscribersAndIgnoresOthers(t *testing.T) {
	h := NewHub()
	id := uuid.New()
	other := uuid.New()
	ch := h.Subscribe(id)
	slow := h.Subscribe(id)
	for range 80 {
		h.Publish(id, map[string]string{"line": "ok"})
	}
	got := 0
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				t.Fatal("closed")
			}
			got++
		default:
			goto done
		}
	}
done:
	if got == 0 || got > 64 {
		t.Fatalf("buffered %d", got)
	}
	h.Publish(other, map[string]string{"line": "no"})
	select {
	case msg := <-slow:
		if string(msg) == "" {
			t.Fatal("empty")
		}
	default:
	}
	h.Unsubscribe(id, ch)
	h.Unsubscribe(id, slow)
	h.Publish(id, map[string]string{"line": "after"})

	w := &plainWriter{h: make(http.Header)}
	h.SSEHandler(id)(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.code != http.StatusInternalServerError {
		t.Fatal(w.code)
	}
}

type plainWriter struct {
	h    http.Header
	code int
}

func (p *plainWriter) Header() http.Header { return p.h }
func (p *plainWriter) Write(b []byte) (int, error) {
	if p.code == 0 {
		p.code = http.StatusOK
	}
	return len(b), nil
}
func (p *plainWriter) WriteHeader(code int) { p.code = code }
