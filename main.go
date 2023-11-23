package main

import (
	"MacArthurGo/struct"
	"MacArthurGo/websocket"
	"github.com/gookit/config/v2"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	cfg := _struct.Config{}
	err := config.LoadFiles("config.json")
	if err != nil {
		panic(err)
	}
	err = config.Decode(&cfg)

	conn := websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
	if conn == nil {
		if config.Int("retryTimes") == 0 {
			for conn == nil {
				time.Sleep(time.Duration(config.Int("waitingSeconds")) * time.Second)
				conn = websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
			}
		} else {
			for i, n := 0, config.Int("retryTimes"); (i < n) && (conn == nil); i++ {
				time.Sleep(time.Duration(config.Int("waitingSeconds")) * time.Second)
				conn = websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
			}
		}
	}
	client := &websocket.Client{Conn: conn, Send: make(chan []byte)}

	disconnect := make(chan bool)
	go client.ReadPump(disconnect)
	go client.WritePump(disconnect)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		select {
		case <-interrupt:
			log.Println("interrupt")
			return
		case <-disconnect:
			log.Println("Trying reconnect...")
			conn = websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
			if config.Int("retryTimes") == 0 {
				for conn == nil {
					time.Sleep(time.Duration(config.Int("waitingSeconds")) * time.Second)
					conn = websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
				}
			} else {
				for i, n := 0, config.Int("retryTimes"); (i < n) && (conn == nil); i++ {
					time.Sleep(time.Duration(config.Int("waitingSeconds")) * time.Second)
					conn = websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
				}
			}
			if conn != nil {
				client.Conn = conn
				disconnect <- false
				go client.ReadPump(disconnect)
				go client.WritePump(disconnect)
			}
		}
	}
}
