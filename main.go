package main

import (
	"MacArthurGo/base"
	_ "MacArthurGo/base"
	"MacArthurGo/client"
	_ "MacArthurGo/plugins"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	tz, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		tz = time.FixedZone("Asia/Shanghai", 8*60*60)
	}

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

	c := &client.Client{}

	err = c.Init(base.Config.Address, base.Config.Port, base.Config.AuthToken)
	for err != nil {
		time.Sleep(30 * time.Second)
		log.Println("Can not connect to server, retrying...")
		err = c.Init(base.Config.Address, base.Config.Port, base.Config.AuthToken)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	go c.EventPipe()

	<-interrupt
	log.Println("Shutting down...")
	c.Close()
}
