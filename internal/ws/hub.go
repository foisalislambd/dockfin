package ws

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uuid.UUID]map[chan []byte]struct{})}
}

func (h *Hub) Subscribe(deploymentID uuid.UUID) chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[deploymentID] == nil {
		h.clients[deploymentID] = make(map[chan []byte]struct{})
	}
	h.clients[deploymentID][ch] = struct{}{}
	return ch
}

func (h *Hub) Unsubscribe(deploymentID uuid.UUID, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.clients[deploymentID]; ok {
		delete(m, ch)
		close(ch)
		if len(m) == 0 {
			delete(h.clients, deploymentID)
		}
	}
}

func (h *Hub) Publish(deploymentID uuid.UUID, event any) {
	b, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients[deploymentID] {
		select {
		case ch <- b:
		default:
		}
	}
}

// SSEHandler streams deployment log events via Server-Sent Events.
func (h *Hub) SSEHandler(deploymentID uuid.UUID) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := h.Subscribe(deploymentID)
		defer h.Unsubscribe(deploymentID, ch)

		notify := r.Context().Done()
		for {
			select {
			case <-notify:
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				_, _ = w.Write([]byte("data: "))
				_, _ = w.Write(msg)
				_, _ = w.Write([]byte("\n\n"))
				flusher.Flush()
			}
		}
	}
}
