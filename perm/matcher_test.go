package perm

import (
	"fmt"
	"path/filepath"
	"testing"
)

var matcherPatterns = []string{
	"presets:list", "Editor", "*", "", ":presets:api_keys:*", "*:reported_issues:*",
	"*:presets:activity_logs", "presets:*", "a?c", "[ab]c", "[", `a\*b`, "a/b", "*/b",
}

var matcherNeedles = []string{
	"presets:list", "presets:lis", "Editor", "editor", "", "a/b", "/", "x",
	":presets:api_keys:12:", ":presets:reported_issues:7:", ":presets:activity_logs",
	"abc", "bc", "a*b", "a/b/c", "presets:list/x",
}

// The fast paths answer exactly what filepath.Match answers, errors included.
func TestMatchPatternAgreesWithFilepathMatch(t *testing.T) {
	for _, p := range matcherPatterns {
		for _, n := range matcherNeedles {
			want, wantErr := filepath.Match(p, n)
			for call := 1; call <= 2; call++ { // the second answer comes from the cache
				got, err := matchPattern(p, n)
				if got != want || (err == nil) != (wantErr == nil) {
					t.Errorf("matchPattern(%q, %q) call %d = %v, %v; filepath.Match = %v, %v", p, n, call, got, err, want, wantErr)
				}
			}
		}
	}
}

func TestPathMatcherMatchesAnyPattern(t *testing.T) {
	m := &PathMatcher{}
	ok, err := m.Matches(nil, []string{"Editor", "Admin"}, "Admin")
	if err != nil || !ok {
		t.Fatalf("Admin among the subjects: %v, %v", ok, err)
	}
	ok, err = m.Matches(nil, []string{"Editor", "Admin"}, "Manager")
	if err != nil || ok {
		t.Fatalf("Manager is not among them: %v, %v", ok, err)
	}
	if _, err := m.Matches(nil, []string{"["}, "x"); err == nil {
		t.Fatal("a malformed pattern must still report filepath.ErrBadPattern")
	}
}

// The cache never holds more than maxCachedMatches answers, however many
// needles (record ids) it is asked about.
func TestMatchCacheIsBounded(t *testing.T) {
	c := &matchCache{answers: map[matchKey]bool{}}
	for i := 0; i < maxCachedMatches+10; i++ {
		if _, err := c.match(":presets:orders:*", fmt.Sprintf(":presets:orders:%d:", i)); err != nil {
			t.Fatal(err)
		}
		if len(c.answers) > maxCachedMatches {
			t.Fatalf("cache grew to %d", len(c.answers))
		}
	}
	if ok, err := c.match(":presets:orders:*", ":presets:orders:7:"); err != nil || !ok {
		t.Fatalf("after a reset the answers are still right: %v, %v", ok, err)
	}
}

func BenchmarkPathMatcher(b *testing.B) {
	m := &PathMatcher{}
	subjects := []string{"Editor", "Attendant", "CSE", "FinancialManager", "Admin"}
	for i := 0; i < b.N; i++ {
		_, _ = m.Matches(nil, []string{"presets:list"}, "presets:update")
		_, _ = m.Matches(nil, subjects, "Admin")
		_, _ = m.Matches(nil, []string{"*"}, ":presets:orders:42:")
		_, _ = m.Matches(nil, []string{":presets:api_keys:*"}, ":presets:orders:42:")
	}
}
