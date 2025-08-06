package client

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn        *websocket.Conn
	ServerAddr  string
	Port        int64
	AccessToken string
}

func (c *Client) Init(addr string, port int64, auth string) error {
	address := fmt.Sprintf("ws://%s:%d/event", addr, port)
	if auth != "" {
		address = fmt.Sprintf("%s?access_token=%s", address, auth)
	}
	log.Printf("Connecting to server at %s", address)
	conn, _, err := websocket.DefaultDialer.Dial(address, nil)
	if err != nil {
		log.Printf("Dial error: %v", err)
		return err
	}

	c.conn = conn
	c.ServerAddr = addr
	c.Port = port
	c.AccessToken = auth
	log.Printf("Connected to server at %s", addr)

	essentials.Info.Send = c.SendAPI

	return nil
}

func (c *Client) EventPipe() {
	defer func() {
		if err := c.conn.Close(); err != nil {
			log.Printf("Connection close error: %v", err)
		}
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected close error: %v", err)
			}
			break
		}

		if base.Config.Debug {
			log.Printf("Receive: %s\n", string(message))
		}

		go MessageFactory(&message, c.SendAPI, true)
	}
}

func (c *Client) SendAPI(api string, body interface{}) {
	url := fmt.Sprintf("http://%s:%d/api/%s", c.ServerAddr, c.Port, api)

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Printf("Request error: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Response error: %v", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Body close error: %v", err)
		}
	}(resp.Body)

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Read body error: %v", err)
		return
	}

	if base.Config.Debug {
		log.Printf("API %s response: %s", api, string(respBytes))
	}

	go MessageFactory(&respBytes, c.SendAPI, false)
}

func (c *Client) Close() {
	if err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		log.Printf("Failed to close websocket connection: %v", err)
	}
}
