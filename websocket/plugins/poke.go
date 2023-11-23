package plugins

import (
	"strconv"
)

func Poke(ctx *map[string]any, words *[]string) int {
	if len(*words) < 2 {
		return int((*ctx)["user_id"].(float64))
	}

	//cc := cqcode.FromStr((*words)[1])
	//if len(*cc) > 0 {
	//	uid := (*cc)[0].Data["qq"]
	//	if uid != nil {
	//		return int(uid.(float64))
	//	}
	//}

	uid, err := strconv.Atoi((*words)[1])
	if err != nil {
		return int((*ctx)["user_id"].(float64))
	}
	return uid
}
