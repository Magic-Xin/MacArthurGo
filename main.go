package main

import (
	"MacArthurGo/base"
	"MacArthurGo/client"
	"MacArthurGo/plugins"
	"MacArthurGo/plugins/essentials"
	"context"
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
	logFile, err := setupLogging("log")
	if err != nil {
		log.Printf("Initialize logging: %v", err)
		os.Exit(1)
	}

	err = run(os.Args[1:])
	if err != nil {
		log.Printf("MacArthurGo stopped: %v", err)
	}
	if closeErr := logFile.Close(); closeErr != nil {
		log.Printf("Close log file: %v", closeErr)
	}
	if err != nil {
		os.Exit(1)
	}
}

func run(args []string) error {
	if err := base.LoadConfig(base.ConfigPath(args)); err != nil {
		return err
	}
	formatBuildTime()

	if err := essentials.OpenDatabase("cache.db"); err != nil {
		return err
	}
	defer func() {
		if err := essentials.CloseDatabase(); err != nil {
			log.Printf("Close database: %v", err)
		}
	}()

	if err := plugins.RegisterAll(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	bot := client.New(client.Options{
		Address:   base.Config.Address,
		AuthToken: base.Config.AuthToken,
	})
	essentials.StartPlugins(ctx, bot.Sender())
	defer func() {
		stop()
		essentials.StopPlugins()
	}()

	if err := bot.Run(ctx); err != nil {
		return fmt.Errorf("run OneBot client: %w", err)
	}
	log.Println("Shutdown complete")
	return nil
}

func setupLogging(directory string) (*os.File, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	location := shanghaiLocation()
	fileName := time.Now().In(location).Format("20060102150405") + ".log"
	path := filepath.Join(directory, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, file))
	return file, nil
}

func formatBuildTime() {
	if base.BuildTime == "" {
		return
	}
	buildTime, err := time.Parse(time.RFC3339, base.BuildTime)
	if err != nil {
		log.Printf("Parse build time %q: %v", base.BuildTime, err)
		return
	}
	base.BuildTime = buildTime.In(shanghaiLocation()).Format("2006-01-02 15:04:05")
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return location
}
