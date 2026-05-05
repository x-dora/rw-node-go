package system

type Conntrack struct{}

func (c Conntrack) DropIP(ip string) error {
	// TODO(M4): implement netlink/conntrack deletion.
	return nil
}
