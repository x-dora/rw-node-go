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
	InboundProtocols         map[string]string
	Plugins                  PluginState
}

type RestartReason string

const (
	RestartReasonForce                 RestartReason = "force_restart"
	RestartReasonCoreNotRunning        RestartReason = "core_not_running"
	RestartReasonNoPreviousHashes      RestartReason = "no_previous_hashes"
	RestartReasonEmptyConfigHashChange RestartReason = "empty_config_hash_changed"
	RestartReasonInboundCountChange    RestartReason = "inbound_count_changed"
	RestartReasonInboundRemoved        RestartReason = "inbound_removed"
	RestartReasonInboundHashChange     RestartReason = "inbound_hash_changed"
	RestartReasonNoRestart             RestartReason = "up_to_date"
)

type RestartDecision struct {
	ShouldRestart bool
	Reason        RestartReason
	InboundTag    string
	PreviousHash  string
	IncomingHash  string
}

func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		NodeVersion:      version.NodeVersion,
		CurrentConfig:    map[string]any{},
		InboundUsers:     map[string]map[string]struct{}{},
		KnownInboundTag:  map[string]struct{}{},
		InboundProtocols: map[string]string{},
		Plugins:          PluginState{},
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
	s.XrayVersion = cloneStringPtr(version)
	s.CurrentConfig = cloneMap(currentConfig)
	s.LastHashes = cloneHashes(hashes)
	s.HasLastHashes = true
	s.InboundUsers = map[string]map[string]struct{}{}
	s.KnownInboundTag = map[string]struct{}{}
	s.InboundProtocols = map[string]string{}
	for _, inbound := range s.LastHashes.Inbounds {
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
		XrayVersion:              cloneStringPtr(s.XrayVersion),
		NodeVersion:              s.NodeVersion,
		CurrentConfig:            cloneMap(s.CurrentConfig),
		LastHashes:               cloneHashes(s.LastHashes),
	}
}

func (s *RuntimeState) RestartDecision(force bool, hashes Hashes, coreRunning bool) RestartDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if force || !coreRunning {
		if force {
			return RestartDecision{ShouldRestart: true, Reason: RestartReasonForce}
		}
		return RestartDecision{ShouldRestart: true, Reason: RestartReasonCoreNotRunning}
	}
	if !s.HasLastHashes {
		return RestartDecision{ShouldRestart: true, Reason: RestartReasonNoPreviousHashes}
	}
	if sameHashes(s.LastHashes, hashes) {
		return RestartDecision{Reason: RestartReasonNoRestart}
	}
	if s.LastHashes.EmptyConfig != hashes.EmptyConfig {
		return RestartDecision{
			ShouldRestart: true,
			Reason:        RestartReasonEmptyConfigHashChange,
			PreviousHash:  s.LastHashes.EmptyConfig,
			IncomingHash:  hashes.EmptyConfig,
		}
	}
	if len(s.LastHashes.Inbounds) != len(hashes.Inbounds) {
		return RestartDecision{ShouldRestart: true, Reason: RestartReasonInboundCountChange}
	}

	incomingByTag := make(map[string]InboundHash, len(hashes.Inbounds))
	for _, inbound := range hashes.Inbounds {
		incomingByTag[inbound.Tag] = inbound
	}
	for _, previous := range s.LastHashes.Inbounds {
		incoming, ok := incomingByTag[previous.Tag]
		if !ok {
			return RestartDecision{
				ShouldRestart: true,
				Reason:        RestartReasonInboundRemoved,
				InboundTag:    previous.Tag,
				PreviousHash:  previous.Hash,
			}
		}
		if previous.Hash != incoming.Hash || previous.UsersCount != incoming.UsersCount {
			return RestartDecision{
				ShouldRestart: true,
				Reason:        RestartReasonInboundHashChange,
				InboundTag:    previous.Tag,
				PreviousHash:  previous.Hash,
				IncomingHash:  incoming.Hash,
			}
		}
	}
	return RestartDecision{ShouldRestart: true, Reason: RestartReasonInboundHashChange}
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
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneValue(value)
	}
	return output
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		output := make([]any, len(typed))
		for i, item := range typed {
			output[i] = cloneValue(item)
		}
		return output
	default:
		return value
	}
}

func cloneHashes(input Hashes) Hashes {
	output := Hashes{
		EmptyConfig: input.EmptyConfig,
		Inbounds:    make([]InboundHash, len(input.Inbounds)),
	}
	copy(output.Inbounds, input.Inbounds)
	return output
}

func cloneStringPtr(input *string) *string {
	if input == nil {
		return nil
	}
	value := *input
	return &value
}
