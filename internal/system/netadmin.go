package system

import "runtime"

func HasNetAdmin() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	// TODO(M4): detect CAP_NET_ADMIN on Linux.
	return false
}
