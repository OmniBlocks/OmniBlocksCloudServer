# OmniBlocks Bot Library (`bot`)

A lightweight, event-driven Go client library for interacting with OmniBlocks cloud variable servers (akin to TurboWarp's Mist).

## Features

- **Event-driven API**: Easily listen for `connected`, `reconnecting`, `set`, and `error` events.
- **Automatic Reconnection**: Robust reconnection loop with exponential backoff if the server restarts or connection drops.
- **Strict Validation & Normalization**: Automatically prepends cloud prefixes (`☁ ` or `:cloud:`) if omitted and correctly serializes payloads.
- **Queued Writes**: Supports calling `Set()` before the connection is established; writes are queued and flushed upon successful handshake.

## Usage Example

```go
package main

import (
	"fmt"
	"time"

	"github.com/OmniBlocks/OmniBlocksCloudServer/bot"
)

func main() {
	client, err := bot.New(bot.Config{
		ProjectID: "12345",
		Username:  "myBot",
		CloudHost: "wss://clouddata.omniblocks.com",
		UserAgent: "MyAwesomeBot/1.0 (contact@example.com)",
	})
	if err != nil {
		panic(err)
	}

	client.On("connected", func() {
		fmt.Println("Connected!")
	})

	client.On("reconnecting", func() {
		fmt.Println("Connection lost, reconnecting...")
	})

	client.On("set", func(name string, value any) {
		fmt.Printf("Variable %s was set to %v\n", name, value)
	})

	client.On("error", func(err error) {
		fmt.Printf("Error: %v\n", err)
	})

	client.Connect()

	// Set variable (queued if not yet connected)
	client.Set("score", 10)

	// Get cached variable value
	val := client.Get("score")
	fmt.Printf("Current score value: %v\n", val)

	time.Sleep(10 * time.Second)
	client.Close()
}
```

## License

Mozilla Public License 2.0 (MPL-2.0).
