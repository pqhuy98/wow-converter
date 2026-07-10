package wowconfig

import "testing"

func TestNormalizeInstallDirectory(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"D:/Games/WoW", `D:\Games\WoW`},
		{`D:\Games\\WoW`, `D:\Games\WoW`},
		{`\\server\share\wow`, `\\server\share\wow`},
		{`//server/share/wow`, `\\server\share\wow`},
		{`/home/user/wow`, `/home/user/wow`},
	}
	for _, tc := range tests {
		if got := NormalizeInstallDirectory(tc.in); got != tc.want {
			t.Errorf("NormalizeInstallDirectory(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
