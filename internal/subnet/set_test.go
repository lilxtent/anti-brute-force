package subnet

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet_AddAndContains(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("192.168.0.0/24"))

	require.True(t, s.Contains(netip.MustParseAddr("192.168.0.42")))
	require.False(t, s.Contains(netip.MustParseAddr("192.168.1.1")))
}

func TestSet_ContainsEmpty(t *testing.T) {
	var s Set
	require.False(t, s.Contains(netip.MustParseAddr("10.0.0.1")))
}

func TestSet_AddDeduplicates(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("10.0.0.0/8"))
	s.Add(netip.MustParsePrefix("10.0.0.0/8"))

	require.Len(t, s.prefixes, 1)
}

func TestSet_AddMasksHostBits(t *testing.T) {
	var s Set
	// Host bits set; Masked() should normalise to 10.0.0.0/8.
	s.Add(netip.MustParsePrefix("10.11.12.13/8"))

	require.Len(t, s.prefixes, 1)
	require.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), s.prefixes[0])
	// Adding the already-masked form must be treated as a duplicate.
	s.Add(netip.MustParsePrefix("10.0.0.0/8"))
	require.Len(t, s.prefixes, 1)
}

func TestSet_Remove(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("192.168.0.0/24"))
	s.Add(netip.MustParsePrefix("172.16.0.0/16"))

	// Unmasked form should still match for removal.
	s.Remove(netip.MustParsePrefix("192.168.0.5/24"))

	require.False(t, s.Contains(netip.MustParseAddr("192.168.0.42")))
	require.True(t, s.Contains(netip.MustParseAddr("172.16.5.5")))
	require.Len(t, s.prefixes, 1)
}

func TestSet_RemoveMissingIsNoop(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("192.168.0.0/24"))
	s.Remove(netip.MustParsePrefix("10.0.0.0/8"))

	require.Len(t, s.prefixes, 1)
	require.True(t, s.Contains(netip.MustParseAddr("192.168.0.1")))
}

func TestSet_ContainsIPv4MappedIPv6(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("192.168.0.0/24"))

	// An IPv4-mapped IPv6 address should be unmapped before matching.
	mapped := netip.AddrFrom16(netip.MustParseAddr("192.168.0.7").As16())
	require.True(t, mapped.Is4In6())
	require.True(t, s.Contains(mapped))
}

func TestSet_ContainsIPv6(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("2001:db8::/32"))

	require.True(t, s.Contains(netip.MustParseAddr("2001:db8::1")))
	require.False(t, s.Contains(netip.MustParseAddr("2001:dead::1")))
}

func TestSet_Load(t *testing.T) {
	var s Set
	s.Load([]netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("10.11.12.13/8"), // duplicate after masking
		netip.MustParsePrefix("192.168.0.0/24"),
	})

	require.Len(t, s.prefixes, 2)
	require.True(t, s.Contains(netip.MustParseAddr("10.1.2.3")))
	require.True(t, s.Contains(netip.MustParseAddr("192.168.0.1")))
}

func TestSet_LoadReplacesExisting(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("172.16.0.0/16"))

	s.Load([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})

	require.False(t, s.Contains(netip.MustParseAddr("172.16.0.1")))
	require.True(t, s.Contains(netip.MustParseAddr("10.0.0.1")))
	require.Len(t, s.prefixes, 1)
}

func TestSet_LoadEmpty(t *testing.T) {
	var s Set
	s.Add(netip.MustParsePrefix("10.0.0.0/8"))
	s.Load(nil)

	require.Empty(t, s.prefixes)
	require.False(t, s.Contains(netip.MustParseAddr("10.0.0.1")))
}

func TestSet_ConcurrentAccess(t *testing.T) {
	var s Set
	var wg sync.WaitGroup

	for range 1000 {
		wg.Add(3)
		go func() { defer wg.Done(); s.Add(netip.MustParsePrefix("10.0.0.0/8")) }()
		go func() { defer wg.Done(); s.Contains(netip.MustParseAddr("10.0.0.1")) }()
		go func() { defer wg.Done(); s.Remove(netip.MustParsePrefix("10.0.0.0/8")) }()
	}

	wg.Wait()
}
