package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

type Manager struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*liveSession
	SSH      *sshx.Pool
	Logger   *slog.Logger
}

type liveSession struct {
	ID        uuid.UUID
	TeamID    uuid.UUID
	ServerID  uuid.UUID
	UserID    uuid.UUID
	Term      *Session
	Client    *ssh.Client
	CreatedAt time.Time
	cancel    context.CancelFunc
}

func NewManager(pool *sshx.Pool, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{sessions: make(map[uuid.UUID]*liveSession), SSH: pool, Logger: logger}
}

type CreateOpts struct {
	TeamID    uuid.UUID
	UserID    uuid.UUID
	ServerID  uuid.UUID
	Client    *ssh.Client
	Container string // optional docker exec target
}

func (m *Manager) Create(opts CreateOpts) (uuid.UUID, error) {
	cmd := ""
	if opts.Container != "" {
		cmd = DockerExec(opts.Container)
	}
	term, err := Start(opts.Client, cmd)
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	ls := &liveSession{
		ID: id, TeamID: opts.TeamID, ServerID: opts.ServerID, UserID: opts.UserID,
		Term: term, Client: opts.Client, CreatedAt: time.Now(), cancel: cancel,
	}
	m.mu.Lock()
	m.sessions[id] = ls
	m.mu.Unlock()
	go func() {
		<-ctx.Done()
		m.Close(id)
	}()
	// Idle timeout 30m
	go func() {
		t := time.NewTimer(30 * time.Minute)
		defer t.Stop()
		select {
		case <-ctx.Done():
		case <-t.C:
			cancel()
		}
	}()
	return id, nil
}

func (m *Manager) Get(id, teamID uuid.UUID) (*liveSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ls, ok := m.sessions[id]
	if !ok || ls.TeamID != teamID {
		return nil, fmt.Errorf("session not found")
	}
	return ls, nil
}

func (m *Manager) Close(id uuid.UUID) {
	m.mu.Lock()
	ls, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	ls.cancel()
	_ = ls.Term.Close()
}

type wsMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

func (m *Manager) ServeWS(ctx context.Context, conn *websocket.Conn, ls *liveSession) {
	defer m.Close(ls.ID)
	conn.SetReadLimit(1 << 20)

	var wg sync.WaitGroup
	wg.Add(2)

	// PTY → WS
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := ls.Term.Stdout.Read(buf)
			if n > 0 {
				werr := conn.Write(ctx, websocket.MessageText, mustJSON(wsMsg{Type: "stdout", Data: string(buf[:n])}))
				if werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WS → PTY
	go func() {
		defer wg.Done()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var msg wsMsg
			if json.Unmarshal(data, &msg) != nil {
				// raw bytes as stdin
				_, _ = ls.Term.Stdin.Write(data)
				continue
			}
			switch msg.Type {
			case "stdin":
				_, _ = io.WriteString(ls.Term.Stdin, msg.Data)
			case "resize":
				cols, rows := msg.Cols, msg.Rows
				if cols <= 0 {
					cols = 120
				}
				if rows <= 0 {
					rows = 40
				}
				_ = ls.Term.SSH.WindowChange(rows, cols)
			case "ping":
				_ = conn.Write(ctx, websocket.MessageText, mustJSON(wsMsg{Type: "pong"}))
			}
		}
	}()

	wg.Wait()
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
