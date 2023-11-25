package plugins

import (
	"github.com/gookit/config/v2"
	"regexp"
	"strconv"
	"strings"
)

func Music(ctx *map[string]any, send *chan []byte) {
	if !config.Bool("plugins.music.enable") {
		return
	}

	var urlType string
	str := (*ctx)["raw_message"].(string)
	if strings.Contains(str, "music.163.com") {
		urlType = "163"
	} else if strings.Contains(str, "i.y.qq.com") {
		urlType = "qq"
	}

	if urlType != "" {
		re := regexp.MustCompile(`id=(\d+)&`)
		match := re.FindAllStringSubmatch(str, -1)
		if len(match) > 0 {
			id, err := strconv.ParseInt(match[0][1], 10, 64)
			if err == nil {
				*send <- *SendMusic(ctx, urlType, id)
			}
		}
	}
}
