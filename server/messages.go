package server

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// inMessage is the union of all fields that can appear in a client/server
// message. Only the fields relevant to Method are populated/used.
type inMessage struct {
	Method    string          `json:"method"`
	ProjectID json.RawMessage `json:"project_id"`
	User      string          `json:"user"`
	Name      string          `json:"name"`
	Value     json.RawMessage `json:"value"`
	NewName   string          `json:"new_name"`
}

// projectIDString normalizes a project_id field, which clients may encode
// as either a JSON string or a JSON number, into a plain string.
func projectIDString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return ""
}

type setMessage struct {
	Method string          `json:"method"`
	Name   string          `json:"name"`
	Value  json.RawMessage `json:"value"`
}

func encodeSetMessage(name string, value json.RawMessage) []byte {
	b, _ := json.Marshal(setMessage{Method: "set", Name: name, Value: value})
	return b
}

func joinMessages(messages [][]byte) []byte {
	return bytes.Join(messages, []byte("\n"))
}
