package system

import (
	"net"
	"os"
	"runtime"

	"github.com/x-dora/rw-node-go/internal/contracts"
)

func SnapshotStats() contracts.SystemStatsPayload {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	hostname, _ := os.Hostname()
	return contracts.SystemStatsPayload{
		Info: contracts.NodeSystemInfo{
			Arch:              runtime.GOARCH,
			CPUs:              runtime.NumCPU(),
			CPUModel:          "",
			MemoryTotal:       mem.Sys,
			Hostname:          hostname,
			Platform:          runtime.GOOS,
			Release:           "",
			Type:              runtime.GOOS,
			Version:           runtime.Version(),
			NetworkInterfaces: interfaceNames(),
		},
		Stats: contracts.NodeSystemStats{
			MemoryFree: 0,
			MemoryUsed: mem.Alloc,
			Uptime:     0,
			LoadAvg:    []float64{0, 0, 0},
			Interface:  nil,
		},
	}
}

func interfaceNames() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []string{}
	}

	names := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		names = append(names, iface.Name)
	}
	return names
}
