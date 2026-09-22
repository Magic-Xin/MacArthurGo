package test

import (
	"MacArthurGo/base"
	botclient "MacArthurGo/client"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
)

type echoRecorder struct {
	messages atomic.Int64
	echoes   atomic.Int64
	groupID  atomic.Int64
}

func (r *echoRecorder) ReceiveMessage(*structs.MessageStruct, chan<- []byte) {
	r.messages.Add(1)
}

func (r *echoRecorder) ReceiveEcho(message *structs.EchoMessageStruct, send chan<- []byte) {
	if message.Echo != "groupList" {
		return
	}
	r.echoes.Add(1)
	if len(message.DataArray) > 0 {
		r.groupID.Store(message.DataArray[0].GroupId)
	}
	send <- []byte("group-list-received")
}

func TestCleanMessage_Scenarios(t *testing.T) {
	tests := []struct {
		name        string
		message     []cqcode.ArrayMessage
		wantCommand string
		want        []cqcode.ArrayMessage
	}{
		{
			name:        "plain text is preserved",
			message:     []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": "hello world"}}},
			wantCommand: "",
			want:        []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": "hello world"}}},
		},
		{
			name: "leading command is removed from text",
			message: []cqcode.ArrayMessage{
				{Type: "at", Data: map[string]any{"qq": "42"}},
				{Type: "text", Data: map[string]any{"text": "  /search   cats  "}},
				{Type: "image", Data: map[string]any{"file": "image.jpg"}},
			},
			wantCommand: "/search",
			want: []cqcode.ArrayMessage{
				{Type: "at", Data: map[string]any{"qq": "42"}},
				{Type: "text", Data: map[string]any{"text": "cats"}},
				{Type: "image", Data: map[string]any{"file": "image.jpg"}},
			},
		},
		{
			name:        "non-string text is preserved",
			message:     []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": 7}}},
			wantCommand: "",
			want:        []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": 7}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, command := botclient.CleanMessage(test.message)
			if command != test.wantCommand {
				t.Fatalf("command = %q, want %q", command, test.wantCommand)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Fatalf("CleanMessage() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCleanMessage_DoesNotMutateInput(t *testing.T) {
	message := []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": "/roll 10"}}}
	botclient.CleanMessage(message)
	if got := message[0].Data["text"]; got != "/roll 10" {
		t.Fatalf("input text mutated to %q", got)
	}
}

func TestMessageFactory_ReturnsAfterDispatch(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		done <- botclient.MessageFactory(
			[]byte(`{"message_type":"private","user_id":42,"message":[{"type":"text","data":{"text":"hello"}}]}`),
			make(chan []byte, 1),
		)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("MessageFactory did not return after plugin dispatch")
	}
}

func TestMessageFactory_DebugModeStillDispatchesActionEcho(t *testing.T) {
	oldConfig := base.Config
	base.Config = &base.Configuration{Debug: true, Admin: 649362775}
	t.Cleanup(func() { base.Config = oldConfig })

	recorder := &echoRecorder{}
	plugin := &essentials.Plugin{
		Name:    fmt.Sprintf("test-action-echo-%p", recorder),
		Enabled: true,
		Handler: recorder,
	}
	if err := essentials.Register(plugin); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	send := make(chan []byte, 1)
	payload := []byte(`{"status":"ok","retcode":0,"data":[{"group_id":790252716,"group_name":"test","member_count":93,"max_member_count":200}],"message":"","wording":"","echo":"groupList","stream":"normal-action"}`)
	if err := botclient.MessageFactory(payload, send); err != nil {
		t.Fatalf("MessageFactory() error = %v", err)
	}

	if got := recorder.messages.Load(); got != 0 {
		t.Fatalf("action response dispatched as %d user messages, want 0", got)
	}
	if got := recorder.echoes.Load(); got != 1 {
		t.Fatalf("action echoes dispatched = %d, want 1", got)
	}
	if got := recorder.groupID.Load(); got != 790252716 {
		t.Fatalf("decoded group id = %d, want 790252716", got)
	}
	if message := receiveWithin(t, send); string(message) != "group-list-received" {
		t.Fatalf("echo side effect = %q", message)
	}
}

func TestClient_RunDispatchesAndWrites(t *testing.T) {
	receivedAuth := make(chan string, 1)
	receivedReply := make(chan string, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			t.Errorf("Upgrade() error = %v", err)
			return
		}
		defer conn.Close()
		receivedAuth <- request.Header.Get("Authorization")
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"post_type":"meta_event"}`)); err != nil {
			t.Errorf("WriteMessage() error = %v", err)
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, reply, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("ReadMessage() error = %v", err)
			return
		}
		receivedReply <- string(reply)
	}))
	defer server.Close()

	events := make(chan string, 1)
	bot := botclient.New(botclient.Options{
		Address:      "ws" + strings.TrimPrefix(server.URL, "http"),
		AuthToken:    "secret",
		Workers:      1,
		ReconnectMin: 10 * time.Millisecond,
		ReconnectMax: 20 * time.Millisecond,
		EventHandler: func(event []byte, send chan<- []byte) error {
			events <- string(event)
			send <- []byte(`{"action":"pong"}`)
			return nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- bot.Run(ctx) }()

	if got := receiveWithin(t, receivedAuth); got != "Bearer secret" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := receiveWithin(t, events); got != `{"post_type":"meta_event"}` {
		t.Fatalf("event = %q", got)
	}
	if got := receiveWithin(t, receivedReply); got != `{"action":"pong"}` {
		t.Fatalf("reply = %q", got)
	}
	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
}

func TestClient_RunReconnects(t *testing.T) {
	var connections atomic.Int32
	secondEvent := make(chan struct{}, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			t.Errorf("Upgrade() error = %v", err)
			return
		}
		defer conn.Close()
		if connections.Add(1) == 1 {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "reconnect"),
				time.Now().Add(time.Second),
			)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"post_type":"meta_event"}`)); err != nil {
			t.Errorf("WriteMessage() error = %v", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	bot := botclient.New(botclient.Options{
		Address:      "ws" + strings.TrimPrefix(server.URL, "http"),
		Workers:      1,
		ReconnectMin: 5 * time.Millisecond,
		ReconnectMax: 10 * time.Millisecond,
		EventHandler: func([]byte, chan<- []byte) error {
			secondEvent <- struct{}{}
			return nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	runDone := make(chan error, 1)
	go func() { runDone <- bot.Run(ctx) }()

	receiveWithin(t, secondEvent)
	if got := connections.Load(); got < 2 {
		t.Fatalf("connection count = %d, want at least 2", got)
	}
	cancel()
	if err := receiveWithin(t, runDone); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func receiveWithin[T any](t *testing.T, channel <-chan T) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(2 * time.Second):
		var zero T
		t.Fatal("timed out waiting for value")
		return zero
	}
}
