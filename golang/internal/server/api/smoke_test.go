package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/server/api"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestGetConfigEndpoint(t *testing.T) {
	c := client.NewHTTPClient("")
	if _, err := c.GetConfig(context.Background(), "exportDirectory"); err != nil {
		t.Skip("wow-data-server unavailable:", err)
	}

	cfg := api.LoadConfig()
	deps := api.NewDeps(c, cfg)
	srv := httptest.NewServer(api.NewServer(deps).Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

func TestMain(m *testing.M) {
	if os.Getenv("WOW_DATA_SERVER_URL") == "" {
		os.Setenv("WOW_DATA_SERVER_URL", "http://127.0.0.1:17753")
	}
	os.Exit(m.Run())
}
