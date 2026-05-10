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

func (s *RuntimeState) SetInboundProtocol(tag string, protocol string) {
	if tag == "" || protocol == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.KnownInboundTag[tag] = struct{}{}
	s.InboundProtocols[tag] = protocol
}

func (s *RuntimeState) InboundProtocol(tag string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.InboundProtocols[tag]
}

func (s *RuntimeState) SetInboundProtocolsFromConfig(config map[string]any) {
	protocols := inboundProtocolsFromConfig(config)
	if len(protocols) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for tag, protocol := range protocols {
		s.KnownInboundTag[tag] = struct{}{}
		s.InboundProtocols[tag] = protocol
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

func inboundProtocolsFromConfig(config map[string]any) map[string]string {
	rawInbounds, ok := config["inbounds"].([]any)
	if !ok {
		return nil
	}
	protocols := map[string]string{}
	for _, rawInbound := range rawInbounds {
		inbound, ok := rawInbound.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := inbound["tag"].(string)
		protocol, _ := inbound["protocol"].(string)
		if tag == "" || protocol == "" {
			continue
		}
		protocols[tag] = protocol
	}
	return protocols
}
