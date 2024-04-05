package websocket

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"encoding/json"
	"log"
)

func MessageFactory(msg *[]byte, send *chan []byte) {
	var i any
	err := json.Unmarshal(*msg, &i)
	if err != nil {
		return
	}

	ctx := i.(map[string]any)

	if base.Config.Debug {
		log.Printf("Received message: %v", ctx)
	}

	for _, p := range essentials.PluginArray {
		go p.Interface.ReceiveAll(&ctx, send)
	}

	if ctx["post_type"] == "message" {
		if ctx["raw_message"].(string) == "" {
			return
		}
		if essentials.BanList.IsBanned(int64((ctx)["sender"].(map[string]any)["user_id"].(float64))) {
			return
		}

		for _, p := range essentials.PluginArray {
			go p.Interface.ReceiveMessage(&ctx, send)
		}
	}

	if ctx["echo"] != nil {
		for _, p := range essentials.PluginArray {
			go p.Interface.ReceiveEcho(&ctx, send)
		}
	}
}
