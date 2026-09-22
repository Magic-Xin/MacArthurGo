package client

import (
	"MacArthurGo/base"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultQueueSize    = 256
	defaultReadLimit    = 8 << 20
	writeTimeout        = 10 * time.Second
	pongTimeout         = 60 * time.Second
	pingInterval        = 45 * time.Second
	minReconnectDelay   = time.Second
	maxReconnectDelay   = 30 * time.Second
	stableConnectionAge = time.Minute
)

type EventHandler func([]byte, chan<- []byte) error

type Options struct {
	Address      string
	AuthToken    string
	QueueSize    int
	Workers      int
	EventHandler EventHandler
	Dialer       *websocket.Dialer
	ReconnectMin time.Duration
	ReconnectMax time.Duration
}

// Client owns a persistent inbound worker pool and outbound queue. Individual
// WebSocket connections can be replaced without changing the channel plugins
// use to send actions.
type Client struct {
	address      string
	authToken    string
	dialer       *websocket.Dialer
	handler      EventHandler
	workers      int
	reconnectMin time.Duration
	reconnectMax time.Duration
	outbound     chan []byte
	events       chan []byte
}

func New(options Options) *Client {
	queueSize := options.QueueSize
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	workers := options.Workers
	if workers <= 0 {
		workers = max(4, runtime.GOMAXPROCS(0))
	}
	handler := options.EventHandler
	if handler == nil {
		handler = MessageFactory
	}
	dialer := options.Dialer
	if dialer == nil {
		clone := *websocket.DefaultDialer
		dialer = &clone
	}
	reconnectMin := options.ReconnectMin
	if reconnectMin <= 0 {
		reconnectMin = minReconnectDelay
	}
	reconnectMax := options.ReconnectMax
	if reconnectMax < reconnectMin {
		reconnectMax = maxReconnectDelay
	}
	return &Client{
		address:      options.Address,
		authToken:    options.AuthToken,
		dialer:       dialer,
		handler:      handler,
		workers:      workers,
		reconnectMin: reconnectMin,
		reconnectMax: reconnectMax,
		outbound:     make(chan []byte, queueSize),
		events:       make(chan []byte, queueSize),
	}
}

func (c *Client) Sender() chan<- []byte {
	return c.outbound
}

// Run maintains the WebSocket connection until ctx is canceled. Disconnects
// are retried with bounded exponential backoff.
func (c *Client) Run(ctx context.Context) error {
	if c.address == "" {
		return errors.New("websocket address is empty")
	}

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	workersDone := c.startWorkers(workerCtx)
	defer func() {
		cancelWorkers()
		select {
		case <-workersDone:
		case <-time.After(writeTimeout):
			log.Printf("Timed out waiting for event workers to stop")
		}
	}()

	delay := c.reconnectMin
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		connectedAt := time.Now()
		err := c.runConnection(ctx)
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("WebSocket disconnected: %v", err)
		if time.Since(connectedAt) >= stableConnectionAge {
			delay = c.reconnectMin
		}
		log.Printf("Reconnecting in %s", delay)
		if !waitContext(ctx, delay) {
			return nil
		}
		delay = min(delay*2, c.reconnectMax)
	}
}

func (c *Client) startWorkers(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	remaining := make(chan struct{}, c.workers)
	for range c.workers {
		go func() {
			defer func() { remaining <- struct{}{} }()
			for {
				select {
				case <-ctx.Done():
					return
				case event := <-c.events:
					if err := c.handler(event, c.outbound); err != nil {
						log.Printf("Handle OneBot event: %v", err)
					}
				}
			}
		}()
	}
	go func() {
		for range c.workers {
			<-remaining
		}
		close(done)
	}()
	return done
}

func (c *Client) runConnection(ctx context.Context) error {
	header := make(http.Header)
	if c.authToken != "" {
		header.Set("Authorization", "Bearer "+c.authToken)
	}
	conn, response, err := c.dialer.DialContext(ctx, c.address, header)
	if err != nil {
		if response != nil {
			return fmt.Errorf("dial websocket (%s): %w", response.Status, err)
		}
		return fmt.Errorf("dial websocket: %w", err)
	}
	log.Printf("WebSocket connected to %s", c.address)

	connectionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer conn.Close()

	errorsCh := make(chan error, 2)
	go func() { errorsCh <- c.readPump(connectionCtx, conn) }()
	go func() { errorsCh <- c.writePump(connectionCtx, conn) }()

	select {
	case <-ctx.Done():
		cancel()
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"),
			time.Now().Add(writeTimeout),
		)
		return nil
	case err := <-errorsCh:
		cancel()
		return err
	}
}

func (c *Client) readPump(ctx context.Context, conn *websocket.Conn) error {
	conn.SetReadLimit(defaultReadLimit)
	_ = conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return fmt.Errorf("read websocket: %w", err)
		}
		if base.Config.Debug {
			log.Printf("Receive: %s", message)
		}
		select {
		case c.events <- message:
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *Client) writePump(ctx context.Context, conn *websocket.Conn) error {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case message := <-c.outbound:
			if len(message) == 0 {
				continue
			}
			if base.Config.Debug {
				log.Printf("Send: %s", debugOutboundMessage(message))
			}
			if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				return fmt.Errorf("set websocket write deadline: %w", err)
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return fmt.Errorf("write websocket: %w", err)
			}
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return fmt.Errorf("ping websocket: %w", err)
			}
		}
	}
}

func debugOutboundMessage(message []byte) string {
	var action struct {
		Action string `json:"action"`
		Echo   string `json:"echo"`
		Params struct {
			StreamID    string `json:"stream_id"`
			ChunkIndex  int    `json:"chunk_index"`
			TotalChunks int    `json:"total_chunks"`
			IsComplete  bool   `json:"is_complete"`
		} `json:"params"`
	}
	if err := json.Unmarshal(message, &action); err != nil || action.Action != "upload_file_stream" {
		return string(message)
	}
	if action.Params.IsComplete {
		return fmt.Sprintf("upload_file_stream stream_id=%s complete=true echo=%s payload_bytes=%d", action.Params.StreamID, action.Echo, len(message))
	}
	return fmt.Sprintf("upload_file_stream stream_id=%s chunk=%d/%d echo=%s payload_bytes=%d", action.Params.StreamID, action.Params.ChunkIndex+1, action.Params.TotalChunks, action.Echo, len(message))
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
