package plugins

import (
	"MacArthurGo/plugins/essentials"
	"github.com/gookit/config/v2"
	"regexp"
)

type Bili struct {
	essentials.Plugin
}

func init() {
	bili := Bili{
		essentials.Plugin{
			Name:    "B 站链接解析",
			Enabled: config.Bool("plugins.chatGPT.groupForward"),
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &bili})
}

func (b *Bili) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (b *Bili) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	biliShort := regexp.MustCompile(`"(https://b23.tv/\w+)`)
	biliLong := regexp.MustCompile(`(https://www.bilibili.com/video/\w+)`)

	rawMsg := (*ctx)["raw_message"].(string)
	var url string
	if match := biliShort.FindAllStringSubmatch(rawMsg, -1); match != nil {
		url := biliLong.FindAllStringSubmatch(*essentials.GetOriginUrl(match[0][1]), -1)[0][1]
	} else if match := biliLong.FindAllStringSubmatch(rawMsg, -1); match != nil {
		url = match[0][1]
	}
}

func (b *Bili) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}
