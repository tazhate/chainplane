package registry

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		candidate, current, prefix string
		want                       bool
	}{
		{"v29.0", "v28.0", "", true},
		{"v28.0", "v29.0", "", false},
		{"v28.0", "v28.0", "", false},
		{"GreatVoyage-v4.9.0", "GreatVoyage-v4.8.1", "GreatVoyage-", true},
		{"GreatVoyage-v4.8.1", "GreatVoyage-v4.9.0", "GreatVoyage-", false},
		{"mainnet-v1.70.0", "mainnet-v1.69.2", "mainnet-", true},
		{"v2025.04", "v2025.03", "", true},
		{"not-a-version", "v1.0.0", "", false},
		{"v1.0.0", "not-a-version", "", false},
	}
	for _, tc := range cases {
		got := IsNewer(tc.candidate, tc.current, tc.prefix)
		if got != tc.want {
			t.Errorf("IsNewer(%q, %q, %q) = %v, want %v", tc.candidate, tc.current, tc.prefix, got, tc.want)
		}
	}
}

func TestNormalizeTag(t *testing.T) {
	cases := []struct {
		tag, prefix, want string
	}{
		{"v28.0", "", "v28.0.0"},
		{"28.0", "", "v28.0.0"},
		{"GreatVoyage-v4.8.1", "GreatVoyage-", "v4.8.1"},
		{"mainnet-v1.69.2", "mainnet-", "v1.69.2"},
		{"v2025.04", "", "v2025.4.0"},
	}
	for _, tc := range cases {
		got := normalizeTag(tc.tag, tc.prefix)
		if got != tc.want {
			t.Errorf("normalizeTag(%q, %q) = %q, want %q", tc.tag, tc.prefix, got, tc.want)
		}
	}
}
