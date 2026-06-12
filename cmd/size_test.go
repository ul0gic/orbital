package cmd

import "testing"

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1", 1},
		{"1024", 1024},
		{"1B", 1},
		{"1K", 1 << 10},
		{"1KiB", 1 << 10},
		{"1KB", 1000},
		{"500MB", 500 * 1000 * 1000},
		{"2GB", 2 * 1000 * 1000 * 1000},
		{"512MiB", 512 << 20},
		{"2GiB", 2 << 30},
		{"1TiB", 1 << 40},
		{"2G", 2 << 30},
		{" 1MB ", 1000 * 1000},
		{"1mb", 1000 * 1000},
		{"1.5KiB", 1536},
	}
	for _, tc := range cases {
		got, err := parseSize(tc.in)
		if err != nil {
			t.Errorf("parseSize(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseSize(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseSizeRejectsInvalid(t *testing.T) {
	for _, in := range []string{"", "  ", "abc", "-5MB", "0", "0MB", "MB", "1.2.3", "-1", "1XB"} {
		if got, err := parseSize(in); err == nil {
			t.Errorf("parseSize(%q) = %d, want error", in, got)
		}
	}
}
