package plugins

import "MacArthurGo/plugins/essentials"

type Bili struct {
	essentials.Plugin
}

func init() {
	bili := Bili{
		essentials.Plugin{
			Name:    "B 站链接解析",
			Enabled: true,
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &bili})
}

func (b *Bili) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (b *Bili) ReceiveMessage(ctx *map[string]any, send *chan []byte) {}

func (b *Bili) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}
