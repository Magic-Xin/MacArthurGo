package main

import (
	"MacArthurGo/base"
	_ "MacArthurGo/base"
	_ "MacArthurGo/plugins"
	"MacArthurGo/websocket"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

func main() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	fileName := fmt.Sprintf(time.Now().In(tz).Format("20060102150405"))
	logPath := filepath.Join(".", "log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		err = os.Mkdir(logPath, os.ModeDir|0755)
		if err != nil {
			log.Fatalf("Can not create log folder error: %v", err)
		}
	}
	logFile, err := os.OpenFile(filepath.Join(".", "log", fileName), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("Can not open or create logfile error: %v", err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	if base.BuildTime != "" {
		buildTime, _ := time.Parse(time.RFC3339, base.BuildTime)
		base.BuildTime = buildTime.In(tz).Format("2006-01-02 15:04:05")
	}

	conn, err := websocket.InitWebsocketConnection(base.Config.Address, base.Config.AuthToken)
	if err != nil {
		if base.Config.RetryTimes == 0 {
			for err != nil {
				time.Sleep(time.Duration(base.Config.RetryTimes) * time.Second)
				conn, err = websocket.InitWebsocketConnection(base.Config.Address, base.Config.AuthToken)
			}
		} else {
			for i, n := int64(0), base.Config.RetryTimes; (i < n) && (err != nil); i++ {
				time.Sleep(time.Duration(base.Config.WaitingSeconds) * time.Second)
				conn, err = websocket.InitWebsocketConnection(base.Config.Address, base.Config.AuthToken)
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
