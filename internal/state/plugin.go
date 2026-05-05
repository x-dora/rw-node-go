package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/x-dora/rw-node-go/internal/contracts"
)

type PluginState struct {
	ActivePlugin               *PluginMetadata
	LastConfigHash             string
	TorrentBlockerEnabled      bool
	TorrentBlockerDuration     int
	TorrentBlockerIgnoredIPs   map[string]struct{}
	TorrentBlockerIgnoredUsers map[string]struct{}
	TorrentBlockerRuleTags     map[string]struct{}
	TorrentBlockerReports      []contracts.TorrentBlockerReport
	NftablesEnabled            bool
	ConfigByName               map[string]map[string]any
}

type PluginMetadata struct {
	UUID string
	Name string
}

type TorrentBlockerConfig struct {
	Enabled         bool
	BlockDuration   int
	IgnoredIPs      []string
	IgnoredUsers    []string
	IncludeRuleTags []string
}

type TorrentBlockerSnapshot struct {
	Enabled         bool
	BlockDuration   int
	IgnoredIPs      []string
	IgnoredUsers    []string
	IncludeRuleTags []string
	ReportsCount    int
}

func (s *RuntimeState) HasActivePlugin() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Plugins.ActivePlugin != nil
}

func (s *RuntimeState) ResetPlugins() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Plugins.ActivePlugin = nil
	s.Plugins.LastConfigHash = ""
	s.resetTorrentBlockerLocked()
}

func (s *RuntimeState) SyncTorrentBlockerPlugin(uuid string, name string, config map[string]any, torrent TorrentBlockerConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Plugins.ActivePlugin = &PluginMetadata{UUID: uuid, Name: name}
	s.Plugins.LastConfigHash = HashPluginConfig(config)
	s.Plugins.TorrentBlockerReports = nil

	if !torrent.Enabled {
		s.resetTorrentBlockerConfigLocked()
		return
	}

	s.Plugins.TorrentBlockerEnabled = true
	s.Plugins.TorrentBlockerDuration = torrent.BlockDuration
	s.Plugins.TorrentBlockerIgnoredIPs = stringSet(torrent.IgnoredIPs)
	s.Plugins.TorrentBlockerIgnoredUsers = stringSet(torrent.IgnoredUsers)
	s.Plugins.TorrentBlockerRuleTags = stringSet(torrent.IncludeRuleTags)
}

func (s *RuntimeState) TorrentBlockerSnapshot() TorrentBlockerSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return TorrentBlockerSnapshot{
		Enabled:         s.Plugins.TorrentBlockerEnabled,
		BlockDuration:   s.Plugins.TorrentBlockerDuration,
		IgnoredIPs:      sortedKeys(s.Plugins.TorrentBlockerIgnoredIPs),
		IgnoredUsers:    sortedKeys(s.Plugins.TorrentBlockerIgnoredUsers),
		IncludeRuleTags: sortedKeys(s.Plugins.TorrentBlockerRuleTags),
		ReportsCount:    len(s.Plugins.TorrentBlockerReports),
	}
}

func (s *RuntimeState) AddTorrentBlockerReport(webhook contracts.XrayWebhookReport, ip string, userID string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.Plugins.TorrentBlockerEnabled || s.Plugins.TorrentBlockerDuration < 0 {
		return false
	}
	if _, ok := defaultIgnoredIPs[ip]; ok {
		return false
	}
	if _, ok := s.Plugins.TorrentBlockerIgnoredIPs[ip]; ok {
		return false
	}
	if _, ok := s.Plugins.TorrentBlockerIgnoredUsers[userID]; ok {
		return false
	}

	duration := s.Plugins.TorrentBlockerDuration
	report := contracts.TorrentBlockerReport{
		ActionReport: contracts.TorrentBlockerActionReport{
			Blocked:       false,
			IP:            ip,
			BlockDuration: duration,
			WillUnblockAt: now.Add(time.Duration(duration) * time.Second).UTC().Format(time.RFC3339),
			UserID:        userID,
			ProcessedAt:   now.UTC().Format(time.RFC3339),
		},
		XrayReport: webhook,
	}
	s.Plugins.TorrentBlockerReports = append(s.Plugins.TorrentBlockerReports, report)
	return true
}

func (s *RuntimeState) FlushTorrentBlockerReports() []contracts.TorrentBlockerReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	reports := append([]contracts.TorrentBlockerReport(nil), s.Plugins.TorrentBlockerReports...)
	s.Plugins.TorrentBlockerReports = nil
	return reports
}

func (s *RuntimeState) TorrentBlockerReportsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Plugins.TorrentBlockerReports)
}

func HashPluginConfig(config map[string]any) string {
	data, err := json.Marshal(config)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func (s *RuntimeState) resetTorrentBlockerLocked() {
	s.resetTorrentBlockerConfigLocked()
	s.Plugins.TorrentBlockerReports = nil
}

func (s *RuntimeState) resetTorrentBlockerConfigLocked() {
	s.Plugins.TorrentBlockerEnabled = false
	s.Plugins.TorrentBlockerDuration = 0
	s.Plugins.TorrentBlockerIgnoredIPs = nil
	s.Plugins.TorrentBlockerIgnoredUsers = nil
	s.Plugins.TorrentBlockerRuleTags = nil
}

func stringSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var defaultIgnoredIPs = map[string]struct{}{
	"::":              {},
	"::1":             {},
	"0.0.0.0":         {},
	"0.0.0.0/0":       {},
	"127.0.0.0/8":     {},
	"127.0.0.1":       {},
	"255.255.255.255": {},
}
