package client

import (
	"context"
	"testing"
	"time"
)

func TestRunDrainsOutboundDuringShutdown(t *testing.T) {
	started := make(chan struct{})
	completed := make(chan struct{})
	bot := New(Options{
		Address:      "ws://[",
		QueueSize:    1,
		Workers:      1,
		ReconnectMin: time.Hour,
		ReconnectMax: time.Hour,
		EventHandler: func(_ []byte, send chan<- []byte) error {
			send <- []byte("first")
			close(started)
			send <- []byte("second")
			close(completed)
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- bot.Run(ctx) }()
	bot.events <- []byte("event")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("event handler did not start")
	}
	cancel()
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("event handler remained blocked on outbound send")
	}
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run remained blocked on event worker")
	}
}
