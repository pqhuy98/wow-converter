package mapexporter

import "testing"

func TestNormalizeMapSaveName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"my-map", "my-map.w3x"},
		{"my-map.w3x", "my-map.w3x"},
		{"MY-MAP.W3X", "MY-MAP.w3x"},
		{"my-map.w3x.w3x", "my-map.w3x"},
		{"  spaced  ", "spaced.w3x"},
		{"  spaced.w3x  ", "spaced.w3x"},
		{"bad/name", "bad_name.w3x"},
		{"", ""},
		{"...", ""},
	}
	for _, tc := range tests {
		got := NormalizeMapSaveName(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeMapSaveName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
