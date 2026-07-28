package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// small helper
type testClient struct {
	t        *testing.T
	conn     *websocket.Conn
	messages chan []byte
	closed   chan error
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := New(NewDiscardLogger())
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts
}

func dial(t *testing.T, ts *httptest.Server) *testClient {
	t.Helper()
	url := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	c := &testClient{
		t:        t,
		conn:     conn,
		messages: make(chan []byte, 16),
		closed:   make(chan error, 1),
	}
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				c.closed <- err
				close(c.messages)
				return
			}
			c.messages <- data
		}
	}()

	return c
}

func (c *testClient) sendJSON(v any) {
	c.t.Helper()
	if err := c.conn.WriteJSON(v); err != nil {
		c.t.Fatalf("write failed: %v", err)
	}
}

func (c *testClient) handshake(projectID, user string) {
	c.t.Helper()
	c.sendJSON(map[string]any{
		"method":     "handshake",
		"project_id": projectID,
		"user":       user,
	})
}

// reads one WebSocket frame
func (c *testClient) expectSetMessages(want ...setMessage) {
	c.t.Helper()
	var data []byte
	select {
	case data = <-c.messages:
	case err := <-c.closed:
		c.t.Fatalf("expected message, got close: %v", err)
	case <-time.After(2 * time.Second):
		c.t.Fatalf("expected message, got nothing")
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) != len(want) {
		c.t.Fatalf("expected %d messages, got %d: %q", len(want), len(lines), data)
	}
	for i, line := range lines {
		var got setMessage
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			c.t.Fatalf("bad json in message %d: %v", i, err)
		}
		if got.Method != want[i].Method || got.Name != want[i].Name || string(got.Value) != string(want[i].Value) {
			c.t.Fatalf("message %d: got %+v with value %s, want %+v with value %s", i, got, got.Value, want[i], want[i].Value)
		}
	}
}

