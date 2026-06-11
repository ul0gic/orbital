package manifest

import (
	"path/filepath"
	"strings"
)

var denylistExact = map[string]struct{}{
	".ssh":        {},
	".git":        {},
	"credentials": {},
}

var denylistGlobs = []string{
	".env*",
	"*.pem",
	"*.key",
	"id_rsa*",
	"*history",
}

// matchesDenylist reports whether any path segment is a denied name. Matching is
// per-segment so a denied directory (e.g. .ssh) excludes everything beneath it.
func matchesDenylist(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		if _, ok := denylistExact[seg]; ok {
			return true
		}
		for _, glob := range denylistGlobs {
			if ok, _ := filepath.Match(glob, seg); ok {
				return true
			}
		}
	}
	return false
}
