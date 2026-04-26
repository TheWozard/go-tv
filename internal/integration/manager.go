package integration

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"go-tv/internal/player"
	"go-tv/internal/schedule"
	"go-tv/internal/state"
)

// Manager holds named player integrations and keeps at most one active at a time.
type Manager struct {
	ctx     context.Context
	players map[string]player.Player
	sched   *schedule.Schedule
	state   *state.State
	active  string
	cancel  context.CancelFunc
	mu      sync.Mutex
}

func NewManager(ctx context.Context, sched *schedule.Schedule, st *state.State) *Manager {
	return &Manager{
		ctx:     ctx,
		players: make(map[string]player.Player),
		sched:   sched,
		state:   st,
	}
}

func (m *Manager) Register(name string, p player.Player) {
	m.players[name] = p
}

// Names returns the registered integration names in sorted order.
func (m *Manager) Names() []string {
	names := make([]string, 0, len(m.players))
	for name := range m.players {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Active returns the name of the currently running integration, or "".
func (m *Manager) Active() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// Activate stops the current player (if any) and starts the named one.
func (m *Manager) Activate(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.players[name]
	if !ok {
		return fmt.Errorf("unknown integration %q", name)
	}
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.active = name
	go player.Run(ctx, p, m.sched, m.state)
	return nil
}

// Stop cancels the active player without starting a new one.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.active = ""
}