func (c *testClient) expectNoMessage() {
	c.t.Helper()
	select {
	case data := <-c.messages:
		c.t.Fatalf("expected no message, but got: %q", data)
	case err := <-c.closed:
		c.t.Fatalf("expected no message, but connection closed: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
}

func (c *testClient) expectClose(code int) {
	c.t.Helper()
	select {
	case data := <-c.messages:
		c.t.Fatalf("expected close, but got message: %q", data)
	case err := <-c.closed:
		closeErr, ok := err.(*websocket.CloseError)
		if !ok {
			c.t.Fatalf("expected close error, got: %v", err)
		}
		if closeErr.Code != code {
			c.t.Fatalf("expected close code %d, got %d", code, closeErr.Code)
		}
	case <-time.After(2 * time.Second):
		c.t.Fatalf("expected close, got nothing")
	}
}

func rawNum(n int) json.RawMessage {
	b, _ := json.Marshal(n)
	return b
}

func rawStr(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func TestHandshakeThenSetBroadcastsToOtherClients(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("123", "alice")

	b := dial(t, ts)
	b.handshake("123", "bob")

	a.sendJSON(map[string]any{
		"method": "set",
		"name":   "☁ score",
		"value":  "42",
	})

	b.expectSetMessages(setMessage{Method: "set", Name: "☁ score", Value: rawStr("42")})
	a.expectNoMessage() // sender does not receive its own update
}

func TestCreateIsTreatedLikeSet(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("123", "alice")
	b := dial(t, ts)
	b.handshake("123", "bob")

	a.sendJSON(map[string]any{
		"method": "create",
		"name":   "☁ score",
		"value":  "7",
	})

	b.expectSetMessages(setMessage{Method: "set", Name: "☁ score", Value: rawStr("7")})
}

func TestValueCanBeNumberOrString(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("123", "alice")
	b := dial(t, ts)
	b.handshake("123", "bob")

	a.sendJSON(map[string]any{
		"method": "set",
		"name":   "☁ score",
		"value":  99,
	})

	b.expectSetMessages(setMessage{Method: "set", Name: "☁ score", Value: rawNum(99)})
}

func TestNewClientReceivesExistingVariablesOnJoin(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("123", "alice")
	a.sendJSON(map[string]any{"method": "set", "name": "☁ one", "value": "1"})
	a.sendJSON(map[string]any{"method": "set", "name": "☁ two", "value": "2"})

	// Give the server a moment to process both sets before b joins
	time.Sleep(50 * time.Millisecond)

	b := dial(t, ts)
	b.handshake("123", "bob")
	b.expectSetMessages(
		setMessage{Method: "set", Name: "☁ one", Value: rawStr("1")},
		setMessage{Method: "set", Name: "☁ two", Value: rawStr("2")},
	)
}

func TestInvalidValueIsSilentlyIgnored(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("123", "alice")
	b := dial(t, ts)
	b.handshake("123", "bob")

	a.sendJSON(map[string]any{"method": "set", "name": "☁ score", "value": "not a number"})
	b.expectNoMessage()

	// Connection must still be alive and usable afterwards.
	a.sendJSON(map[string]any{"method": "set", "name": "☁ score", "value": "5"})
	b.expectSetMessages(setMessage{Method: "set", Name: "☁ score", Value: rawStr("5")})
}

func TestHandshakeInvalidUsernameClosesWithUsernameError(t *testing.T) {
	ts := newTestServer(t)
	c := dial(t, ts)
	c.handshake("123", "") // empty username is invalid
	c.expectClose(CodeUsernameError)
}

func TestHandshakeInvalidProjectIDClosesWithGenericError(t *testing.T) {
	ts := newTestServer(t)
	c := dial(t, ts)
	c.handshake("", "alice")
	c.expectClose(CodeGenericError)
}

func TestMessageBeforeHandshakeCloses(t *testing.T) {
	ts := newTestServer(t)
	c := dial(t, ts)
	c.sendJSON(map[string]any{"method": "set", "name": "☁ x", "value": "1"})
	c.expectClose(CodeGenericError)
}

func TestUnknownMethodCloses(t *testing.T) {
	ts := newTestServer(t)
	c := dial(t, ts)
	c.handshake("123", "alice")
	c.sendJSON(map[string]any{"method": "flarp"})
	c.expectClose(CodeGenericError)
}

func TestRenameMovesValueToNewName(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("123", "alice")
	a.sendJSON(map[string]any{"method": "set", "name": "☁ old", "value": "1"})
	time.Sleep(50 * time.Millisecond)
	a.sendJSON(map[string]any{"method": "rename", "name": "☁ old", "new_name": "☁ new"})
	time.Sleep(50 * time.Millisecond)

	b := dial(t, ts)
	b.handshake("123", "bob")
	b.expectSetMessages(setMessage{Method: "set", Name: "☁ new", Value: rawStr("1")})
}

func TestRenameToInvalidNameCloses(t *testing.T) {
	ts := newTestServer(t)
	a := dial(t, ts)
	a.handshake("123", "alice")
	a.sendJSON(map[string]any{"method": "set", "name": "☁ old", "value": "1"})
	time.Sleep(50 * time.Millisecond)
	a.sendJSON(map[string]any{"method": "rename", "name": "☁ old", "new_name": "no prefix"})
	a.expectClose(CodeGenericError)
}

func TestRenameNonexistentVariableCloses(t *testing.T) {
	ts := newTestServer(t)
	a := dial(t, ts)
	a.handshake("123", "alice")
	a.sendJSON(map[string]any{"method": "rename", "name": "☁ nope", "new_name": "☁ new"})
	a.expectClose(CodeGenericError)
}

func TestDeleteRemovesVariable(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("123", "alice")
	a.sendJSON(map[string]any{"method": "set", "name": "☁ gone", "value": "1"})
	time.Sleep(50 * time.Millisecond)
	a.sendJSON(map[string]any{"method": "delete", "name": "☁ gone"})
	time.Sleep(50 * time.Millisecond)

	b := dial(t, ts)
	b.handshake("123", "bob")
	b.expectNoMessage()
}

func TestDeleteNonexistentVariableCloses(t *testing.T) {
	ts := newTestServer(t)
	a := dial(t, ts)
	a.handshake("123", "alice")
	a.sendJSON(map[string]any{"method": "delete", "name": "☁ nope"})
	a.expectClose(CodeGenericError)
}

func TestSeparateProjectsDoNotSeeEachOthersUpdates(t *testing.T) {
	ts := newTestServer(t)

	a := dial(t, ts)
	a.handshake("111", "alice")
	b := dial(t, ts)
	b.handshake("222", "bob")

	a.sendJSON(map[string]any{"method": "set", "name": "☁ x", "value": "1"})
	b.expectNoMessage()
}
