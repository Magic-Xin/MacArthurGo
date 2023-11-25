package plugins

import (
	"MacArthurGo/structs/cqcode"
	"github.com/gookit/config/v2"
	"strconv"
)

func Poke(ctx *map[string]any, words *[]string, send *chan []byte) {
	if (*words)[0] != config.String("plugins.poke.args") || !config.Bool("plugins.poke.enable") {
		return
	}

	var (
		uid int64
		err error
	)
	if len(*words) < 2 {
		uid = int64((*ctx)["user_id"].(float64))
	} else {
		cc := cqcode.FromStr((*words)[1])
		if len(*cc) > 0 {
			uid, err = strconv.ParseInt((*cc)[0].Data["qq"].(string), 10, 64)
			if err != nil {
				uid, err = strconv.ParseInt((*words)[1], 10, 64)
				if err != nil {
					uid = int64((*ctx)["user_id"].(float64))
				}
			}
		}
	}

	msg := SendPoke(ctx, uid)
	*send <- *msg
}
