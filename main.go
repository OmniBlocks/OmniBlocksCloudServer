package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/OmniBlocks/OmniBlocksCloudServer/server"
)

const configPath = "config.toml"

func main() {
	cfg, err := server.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config %q: %v", configPath, err)
	}

	logger, err := server.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Close()

	logger.Config("loaded config from %q (port=%d)", configPath, cfg.Port)

	addr := ":" + strconv.Itoa(cfg.Port)
	fmt.Println("WebSocket Server started on " + addr)
	logger.Server("listening on %s", addr)

	s := server.New(logger)
	http.Handle("/ws", s)

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Server("server stopped: %v", err)
		logger.Close()
		os.Exit(1)
	}
}
