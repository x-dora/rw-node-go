package system

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	gopsutilcpu "github.com/shirou/gopsutil/v4/cpu"
	gopsutilhost "github.com/shirou/gopsutil/v4/host"
	gopsutilload "github.com/shirou/gopsutil/v4/load"
	gopsutilmem "github.com/shirou/gopsutil/v4/mem"
	gopsutilnet "github.com/shirou/gopsutil/v4/net"
	"github.com/x-dora/rw-node-go/internal/contracts"
)

const (
	defaultSampleInterval = time.Second
	procNetRoutePath      = "/proc/net/route"
)

type Snapshotter interface {
	SnapshotStats(ctx context.Context) contracts.SystemStatsPayload
	Close() error
}

type DefaultSnapshotter struct {
	provider systemProvider
	sampler  *NetworkSampler
}

func NewSnapshotter() *DefaultSnapshotter {
	provider := gopsutilProvider{}
	return NewSnapshotterWithProvider(provider, NewNetworkSampler(provider, defaultInterfaceResolver{}, defaultSampleInterval))
}

func NewSnapshotterWithProvider(provider systemProvider, sampler *NetworkSampler) *DefaultSnapshotter {
	return &DefaultSnapshotter{provider: provider, sampler: sampler}
}

func (s *DefaultSnapshotter) SnapshotStats(ctx context.Context) contracts.SystemStatsPayload {
	info := contracts.NodeSystemInfo{
		Arch:     runtime.GOARCH,
		CPUs:     runtime.NumCPU(),
		Platform: runtime.GOOS,
		Type:     runtime.GOOS,
		Version:  runtime.Version(),
	}
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	if hostInfo, err := s.provider.HostInfo(ctx); err == nil && hostInfo != nil {
		if hostInfo.Hostname != "" {
			info.Hostname = hostInfo.Hostname
		}
		if hostInfo.Platform != "" {
			info.Platform = hostInfo.Platform
		}
		if hostInfo.KernelVersion != "" {
			info.Release = hostInfo.KernelVersion
		}
		if hostInfo.OS != "" {
			info.Type = hostInfo.OS
		}
		if hostInfo.KernelArch != "" {
			info.Arch = hostInfo.KernelArch
		}
		if hostInfo.PlatformVersion != "" {
			info.Version = hostInfo.PlatformVersion
		}
	}

	if count, err := s.provider.CPUCounts(ctx); err == nil && count > 0 {
		info.CPUs = count
	}
	if cpuInfo, err := s.provider.CPUInfo(ctx); err == nil && len(cpuInfo) > 0 {
		info.CPUModel = firstCPUModel(cpuInfo)
	}
	if memory, err := s.provider.VirtualMemory(ctx); err == nil && memory != nil {
		info.MemoryTotal = memory.Total
	}
	if interfaces, err := s.provider.NetInterfaces(ctx); err == nil {
		info.NetworkInterfaces = interfaceNames(interfaces)
	}

	stats := contracts.NodeSystemStats{
		LoadAvg: []float64{0, 0, 0},
	}
	if memory, err := s.provider.VirtualMemory(ctx); err == nil && memory != nil {
		stats.MemoryFree = memory.Available
		if stats.MemoryFree == 0 {
			stats.MemoryFree = memory.Free
		}
		stats.MemoryUsed = memory.Used
	}
	if uptime, err := s.provider.Uptime(ctx); err == nil {
		stats.Uptime = uptime
	}
	if loadAvg, err := s.provider.LoadAvg(ctx); err == nil && loadAvg != nil {
		stats.LoadAvg = []float64{loadAvg.Load1, loadAvg.Load5, loadAvg.Load15}
	}
	if s.sampler != nil {
		stats.Interface = s.sampler.Snapshot()
	}

	return contracts.SystemStatsPayload{Info: info, Stats: stats}
}

func (s *DefaultSnapshotter) Close() error {
	if s.sampler == nil {
		return nil
	}
	return s.sampler.Close()
}

func SnapshotStats() contracts.SystemStatsPayload {
	return NewSnapshotterWithProvider(gopsutilProvider{}, nil).SnapshotStats(context.Background())
}

type NetworkSampler struct {
	provider  systemProvider
	resolver  interfaceResolver
	interval  time.Duration
	closeOnce sync.Once
	done      chan struct{}

	mu       sync.RWMutex
	previous *networkCounterSample
	current  *contracts.NetworkInterface
	now      func() time.Time
}

func NewNetworkSampler(provider systemProvider, resolver interfaceResolver, interval time.Duration) *NetworkSampler {
	if interval <= 0 {
		interval = defaultSampleInterval
	}
	sampler := &NetworkSampler{
		provider: provider,
		resolver: resolver,
		interval: interval,
		done:     make(chan struct{}),
		now:      time.Now,
	}
	sampler.sample(context.Background())
	go sampler.run()
	return sampler
}

func (s *NetworkSampler) Snapshot() *contracts.NetworkInterface {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return nil
	}
	snapshot := *s.current
	return &snapshot
}

