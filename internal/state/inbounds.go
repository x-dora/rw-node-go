package state

func (s *RuntimeState) KnownInboundTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make([]string, 0, len(s.KnownInboundTag))
	for tag := range s.KnownInboundTag {
		tags = append(tags, tag)
	}
	return tags
}
