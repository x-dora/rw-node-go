package system

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	gopsutilcpu "github.com/shirou/gopsutil/v4/cpu"
	gopsutilhost "github.com/shirou/gopsutil/v4/host"
	gopsutilload "github.com/shirou/gopsutil/v4/load"
	gopsutilmem "github.com/shirou/gopsutil/v4/mem"
	gopsutilnet "github.com/shirou/gopsutil/v4/net"
)

func TestSnapshotStatsMapsProviderData(t *testing.T) {
	provider := &fakeSystemProvider{
		hostInfo: &gopsutilhost.InfoStat{
			Hostname:        "node-1",
			Platform:        "ubuntu",
			PlatformVersion: "24.04",
			KernelVersion:   "6.8.0",
			KernelArch:      "x86_64",
			OS:              "linux",
		},
		uptime: 123,
		memory: &gopsutilmem.VirtualMemoryStat{
			Total:     1024,
			Available: 256,
			Free:      128,
			Used:      768,
		},
		cpuCount: 4,
		cpuInfo:  []gopsutilcpu.InfoStat{{ModelName: "fixture-cpu"}},
		load:     &gopsutilload.AvgStat{Load1: 1.1, Load5: 2.2, Load15: 3.3},
		interfaces: []gopsutilnet.InterfaceStat{
			{Name: "lo"},
			{Name: "eth0"},
		},
	}
	snapshotter := NewSnapshotterWithProvider(provider, nil)

	got := snapshotter.SnapshotStats(context.Background())

	if got.Info.Hostname != "node-1" || got.Info.Platform != "ubuntu" || got.Info.Release != "6.8.0" || got.Info.Type != "linux" || got.Info.Version != "24.04" {
		t.Fatalf("info = %#v", got.Info)
	}
	if got.Info.CPUs != 4 || got.Info.CPUModel != "fixture-cpu" || got.Info.MemoryTotal != 1024 {
		t.Fatalf("info = %#v", got.Info)
	}
	if len(got.Info.NetworkInterfaces) != 2 || got.Info.NetworkInterfaces[1] != "eth0" {
		t.Fatalf("network interfaces = %#v", got.Info.NetworkInterfaces)
	}
	if got.Stats.MemoryFree != 256 || got.Stats.MemoryUsed != 768 || got.Stats.Uptime != 123 {
		t.Fatalf("stats = %#v", got.Stats)
	}
	if len(got.Stats.LoadAvg) != 3 || got.Stats.LoadAvg[0] != 1.1 || got.Stats.LoadAvg[1] != 2.2 || got.Stats.LoadAvg[2] != 3.3 {
		t.Fatalf("loadAvg = %#v", got.Stats.LoadAvg)
	}
}

func TestSnapshotStatsFallsBackOnProviderErrors(t *testing.T) {
	provider := &fakeSystemProvider{
		hostErr:       errors.New("host"),
		uptimeErr:     errors.New("uptime"),
		memoryErr:     errors.New("memory"),
		cpuCountErr:   errors.New("cpu count"),
		cpuInfoErr:    errors.New("cpu info"),
		loadErr:       errors.New("load"),
		interfacesErr: errors.New("interfaces"),
	}
	snapshotter := NewSnapshotterWithProvider(provider, nil)

	got := snapshotter.SnapshotStats(context.Background())

	if got.Info.CPUs <= 0 || got.Info.Arch == "" || got.Info.Platform == "" || got.Info.Type == "" || got.Info.Version == "" {
		t.Fatalf("fallback info = %#v", got.Info)
	}
	if got.Info.CPUModel != "" || got.Info.MemoryTotal != 0 || len(got.Info.NetworkInterfaces) != 0 {
		t.Fatalf("fallback info = %#v", got.Info)
	}
	if got.Stats.MemoryFree != 0 || got.Stats.MemoryUsed != 0 || got.Stats.Uptime != 0 || got.Stats.Interface != nil {
		t.Fatalf("fallback stats = %#v", got.Stats)
	}
	if len(got.Stats.LoadAvg) != 3 || got.Stats.LoadAvg[0] != 0 || got.Stats.LoadAvg[1] != 0 || got.Stats.LoadAvg[2] != 0 {
		t.Fatalf("fallback loadAvg = %#v", got.Stats.LoadAvg)
	}
}

func TestDefaultInterfaceFromRoute(t *testing.T) {
	route := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"lo\t0000007F\t00000000\t0001\t0\t0\t0\t000000FF\t0\t0\t0\n" +
		"eth0\t00000000\t0102A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0\n"

	got, err := defaultInterfaceFromRoute(strings.NewReader(route))
	if err != nil {
		t.Fatalf("defaultInterfaceFromRoute returned error: %v", err)
	}
	if got != "eth0" {
		t.Fatalf("default interface = %q, want eth0", got)
	}
}

func TestDefaultInterfaceFromRouteMissingDefault(t *testing.T) {
	_, err := defaultInterfaceFromRoute(strings.NewReader("Iface\tDestination\nlo\t0000007F\n"))
	if err == nil {
		t.Fatalf("defaultInterfaceFromRoute returned nil error")
	}
}

