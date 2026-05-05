package system

func HasNetAdmin() bool {
	// TODO(M4): detect CAP_NET_ADMIN on Linux.
	return false
}
