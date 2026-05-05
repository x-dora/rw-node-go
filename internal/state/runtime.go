package state

import (
	"sync"

	"github.com/x-dora/rw-node-go/internal/version"
)

type RuntimeState struct {
	mu sync.RWMutex

	XrayRunning     bool
	XrayVersion     string
	NodeVersion     string
	CurrentConfig   map[string]any
	LastHashes      Hashes
	InboundUsers    map[string]map[string]struct{}
	KnownInboundTag map[string]struct{}
	Plugins         PluginState
}

func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		XrayVersion:     "",
		NodeVersion:     version.Version,
		CurrentConfig:   map[string]any{},
		InboundUsers:    map[string]map[string]struct{}{},
		KnownInboundTag: map[string]struct{}{},
		Plugins:         PluginState{},
	}
}

func (s *RuntimeState) IsXrayRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.XrayRunning
}

func (s *RuntimeState) SetXrayRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.XrayRunning = running
}
