package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"strconv"
)

type Poke struct {
	essentials.Plugin
}

func init() {
	poke := Poke{
		essentials.Plugin{
			Name:    "戳一戳",
			Enabled: base.Config.Plugins.Poke.Enable,
			Args:    base.Config.Plugins.Poke.Args,
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.Plugin{Interface: &poke})
}

func (p *Poke) ReceiveAll() *[]byte {
	return nil
}

func (p *Poke) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if !essentials.CheckArgumentArray(&messageStruct.Message, &p.Args) || !p.Enabled {
		return nil
	}

	words := essentials.SplitArgument(&messageStruct.Message)

	var (
		uid int64
		err error
	)

	if len(words) < 2 {
		uid = messageStruct.UserId
	} else {
		msg := messageStruct.Message
		if msg != nil {
			for _, m := range msg {
				if m.Type == "at" {
					uid, err = strconv.ParseInt(m.Data["qq"].(string), 10, 64)
					if err != nil {
						break
					}
				}
			}
			if err != nil {
				uid, err = strconv.ParseInt((words)[1], 10, 64)
				if err != nil {
					uid = messageStruct.UserId
				}
			}
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
