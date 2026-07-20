package subnet

import (
	"net/netip"
	"slices"
	"sync"
)

type Set struct {
	mutex    sync.RWMutex
	prefixes []netip.Prefix
}

func (s *Set) Add(prefix netip.Prefix) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.addLocked(prefix)
}

func (s *Set) addLocked(prefix netip.Prefix) {
	prefix = prefix.Masked()
	if slices.Contains(s.prefixes, prefix) {
		return
	}
	s.prefixes = append(s.prefixes, prefix)
}

func (s *Set) Remove(prefix netip.Prefix) {
	prefix = prefix.Masked()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if i := slices.Index(s.prefixes, prefix); i >= 0 {
		s.prefixes = slices.Delete(s.prefixes, i, i+1)
	}
}

func (s *Set) Contains(addr netip.Addr) bool {
	addr = addr.Unmap()
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, prefix := range s.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (s *Set) Load(prefixes []netip.Prefix) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.prefixes = make([]netip.Prefix, 0, len(prefixes))
	for _, p := range prefixes {
		s.addLocked(p)
	}
}
