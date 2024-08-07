package client

import (
	"MacArthurGo/base"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

type Client struct {
	Conn     *websocket.Conn
	SendPump chan *[]byte
}

func InitWebsocketConnection(addr string, at string) (*websocket.Conn, error) {
	header := http.Header{}
	if at != "" {
		header = http.Header{"AUTHORIZATION": []string{fmt.Sprintf("Bearer %s", at)}}
	}
	c, _, err := websocket.DefaultDialer.Dial(addr, header)
	if err != nil {
		log.Printf("Dial error: %v", err)
		return nil, err
	}
	return c, nil
}

func (c *Client) ReadPump() {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Close error: %v", err)
		}
	}(c.Conn)

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Fatalf("Unexpected close error: %v", err)
			}
			break
		}

		if base.Config.Debug {
			log.Printf("Receive: %s\n", string(message))
		}

		go func() {
			MessageFactory(&message, c.SendPump)
		}()
	}
}

func (c *Client) WritePump() {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Close error: %v", err)
		}
	}(c.Conn)

	for {
		select {
		case message, ok := <-c.SendPump:
			if message == nil {
				continue
			}

			if !ok {
				err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					log.Fatalf("Client channel error: %v", err)
				}
				return
			}

			if base.Config.Debug {
				log.Printf("Send: %s\n", string(*message))
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Fatalf("Next writer error: %v", err)
			}

			_, err = w.Write(*message)
			if err != nil {
				log.Fatalf("Write message error: %v", err)
			}

			err = w.Close()
			if err != nil {
				log.Fatalf("Writer close error: %v", err)
			}
		}
	}
}

func (c *Client) Close() {
	if err := c.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		log.Printf("Failed to close websocket connection: %v", err)
		return
	}

	close(c.SendPump)
}
