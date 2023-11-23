package plugins

import (
	"MacArthurGo/struct/cqcode"
	"strconv"
)

func Poke(ctx *map[string]any, words *[]string) int {
	if len(*words) < 2 {
		return int((*ctx)["user_id"].(float64))
	}

	cc := cqcode.FromStr((*words)[1])
	if len(*cc) > 0 {
		uid, err := strconv.Atoi((*cc)[0].Data["qq"].(string))
		if err == nil {
			return uid
		}
	}

	uid, err := strconv.Atoi((*words)[1])
	if err != nil {
		return int((*ctx)["user_id"].(float64))
	}
	return uid
}
