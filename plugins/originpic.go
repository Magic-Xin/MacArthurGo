package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs/cqcode"
)

type OriginPic struct {
	essentials.Plugin
}

func init() {
	originPic := OriginPic{
		essentials.Plugin{
			Name:    "原图",
			Enabled: base.Config.Plugins.OriginPic.Enable,
			Args:    base.Config.Plugins.OriginPic.Args,
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &originPic})
}

func (o OriginPic) ReceiveAll(*map[string]any, *chan []byte) {}

func (o OriginPic) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !o.Enabled {
		return
	}

	if !essentials.CheckArgumentArray(ctx, &o.Args) {
		return
	}

	message := essentials.DecodeArrayMessage(ctx)
	if message == nil {
		return
	}

	var reply []cqcode.ArrayMessage
	for _, m := range *message {
		if m.Type == "image" {
			reply = append(reply, *cqcode.Image(m.Data["url"].(string)))
		}
	}

	if len(reply) > 0 {
		*send <- *essentials.SendMsg(ctx, "", &reply, false, true)
	}
}

func (o OriginPic) ReceiveEcho(*map[string]any, *chan []byte) {}
