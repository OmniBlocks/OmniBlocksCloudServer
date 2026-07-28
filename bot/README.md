# OmniBlocks Bot Library

A lightweight, event-driven Go client library for interacting with OmniBlocks cloud variable servers.

## Features

- Event-driven API
- Automatic Reconnection
- Strict Validation and Normalization
- Queued Writes

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
		CloudHost: "ws://127.0.0.1:9080",
		UserAgent: "BoxyScraper/1.0 (contact@boxy.com)",
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

MPL-2.0. See [LICENSE](./LICENSE).