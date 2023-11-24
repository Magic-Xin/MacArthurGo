package main

import (
	"MacArthurGo/websocket"
	"github.com/gookit/config/v2"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	err := config.LoadFiles("config.json")
	if err != nil {
		log.Fatalf("Can not find config error: %v", err)
	}

	conn, err := websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
	if err != nil {
		if config.Int("retryTimes") == 0 {
			for err != nil {
				time.Sleep(time.Duration(config.Int("waitingSeconds")) * time.Second)
				conn, err = websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
			}
		} else {
			for i, n := 0, config.Int("retryTimes"); (i < n) && (err != nil); i++ {
				time.Sleep(time.Duration(config.Int("waitingSeconds")) * time.Second)
				conn, err = websocket.InitWebsocketConnection(config.String("address"), config.String("authToken"))
			}
		}
	}
	client := &websocket.Client{Conn: conn, Send: make(chan []byte)}

	go client.ReadPump()
	go client.WritePump()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		select {
		case <-interrupt:
			log.Println("interrupt")
			return
		}
	}
}
