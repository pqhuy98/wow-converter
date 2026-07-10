package httplog

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeRoutePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/maps/0/generate-wc3", "/api/maps/{map}/generate-wc3"},
		{"/api/maps/generate-wc3/status/job-1", "/api/maps/generate-wc3/status/{jobId}"},
		{"/api/export/character/status/job-1", "/api/export/character/status/{jobId}"},
		{"/api/assets/wow/foo.blp", "/api/assets/*"},
		{"/api/texture/png/interface/icons/foo.png", "/api/texture/png/*"},
		{"/api/export/character", "/api/export/character"},
	}
	for _, tc := range tests {
		if got := normalizeRoutePath(tc.path); got != tc.want {
			t.Errorf("normalizeRoutePath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestIsLoggedRequest(t *testing.T) {
	tests := []struct {
		method string
		path   string
		logged bool
	}{
		{"POST", "/api/export/character", true},
		{"POST", "/api/wow-config/apply", true},
		{"POST", "/api/maps/0/generate-wc3", true},
		{"POST", "/rest/loadCascLocal", true},
		{"POST", "/api/texture/blp", true},

		{"POST", "/api/wow-config/discover-local", false},
		{"POST", "/api/wow-config/discover-remote", false},
		{"POST", "/api/wow-config/pick-local-folder", false},
		{"POST", "/api/maps/0/creatures-check", false},
		{"POST", "/api/download", false},
		{"POST", "/rest/charMeta", false},

		{"GET", "/api/maps", false},
		{"GET", "/api/export/character/status/job-1", false},
		{"GET", "/rest/cascFile", false},

		{"GET", "/setup", false},
		{"GET", "/_next/static/chunks/app.js", false},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if got := isLoggedRequest(req); got != tc.logged {
			t.Errorf("isLoggedRequest(%s %q) = %v, want %v", tc.method, tc.path, got, tc.logged)
		}
	}
}

func TestLoggedRoutesTrueEntriesAreWrites(t *testing.T) {
	for key, logged := range loggedRoutes {
		if !logged {
			continue
		}
		if strings.Contains(key, "discover") || strings.Contains(key, "charMeta") ||
			strings.Contains(key, "creatures-check") || strings.HasSuffix(key, "/download") {
			t.Errorf("logged route %q is true but looks like a read/query endpoint", key)
		}
	}
}
