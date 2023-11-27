package plugins

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs/cqcode"
	"github.com/gookit/config/v2"
	"strconv"
)

type Poke struct{}

func init() {
	poke := essentials.Plugin{
		Name:            "戳一戳",
		Enabled:         config.Bool("plugins.poke.enable"),
		Arg:             config.String("plugins.poke.args"),
		PluginInterface: &Poke{},
	}
	essentials.PluginArray = append(essentials.PluginArray, &poke)

	essentials.MessageArray = append(essentials.MessageArray, &poke)
}

func (p *Poke) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (p *Poke) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !essentials.CheckArgument(ctx, config.String("plugins.poke.args")) || !config.Bool("plugins.poke.enable") {
		return
	}

	words := essentials.SplitArgument(ctx)

	var (
		uid int64
		err error
	)
	if len(words) < 2 {
		uid = int64((*ctx)["user_id"].(float64))
	} else {
		cc := cqcode.FromStr((words)[1])
		if len(*cc) > 0 {
			uid, err = strconv.ParseInt((*cc)[0].Data["qq"].(string), 10, 64)
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
