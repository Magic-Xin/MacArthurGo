package plugins

import (
	"MacArthurGo/plugins/essentials"
	"fmt"
	"github.com/gookit/config/v2"
	"math/rand"
	"strconv"
	"time"
)

type Roll struct {
	essentials.Plugin
}

func init() {
	roll := Roll{
		essentials.Plugin{
			Name:    "随机",
			Enabled: config.Bool("plugins.roll.enable"),
			Args:    config.Strings("plugins.roll.args"),
		},
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &roll})
}

func (r *Roll) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (r *Roll) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !essentials.CheckArgumentArray(ctx, &r.Args) || !r.Enabled {
		return
	}

	words := essentials.SplitArgument(ctx)
	var result string
	if len(words) == 1 {
		result = r.getRoll(-1)
	} else if len(words) == 2 {
		n, err := strconv.Atoi((words)[1])
		if err != nil {
			result = r.getRoll(-1)
		} else {
			result = r.getRoll(n)
		}
	} else {
		result = r.getRollContent((words)[1:])
	}

	msg := essentials.SendMsg(ctx, result, nil, false, true)
	*send <- *msg
}

func (r *Roll) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}

func (*Roll) getRoll(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if n < 1 {
		return fmt.Sprintf("生成 [0-9] 随机值：%d", r.Intn(10))
	}
	return fmt.Sprintf("生成 [0-%d] 随机值：%d", n, r.Intn(n))
}

func (*Roll) getRollContent(content []string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("随机结果为：%s", content[r.Intn(len(content))])
}