func TestNetworkSamplerCalculatesRates(t *testing.T) {
	provider := &fakeSystemProvider{
		ioCountersSeq: [][]gopsutilnet.IOCountersStat{
			{{Name: "eth0", BytesRecv: 100, BytesSent: 200}},
			{{Name: "eth0", BytesRecv: 400, BytesSent: 800}},
		},
	}
	sampler := &NetworkSampler{
		provider: provider,
		resolver: fakeInterfaceResolver{iface: "eth0"},
		interval: time.Second,
		done:     make(chan struct{}),
		now:      sequenceClock(time.Unix(100, 0), time.Unix(101, 0)),
	}

	sampler.sample(context.Background())
	sampler.sample(context.Background())

	got := sampler.Snapshot()
	if got == nil {
		t.Fatalf("interface snapshot is nil")
	}
	if got.Interface != "eth0" || got.RxBytesPerSec != 300 || got.TxBytesPerSec != 600 || got.RxTotal != 400 || got.TxTotal != 800 {
		t.Fatalf("interface snapshot = %#v", got)
	}
}

func TestNetworkSamplerHandlesCounterResetAndMissingInterface(t *testing.T) {
	provider := &fakeSystemProvider{
		ioCountersSeq: [][]gopsutilnet.IOCountersStat{
			{{Name: "eth0", BytesRecv: 500, BytesSent: 500}},
			{{Name: "eth0", BytesRecv: 100, BytesSent: 50}},
			{{Name: "wlan0", BytesRecv: 1, BytesSent: 1}},
		},
	}
	sampler := &NetworkSampler{
		provider: provider,
		resolver: fakeInterfaceResolver{iface: "eth0"},
		interval: time.Second,
		done:     make(chan struct{}),
		now:      sequenceClock(time.Unix(100, 0), time.Unix(101, 0), time.Unix(102, 0)),
	}

	sampler.sample(context.Background())
	sampler.sample(context.Background())
	got := sampler.Snapshot()
	if got == nil || got.RxBytesPerSec != 0 || got.TxBytesPerSec != 0 {
		t.Fatalf("counter reset snapshot = %#v", got)
	}

	sampler.sample(context.Background())
	if got := sampler.Snapshot(); got != nil {
		t.Fatalf("missing interface snapshot = %#v, want nil", got)
	}
}

type fakeSystemProvider struct {
	hostInfo      *gopsutilhost.InfoStat
	hostErr       error
	uptime        uint64
	uptimeErr     error
	memory        *gopsutilmem.VirtualMemoryStat
	memoryErr     error
	cpuCount      int
	cpuCountErr   error
	cpuInfo       []gopsutilcpu.InfoStat
	cpuInfoErr    error
	load          *gopsutilload.AvgStat
	loadErr       error
	interfaces    []gopsutilnet.InterfaceStat
	interfacesErr error
	ioCountersSeq [][]gopsutilnet.IOCountersStat
	ioCountersErr error
	ioCall        int
}

func sequenceClock(times ...time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		if len(times) == 0 {
			return time.Unix(0, 0)
		}
		if index >= len(times) {
			return times[len(times)-1]
		}
		value := times[index]
		index++
		return value
	}
}

func (p *fakeSystemProvider) HostInfo(ctx context.Context) (*gopsutilhost.InfoStat, error) {
	return p.hostInfo, p.hostErr
}

func (p *fakeSystemProvider) Uptime(ctx context.Context) (uint64, error) {
	return p.uptime, p.uptimeErr
}

func (p *fakeSystemProvider) VirtualMemory(ctx context.Context) (*gopsutilmem.VirtualMemoryStat, error) {
	return p.memory, p.memoryErr
}

func (p *fakeSystemProvider) CPUCounts(ctx context.Context) (int, error) {
	return p.cpuCount, p.cpuCountErr
}

func (p *fakeSystemProvider) CPUInfo(ctx context.Context) ([]gopsutilcpu.InfoStat, error) {
	return p.cpuInfo, p.cpuInfoErr
}

func (p *fakeSystemProvider) LoadAvg(ctx context.Context) (*gopsutilload.AvgStat, error) {
	return p.load, p.loadErr
}

func (p *fakeSystemProvider) NetInterfaces(ctx context.Context) ([]gopsutilnet.InterfaceStat, error) {
	return p.interfaces, p.interfacesErr
}

func (p *fakeSystemProvider) NetIOCounters(ctx context.Context, pernic bool) ([]gopsutilnet.IOCountersStat, error) {
	if p.ioCountersErr != nil {
		return nil, p.ioCountersErr
	}
	if len(p.ioCountersSeq) == 0 {
		return nil, nil
	}
	index := p.ioCall
	if index >= len(p.ioCountersSeq) {
		index = len(p.ioCountersSeq) - 1
	}
	p.ioCall++
	return p.ioCountersSeq[index], nil
}

type fakeInterfaceResolver struct {
	iface string
	err   error
}

func (r fakeInterfaceResolver) DefaultInterface() (string, error) {
	return r.iface, r.err
}
