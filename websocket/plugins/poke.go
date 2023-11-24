package plugins

import (
	"MacArthurGo/struct/cqcode"
	"strconv"
)

func Poke(ctx *map[string]any, words *[]string) int64 {
	if len(*words) < 2 {
		return int64((*ctx)["user_id"].(float64))
	}

	cc := cqcode.FromStr((*words)[1])
	if len(*cc) > 0 {
		uid, err := strconv.ParseInt((*cc)[0].Data["qq"].(string), 10, 64)
		if err == nil {
			return uid
		}
	}

	uid, err := strconv.ParseInt((*words)[1], 10, 64)
	if err != nil {
		return int64((*ctx)["user_id"].(float64))
	}
	return uid
}
