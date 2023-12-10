package plugins

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"github.com/gookit/config/v2"
	"log"
	"strconv"
)

type Poke struct {
	essentials.Plugin
}

func init() {
	poke := Poke{
		essentials.Plugin{
			Name:    "戳一戳",
			Enabled: config.Bool("plugins.poke.enable"),
			Args:    config.Strings("plugins.poke.args"),
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &poke})
}

func (p *Poke) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (p *Poke) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !essentials.CheckArgumentArray(ctx, &p.Args) || !p.Enabled {
		return
	}

	words := essentials.SplitArgument(ctx)

	var uid int64
	if len(words) < 2 {
		uid = int64((*ctx)["user_id"].(float64))
	} else {
		msg, err := json.Marshal((*ctx)["message"])
		if err != nil {
			log.Printf("Marshal message error: %v", err)
			return
		}
		am := cqcode.Unmarshal(msg)

		if am != nil {
			for _, m := range *am {
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
					uid = int64((*ctx)["user_id"].(float64))
				}
			}
		}
	}

	msg := essentials.SendPoke(ctx, uid)
	*send <- *msg
}

func (p *Poke) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}
