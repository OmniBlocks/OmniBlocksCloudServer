package bot_test

import (
	"fmt"
	"time"

	"github.com/OmniBlocks/OmniBlocksCloudServer/bot"
)

func ExampleBot() {
	client, err := bot.New(bot.Config{
		ProjectID: "99999",
		Username:  "exampleBot",
		CloudHost: "wss://clouddata.omniblocks.com",
		UserAgent: "MyBot/1.0 (contact@example.com)",
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
