package state

import "sort"

func (s *RuntimeState) KnownInboundTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make([]string, 0, len(s.KnownInboundTag))
	for tag := range s.KnownInboundTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func (s *RuntimeState) AddKnownInboundTag(tag string) {
	if tag == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.KnownInboundTag[tag] = struct{}{}
}

func (s *RuntimeState) AddKnownInboundTags(tags ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tag := range tags {
		if tag != "" {
			s.KnownInboundTag[tag] = struct{}{}
		}
	}
}

func (s *RuntimeState) AddUserToInbound(tag string, userHash string) {
	if tag == "" || userHash == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InboundUsers[tag] == nil {
		s.InboundUsers[tag] = map[string]struct{}{}
	}
	s.InboundUsers[tag][userHash] = struct{}{}
	s.KnownInboundTag[tag] = struct{}{}
}

func (s *RuntimeState) RemoveUserFromInbound(tag string, userHash string) {
	if tag == "" || userHash == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InboundUsers[tag] == nil {
		return
	}
	delete(s.InboundUsers[tag], userHash)
}

func (s *RuntimeState) InboundUserHashes(tag string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := s.InboundUsers[tag]
	hashes := make([]string, 0, len(users))
	for userHash := range users {
		hashes = append(hashes, userHash)
	}
	sort.Strings(hashes)
	return hashes
}
