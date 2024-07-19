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

func (p *Poke) ReceiveAll() *[]byte {
	return nil
}

func (p *Poke) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if !essentials.CheckArgumentArray(&messageStruct.Message, &base.Config.Plugins.Poke.Args) {
		return nil
	}

	words := essentials.SplitArgument(&messageStruct.Message)

	var (
		uid int64
		err error
	)

	for _, m := range messageStruct.Message {
		if m.Type == "at" {
			uid, err = strconv.ParseInt(m.Data["qq"].(string), 10, 64)
			if err != nil {
				break
			}
		}
	}

	if uid == 0 {
		if len(words) > 1 {
			uid, err = strconv.ParseInt((words)[1], 10, 64)
			if err != nil {
				uid = messageStruct.UserId
			}
		} else {
			uid = messageStruct.UserId
		}
	}

	if uid != 0 {
		return essentials.SendPoke(messageStruct, uid)
	}

	return nil
}

func (p *Poke) ReceiveEcho(*structs.EchoMessageStruct) *[]byte {
	return nil
}
