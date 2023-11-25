package websocket

import (
	"MacArthurGo/plugins"
	"encoding/json"
	"log"
	"strings"
)

func MessageFactory(msg *[]byte, send *chan []byte) {
	var i any
	err := json.Unmarshal(*msg, &i)
	if err != nil {
		return
	}

	ctx := i.(map[string]any)
	if ctx["post_type"] == "message" {
		words := strings.Fields(ctx["raw_message"].(string))
		if len(words) < 1 {
			return
		}
		log.Println(ctx)

		go plugins.Poke(&ctx, &words, send)
		go plugins.Roll(&ctx, &words, send)
		go plugins.ChatGPT(&ctx, &words, send)

		go plugins.Music(&ctx, send)
		go plugins.PicSearch(&ctx, send)

		switch words[0] {
		case "/test":
			*send <- *plugins.SendMsg(&ctx, "活着呢", false, true)
		}
	}

	if ctx["echo"] != nil {
		go plugins.PicSearch(&ctx, send)
		switch ctx["echo"].(string) {
		case "info":
			plugins.Info(ctx["data"].(map[string]any))
		}
	}
}
