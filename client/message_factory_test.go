package client

import (
	"encoding/json"
	"testing"
)

func TestDecodeEchoPreservesHistoryData(t *testing.T) {
	payload := []byte(`{"status":"ok","data":{"messages":[{"message_id":42,"message":[{"type":"text","data":{"text":"hello"}}]}]},"echo":"groupSummary:1:0"}`)
	echo, err := decodeEcho(payload)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Messages []struct {
			MessageID int64 `json:"message_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(echo.RawData, &data); err != nil {
		t.Fatal(err)
	}
	if echo.Echo != "groupSummary:1:0" || len(data.Messages) != 1 || data.Messages[0].MessageID != 42 {
		t.Fatalf("decoded history = %#v, %#v", echo, data)
	}
}
