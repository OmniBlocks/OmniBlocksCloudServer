package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/OmniBlocks/OmniBlocksCloudServer/server"
)

const addr = ":9080"

func main() {
	fmt.Println("WebSocket Server started on " + addr)

	s := server.New()
	http.Handle("/ws", s)

	log.Fatal(http.ListenAndServe(addr, nil))
}
