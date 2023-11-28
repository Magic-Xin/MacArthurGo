package websocket

import (
	"MacArthurGo/plugins/essentials"
	"encoding/json"
)

func MessageFactory(msg *[]byte, send *chan []byte) {
	//TODO B站链接解析
	//TODO 语言库回复
	//TODO setu
	var i any
	err := json.Unmarshal(*msg, &i)
	if err != nil {
		return
	}

	ctx := i.(map[string]any)
	for _, p := range essentials.PluginArray {
		go p.Interface.ReceiveAll(&ctx, send)
	}

	if ctx["post_type"] == "message" {
		if ctx["raw_message"].(string) == "" {
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
