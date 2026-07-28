package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/OmniBlocks/OmniBlocksCloudServer/server"
)

const configPath = "config.toml"

func main() {
	cfg, err := server.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config %q: %v", configPath, err)
	}

	addr := ":" + strconv.Itoa(cfg.Port)
	fmt.Println("WebSocket Server started on " + addr)

	s := server.New()
	http.Handle("/ws", s)

	log.Fatal(http.ListenAndServe(addr, nil))
}
