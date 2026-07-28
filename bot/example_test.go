package bot_test

import (
	"fmt"
	"time"

	"github.com/OmniBlocks/OmniBlocksCloudServer/bot"
)

func ExampleBot() {
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
		fmt.Println("Connected to OmniBlocks cloud server!")
	})

	client.On("set", func(name string, value any) {
		fmt.Printf("Variable %s set to %v\n", name, value)
	})

	client.Connect()

	// Update variable
	client.Set("score", 100)

	time.Sleep(50 * time.Millisecond)
	client.Close()
}
