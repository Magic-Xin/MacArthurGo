package base

import (
	"github.com/gookit/config/v2"
	"log"
)

func init() {
	err := config.LoadFiles("config.json")
	if err != nil {
		log.Fatalf("Can not find config error: %v", err)
	}
}