func (s *NetworkSampler) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
	})
	return nil
}

func (s *NetworkSampler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.sample(context.Background())
		case <-s.done:
			return
		}
	}
}

func (s *NetworkSampler) sample(ctx context.Context) {
	iface, err := s.resolver.DefaultInterface()
	if err != nil || iface == "" {
		s.clear()
		return
	}
	counters, err := s.provider.NetIOCounters(ctx, true)
	if err != nil {
		s.clear()
		return
	}
	counter, ok := findCounter(counters, iface)
	if !ok {
		s.clear()
		return
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	current := networkCounterSample{
		iface: iface,
		rx:    counter.BytesRecv,
		tx:    counter.BytesSent,
		at:    now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.previous != nil && s.previous.iface == iface {
		elapsed := current.at.Sub(s.previous.at).Seconds()
		if elapsed > 0 {
			s.current = &contracts.NetworkInterface{
				Interface:     iface,
				RxBytesPerSec: perSecond(current.rx, s.previous.rx, elapsed),
				TxBytesPerSec: perSecond(current.tx, s.previous.tx, elapsed),
				RxTotal:       current.rx,
				TxTotal:       current.tx,
			}
		}
	}
	s.previous = &current
}

func (s *NetworkSampler) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previous = nil
	s.current = nil
}

type networkCounterSample struct {
	iface string
	rx    uint64
	tx    uint64
	at    time.Time
}

func perSecond(current uint64, previous uint64, elapsed float64) uint64 {
	if current < previous {
		return 0
	}
	rate := float64(current-previous) / elapsed
	if rate <= 0 {
		return 0
	}
	if rate >= float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(rate)
}

func findCounter(counters []gopsutilnet.IOCountersStat, iface string) (gopsutilnet.IOCountersStat, bool) {
	for _, counter := range counters {
		if counter.Name == iface {
			return counter, true
		}
	}
	return gopsutilnet.IOCountersStat{}, false
}

func firstCPUModel(items []gopsutilcpu.InfoStat) string {
	for _, item := range items {
		if item.ModelName != "" {
			return item.ModelName
		}
	}
	return ""
}

func interfaceNames(interfaces []gopsutilnet.InterfaceStat) []string {
	names := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		names = append(names, iface.Name)
	}
	return names
}

type interfaceResolver interface {
	DefaultInterface() (string, error)
}

type defaultInterfaceResolver struct{}

func (r defaultInterfaceResolver) DefaultInterface() (string, error) {
	if runtime.GOOS != "linux" {
		return "", errors.New("default interface resolution is only supported on linux")
	}
	file, err := os.Open(procNetRoutePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return defaultInterfaceFromRoute(file)
}

func defaultInterfaceFromRoute(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "00000000" {
			return fields[0], nil
		}
	}
	return "", errors.New("default route not found")
}

type systemProvider interface {
	HostInfo(ctx context.Context) (*gopsutilhost.InfoStat, error)
	Uptime(ctx context.Context) (uint64, error)
	VirtualMemory(ctx context.Context) (*gopsutilmem.VirtualMemoryStat, error)
	CPUCounts(ctx context.Context) (int, error)
	CPUInfo(ctx context.Context) ([]gopsutilcpu.InfoStat, error)
	LoadAvg(ctx context.Context) (*gopsutilload.AvgStat, error)
	NetInterfaces(ctx context.Context) ([]gopsutilnet.InterfaceStat, error)
	NetIOCounters(ctx context.Context, pernic bool) ([]gopsutilnet.IOCountersStat, error)
}

type gopsutilProvider struct{}

func (p gopsutilProvider) HostInfo(ctx context.Context) (*gopsutilhost.InfoStat, error) {
	return gopsutilhost.InfoWithContext(ctx)
}

func (p gopsutilProvider) Uptime(ctx context.Context) (uint64, error) {
	return gopsutilhost.UptimeWithContext(ctx)
}

func (p gopsutilProvider) VirtualMemory(ctx context.Context) (*gopsutilmem.VirtualMemoryStat, error) {
	return gopsutilmem.VirtualMemoryWithContext(ctx)
}

func (p gopsutilProvider) CPUCounts(ctx context.Context) (int, error) {
	return gopsutilcpu.CountsWithContext(ctx, true)
}

func (p gopsutilProvider) CPUInfo(ctx context.Context) ([]gopsutilcpu.InfoStat, error) {
	return gopsutilcpu.InfoWithContext(ctx)
}

func (p gopsutilProvider) LoadAvg(ctx context.Context) (*gopsutilload.AvgStat, error) {
	return gopsutilload.AvgWithContext(ctx)
}

func (p gopsutilProvider) NetInterfaces(ctx context.Context) ([]gopsutilnet.InterfaceStat, error) {
	return gopsutilnet.InterfacesWithContext(ctx)
}

func (p gopsutilProvider) NetIOCounters(ctx context.Context, pernic bool) ([]gopsutilnet.IOCountersStat, error) {
	return gopsutilnet.IOCountersWithContext(ctx, pernic)
}
