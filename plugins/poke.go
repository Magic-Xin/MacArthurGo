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

func (*Poke) ReceiveAll() *[]byte {
	return nil
}

func (*Poke) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if !essentials.CheckArgumentArray(messageStruct.Command, &base.Config.Plugins.Poke.Args) {
		return nil
	}

	var uid int64

	for _, m := range *messageStruct.CleanMessage {
		if m.Type == "at" {
			uid, _ = strconv.ParseInt(m.Data["qq"].(string), 10, 64)
		}
		if m.Type == "text" && uid == 0 {
			uid, _ = strconv.ParseInt(m.Data["text"].(string), 10, 64)
		}
	}
	if uid != 0 {
		return essentials.SendPoke(messageStruct, uid)
	} else if messageStruct.UserId != 0 {
		return essentials.SendPoke(messageStruct, messageStruct.UserId)
	}

	return nil
}

func (*Poke) ReceiveEcho(*structs.EchoMessageStruct) *[]byte {
	return nil
}
