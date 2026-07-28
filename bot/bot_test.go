package bot

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OmniBlocks/OmniBlocksCloudServer/server"
)

func TestBotNormalization(t *testing.T) {
	if got := normalizeName("variable"); got != "☁ variable" {
		t.Errorf("expected '☁ variable', got %q", got)
	}
	if got := normalizeName("☁ variable"); got != "☁ variable" {
		t.Errorf("expected '☁ variable', got %q", got)
	}
	if got := normalizeName(":cloud: test"); got != ":cloud: test" {
		t.Errorf("expected ':cloud: test', got %q", got)
	}
}

func TestBotIntegration(t *testing.T) {
	srv := server.New(nil)
	testSrv := httptest.NewServer(srv)
	defer testSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(testSrv.URL, "http")

	botClient, err := New(Config{
		ProjectID: "12345",
		Username:  "testbot",
		CloudHost: wsURL,
		UserAgent: "TestBot/1.0",
	})
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	connectedChan := make(chan struct{})

	botClient.On("connected", func() {
		close(connectedChan)
	})

	botClient.Connect()
	defer botClient.Close()

	select {
	case <-connectedChan:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for bot connection")
	}

	// Test setting variable
	botClient.Set("score", 42)

	// Verify via get
	time.Sleep(100 * time.Millisecond)
	if val := botClient.Get("score"); val != float64(42) && val != 42 {
		t.Errorf("expected score 42, got %v (%T)", val, val)
	}
}
