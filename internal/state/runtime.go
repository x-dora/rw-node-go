package state

import (
	"sync"

	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/version"
)

type RuntimeState struct {
	mu sync.RWMutex

	XrayRunning              bool
	XrayInternalStatusCached bool
	XrayVersion              *string
	NodeVersion              string
	CurrentConfig            map[string]any
	LastHashes               Hashes
	HasLastHashes            bool
	InboundUsers             map[string]map[string]struct{}
	KnownInboundTag          map[string]struct{}
	Plugins                  PluginState
}

func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		NodeVersion:     version.NodeVersion,
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
	if !running {
		s.XrayInternalStatusCached = false
	}
}

func (s *RuntimeState) SetXrayStarted(version *string, currentConfig map[string]any, hashes Hashes) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.XrayRunning = true
	s.XrayInternalStatusCached = true
	s.XrayVersion = version
	s.CurrentConfig = currentConfig
	s.LastHashes = hashes
	s.HasLastHashes = true
	s.KnownInboundTag = map[string]struct{}{}
	for _, inbound := range hashes.Inbounds {
		if inbound.Tag != "" {
			s.KnownInboundTag[inbound.Tag] = struct{}{}
		}
	}
}

func (s *RuntimeState) SetXrayInternalStatusCached(cached bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.XrayInternalStatusCached = cached
	if !cached {
		s.XrayRunning = false
	}
}

func (s *RuntimeState) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Snapshot{
		XrayRunning:              s.XrayRunning,
		XrayInternalStatusCached: s.XrayInternalStatusCached,
		XrayVersion:              s.XrayVersion,
		NodeVersion:              s.NodeVersion,
		CurrentConfig:            cloneMap(s.CurrentConfig),
		LastHashes:               s.LastHashes,
	}
}

func (s *RuntimeState) ShouldRestart(force bool, hashes Hashes, coreRunning bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if force || !coreRunning {
		return true
	}
	if !s.HasLastHashes {
		return true
	}
	return !sameHashes(s.LastHashes, hashes)
}

type Snapshot struct {
	XrayRunning              bool
	XrayInternalStatusCached bool
	XrayVersion              *string
	NodeVersion              string
	CurrentConfig            map[string]any
	LastHashes               Hashes
}

func HashesFromContract(hashes contracts.Hashes) Hashes {
	inbounds := make([]InboundHash, len(hashes.Inbounds))
	for i, inbound := range hashes.Inbounds {
		inbounds[i] = InboundHash{
			UsersCount: inbound.UsersCount,
			Hash:       inbound.Hash,
			Tag:        inbound.Tag,
		}
	}
	return Hashes{
		EmptyConfig: hashes.EmptyConfig,
		Inbounds:    inbounds,
	}
}

func sameHashes(a, b Hashes) bool {
	if a.EmptyConfig != b.EmptyConfig || len(a.Inbounds) != len(b.Inbounds) {
		return false
	}
	counts := map[InboundHash]int{}
	for _, inbound := range a.Inbounds {
		counts[inbound]++
	}
	for _, inbound := range b.Inbounds {
		counts[inbound]--
		if counts[inbound] < 0 {
			return false
		}
	}
	return true
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
