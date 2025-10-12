package main

import (
	"log"
	"os"
	"strconv"

	"wowexportd/internal/server"
)

func main() {
	port := 17753
	if v := os.Getenv("WOWEXPORT_REST_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}

	s := server.New(port)
	if err := s.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
