package plugins

import (
	"MacArthurGo/plugins/essentials"
	"github.com/gookit/config/v2"
	"regexp"
	"strconv"
	"strings"
)

type Music struct{}

func init() {
	music := essentials.Plugin{
		Name:            "音乐链接解析",
		Enabled:         config.Bool("plugins.music.enable"),
		Arg:             "",
		PluginInterface: &Music{},
	}
	essentials.PluginArray = append(essentials.PluginArray, &music)

	essentials.MessageArray = append(essentials.MessageArray, &music)
}

func (m *Music) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (m *Music) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
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
				*send <- *essentials.SendMusic(ctx, urlType, id)
			}
		}
	}
}

func (m *Music) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}
