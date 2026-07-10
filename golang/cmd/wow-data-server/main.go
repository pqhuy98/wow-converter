package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pqhuy98/wow-converter/internal/server/rest"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	wowlog "github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

func main() {
	prefix := os.Getenv("WOW_LOG_PREFIX")
	if prefix == "" {
		prefix = "go"
	}
	log.SetPrefix(fmt.Sprintf("[%s] ", prefix))

	if root, err := workspace.ChdirRepoRoot(); err != nil {
		log.Printf("warning: chdir repo root: %v", err)
	} else {
		log.Printf("Working directory: %s", root)
	}
	_ = workspace.LoadEnvFile(".env")

	ctx := context.Background()
	handler, err := bootstrap.StartWowDataServer(ctx)
	if err != nil {
		log.Fatalf("failed to start wow-data-server: %v", err)
	}

	port := server.GetServerPort()
	srv := rest.NewServer(handler, port)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("wow-data-server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	wowlog.Write("wow-data-server stopped")
}
