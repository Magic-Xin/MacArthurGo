package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"strconv"
)

type Poke struct{}

func init() {
	plugin := &essentials.Plugin{
		Name:      "戳一戳",
		Enabled:   base.Config.Plugins.Poke.Enable,
		Args:      base.Config.Plugins.Poke.Args,
		Interface: &Poke{},
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (*Poke) ReceiveAll(essentials.SendFunc) {}

func (*Poke) ReceiveMessage(incomingMessageStruct *structs.IncomingMessageStruct, send essentials.SendFunc) {
	if !essentials.CheckArgumentArray(incomingMessageStruct.Command, &base.Config.Plugins.Poke.Args) || incomingMessageStruct.MessageScene != "group" {
		return
	}

	var uid int64

	for _, m := range *incomingMessageStruct.CleanMessage {
		if m.Type == "at" {
			uid, _ = strconv.ParseInt(m.Data["qq"].(string), 10, 64)
		}
		if m.Type == "text" && uid == 0 {
			uid, _ = strconv.ParseInt(m.Data["text"].(string), 10, 64)
		}
	}
	if uid != 0 {
		essentials.SendGroupNudge(incomingMessageStruct, uid, send)
	} else if incomingMessageStruct.SenderID != 0 {
		essentials.SendGroupNudge(incomingMessageStruct, incomingMessageStruct.SenderID, send)
	}
}

func (*Poke) ReceiveEcho(*structs.FeedbackStruct, essentials.SendFunc) {}
