package formats

import (
	"fmt"
	"net/http"
	"testing"
)

func TestIsRetryableDownloadError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "request timeout", err: downloadStatusError{status: http.StatusRequestTimeout}, want: true},
		{name: "rate limited", err: downloadStatusError{status: http.StatusTooManyRequests}, want: true},
		{name: "server error", err: downloadStatusError{status: http.StatusBadGateway}, want: true},
		{name: "gateway timeout", err: downloadStatusError{status: http.StatusGatewayTimeout}, want: true},
		{name: "not found", err: downloadStatusError{status: http.StatusNotFound}, want: false},
		{name: "client error", err: downloadStatusError{status: http.StatusBadRequest}, want: false},
		{name: "network timeout", err: fmt.Errorf("request timeout"), want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableDownloadError(tc.err); got != tc.want {
				t.Fatalf("isRetryableDownloadError(%v) = %t, want %t", tc.err, got, tc.want)
			}
		})
	}
}
