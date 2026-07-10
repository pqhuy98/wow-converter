package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/server/api"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestBundledGetConfigWithoutCasc(t *testing.T) {
	t.Setenv("CASC_LOCAL_WOW", "")
	t.Setenv("CASC_REMOTE_REGION", "")

	handler, err := bootstrap.StartWowDataServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c := client.NewInProcessClient(handler)
	cfg := api.LoadConfig()
	deps := api.NewDeps(c, cfg)
	srv := httptest.NewServer(api.NewServer(deps).Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/get-config", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
