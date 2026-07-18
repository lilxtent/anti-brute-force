package subnet

import (
	"net/netip"
	"sync"
)

type Set struct {
	mutex     sync.RWMutex
	prefixies []*netip.Prefix
}

func (s *Set) Add(prefix netip.Prefix) {
	prefix = prefix.Masked()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, existing := range s.prefixies {
		if *existing == prefix {
			return
		}
	}
	s.prefixies = append(s.prefixies, &prefix)
}

func (s *Set) Remove(prefix netip.Prefix) {
	prefix = prefix.Masked()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for i, existing := range s.prefixies {
		if *existing == prefix {
			s.prefixies = append(s.prefixies[:i], s.prefixies[i+1:]...)
			return
		}
	}
}

func (s *Set) Contains(addr netip.Addr) bool {
	addr = addr.Unmap()
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, prefix := range s.prefixies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (s *Set) Load(prefixes []netip.Prefix) {
	deduped := make([]*netip.Prefix, 0, len(prefixes))
	for _, p := range prefixes {
		masked := p.Masked()
		dup := false
		for _, existing := range deduped {
			if *existing == masked {
				dup = true
				break
			}
		}
		if !dup {
			deduped = append(deduped, &masked)
		}
	}
	s.mutex.Lock()
	s.prefixies = deduped
	s.mutex.Unlock()
}
