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
		ProjectID: "1141249869",
		Username:  "boxy",
		CloudHost: "ws://localhost:9080",
		UserAgent: "BoxyScraper/1.0 (aipoweredtools@boxy.com)",
	})
	if err != nil {
		panic(err)
	}

	client.On("connected", func() {
		fmt.Println("Connected to cloud server!")
	})

	client.On("set", func(name string, value any) {
		fmt.Printf("Variable %s set to %v\n", name, value)
	})

	client.Connect()

	// Update variable
	client.Set("CLOUD 1", 01100010)

	time.Sleep(50 * time.Millisecond)
	client.Close()
}
```

## License

MPL-2.0. See [LICENSE](./LICENSE).