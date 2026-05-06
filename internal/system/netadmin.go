package system

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

const capNetAdmin = 12

func HasNetAdmin() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(status), "\n") {
		if !strings.HasPrefix(line, "CapEff:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return false
		}
		value, err := strconv.ParseUint(fields[1], 16, 64)
		if err != nil {
			return false
		}
		return value&(1<<capNetAdmin) != 0
	}
	return false
}
