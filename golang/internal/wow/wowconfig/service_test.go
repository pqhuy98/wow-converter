package wowconfig

import (
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestDiscoverFailureErrorUsesTypedRESTError(t *testing.T) {
	tests := []struct {
		name string
		err  *client.RESTError
		want string
	}{
		{
			name: "active CASC",
			err:  &client.RESTError{ID: "ERR_CASC_ACTIVE", Message: "already active"},
			want: "already loaded",
		},
		{
			name: "invalid install details",
			err:  &client.RESTError{ID: "ERR_INVALID_INSTALL", Message: "missing .build.info"},
			want: "missing .build.info",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := discoverFailureError(true, tt.err).Error(); !strings.Contains(got, tt.want) {
				t.Fatalf("discoverFailureError() = %q, want %q", got, tt.want)
			}
		})
	}
}
