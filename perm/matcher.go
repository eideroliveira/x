package perm

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/ory/ladon"
)

type PathMatcher struct{}

// Matches reports whether any pattern of haystack matches needle, with
// filepath.Match's semantics.
//
// ladon asks it three times (action, subject, resource) for every policy on
// every permission check, and an admin page asks once per menu entry, so it
// runs millions of times on a busy admin: 20 million calls for the admin's
// two smoke tests (2026-10-19). Most patterns are a literal (an action such
// as "presets:list", a role such as "Editor") or a bare "*", answered here
// directly: a pattern with no metacharacter matches only itself, and "*"
// matches any needle without a path separator. The rest ("*:orders:*") cost
// filepath.Match microseconds each, since a star tries every split of the
// needle, so their answers are remembered (matchCache).
func (m *PathMatcher) Matches(p ladon.Policy, haystack []string, needle string) (bool, error) {
	for _, h := range haystack {
		m, err := matchPattern(h, needle)
		if err != nil {
			return false, err
		}
		if m {
			return true, nil
		}
	}
	return false, nil
}

func matchPattern(pattern, needle string) (bool, error) {
	if pattern == "*" {
		return !strings.ContainsRune(needle, filepath.Separator), nil
	}
	if !strings.ContainsAny(pattern, `*?[\`) {
		return pattern == needle, nil
	}
	return matches.match(pattern, needle)
}

// maxCachedMatches bounds matchCache. Needles carry record ids
// (":presets:orders:42:"), so the pairs are not bounded by the policies; when
// the cache is full it starts again empty. The admin's tests ask under 17,000
// distinct pairs.
const maxCachedMatches = 1 << 16

// matchCache remembers filepath.Match's answer for a pattern and a needle.
// The answer is a function of the two strings alone, so no policy change
// can make an entry stale. A malformed pattern's error is not remembered.
type matchCache struct {
	mu      sync.RWMutex
	answers map[matchKey]bool
}

type matchKey struct{ pattern, needle string }

var matches = &matchCache{answers: map[matchKey]bool{}}

func (c *matchCache) match(pattern, needle string) (bool, error) {
	key := matchKey{pattern, needle}
	c.mu.RLock()
	ok, found := c.answers[key]
	c.mu.RUnlock()
	if found {
		return ok, nil
	}
	ok, err := filepath.Match(pattern, needle)
	if err != nil {
		return false, err
	}
	c.mu.Lock()
	if len(c.answers) >= maxCachedMatches {
		c.answers = make(map[matchKey]bool, len(c.answers))
	}
	c.answers[key] = ok
	c.mu.Unlock()
	return ok, nil
}
