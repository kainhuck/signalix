package cmd

import "testing"

func TestNormalizeMarketFlag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{" spot ", "spot", false},
		{"PERP", "perp", false},
		{"futures", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeMarketFlag(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("normalizeMarketFlag(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeMarketFlag(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeMarketFlag(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
