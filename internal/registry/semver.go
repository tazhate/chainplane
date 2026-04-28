package registry

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// IsNewer reports whether candidate is a newer version than current,
// after stripping the given prefix from both. Returns false if either
// is not a valid semver (with or without "v" prefix).
func IsNewer(candidate, current, prefix string) bool {
	c := normalizeTag(candidate, prefix)
	cur := normalizeTag(current, prefix)
	if !semver.IsValid(c) || !semver.IsValid(cur) {
		return false
	}
	return semver.Compare(c, cur) > 0
}

// normalizeTag strips prefix, ensures a leading "v", and pads to vMAJOR.MINOR.PATCH
// to satisfy golang.org/x/mod/semver strict validation (no leading zeros, 3 parts).
func normalizeTag(tag, prefix string) string {
	s := strings.TrimPrefix(tag, prefix)
	if !strings.HasPrefix(s, "v") {
		s = "v" + s
	}
	core := s
	var suffix string
	if idx := strings.IndexAny(s[1:], "-+"); idx >= 0 {
		core = s[:idx+1]
		suffix = s[idx+1:]
	}
	vParts := strings.Split(strings.TrimPrefix(core, "v"), ".")
	normalized := make([]string, len(vParts))
	for i, p := range vParts {
		if n, err := strconv.ParseUint(p, 10, 64); err == nil {
			normalized[i] = fmt.Sprintf("%d", n)
		} else {
			normalized[i] = p
		}
	}
	for len(normalized) < 3 {
		normalized = append(normalized, "0")
	}
	result := "v" + strings.Join(normalized, ".")
	if suffix != "" {
		result += suffix
	}
	return result
}
