package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/server/api"
	"github.com/pqhuy98/wow-converter/internal/server/rest"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func main() {
	root := workspace.AppRoot()
	if err := os.Chdir(root); err != nil {
		log.Printf("warning: chdir app root: %v", err)
	}
	_ = workspace.LoadEnvFile(filepath.Join(root, ".env"))

	bundled := flag.Bool("bundled", workspace.IsDesktopApp() || envTruthy("WOW_CONVERTER_BUNDLED"), "run wow-data-server in-process")
	flag.Parse()

	ctx := context.Background()
	var dataClient client.Client
	var dataServer *rest.Server
	var monitorCancel context.CancelFunc

	if *bundled {
		transport.ConfigureBundled()
		handler, err := bootstrap.StartWowDataServer(ctx)
		if err != nil {
			log.Fatalf("failed to start in-process wow-data-server: %v", err)
		}
		dataClient = client.NewInProcessClient(handler)
		listenOpts := rest.ServerListenOptions{Port: server.GetServerPort()}
		if transport.UsesSocketTransport() {
			listenOpts.SocketPath = transport.DefaultSocketPath()
		}
		dataServer = rest.NewServerWithOptions(handler, listenOpts)
		go func() {
			if err := dataServer.ListenAndServe(); err != nil {
				log.Printf("wow-data-server: %v", err)
			}
		}()
	} else {
		dataClient = client.NewHTTPClient("")
		log.Println("Dev mode: wow-data client via HTTP (default http://127.0.0.1:17753)")
	}

	cfg := api.LoadConfig()
	if cfg.IsSharedHosting {
		log.Println("Shared hosting mode enabled")
	}

	if *bundled {
		src := server.GlobalRuntime.GetCascOptional()
		if src == nil || !src.IsLoaded() {
			setupURL := fmt.Sprintf("http://127.0.0.1:%d/setup", cfg.Port)
			log.Printf("%s %s to choose your WoW installation.", ansi.Gray("WoW data not loaded yet. Open"), ansi.Blue(setupURL))
		}
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	monitorCancel = cancel
	client.StartCascMonitor(monitorCtx, dataClient)

	deps := api.NewDeps(dataClient, cfg)
	srv := api.NewServer(deps)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("wow-converter failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if monitorCancel != nil {
		monitorCancel()
	}
	if dataServer != nil {
		_ = dataServer.Shutdown(shutdownCtx)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("wow-converter stopped")
}

func envTruthy(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}
