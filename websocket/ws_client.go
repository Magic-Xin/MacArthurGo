package websocket

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

type Client struct {
	Conn *websocket.Conn
	Send chan []byte
}

func InitWebsocketConnection(addr string, at string) *websocket.Conn {
	header := http.Header{}
	if at != "" {
		header = http.Header{"AUTHORIZATION": []string{fmt.Sprintf("Bearer %s", at)}}
	}
	c, _, err := websocket.DefaultDialer.Dial(addr, header)
	if err != nil {
		log.Printf("Dial error: %v", err)
		return nil
	}
	return c
}

func (c *Client) ReadPump(disconnect chan bool) {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Printf("Close error: %v", err)
		}
	}(c.Conn)

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected close error: %v", err)
			}
			break
		}
		go MessageFactory(&message, c)
	}

	disconnect <- true
}

func (c *Client) WritePump(disconnect chan bool) {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Printf("Close error: %v", err)
		}
	}(c.Conn)

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					log.Printf("Client channel error: %v", err)
				}
				disconnect <- true
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Next writer error: %v", err)
				break
			}

			_, err = w.Write(message)
			if err != nil {
				log.Printf("Write message error: %v", err)
				break
			}

			err = w.Close()
			if err != nil {
				log.Printf("Writer close error: %v", err)
				break
			}
		}
	}
}
