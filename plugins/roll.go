package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

type Roll struct{}

func init() {
	plugin := &essentials.Plugin{
		Name:      "随机",
		Enabled:   base.Config.Plugins.Roll.Enable,
		Args:      base.Config.Plugins.Roll.Args,
		Interface: &Roll{},
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (*Roll) ReceiveAll(chan<- *[]byte) {}

func (r *Roll) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	if !essentials.CheckArgumentArray(messageStruct.Command, &base.Config.Plugins.Roll.Args) {
		return
	}

	words := essentials.SplitArgument(&messageStruct.Message)
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

	if result != "" {
		send <- essentials.SendMsg(messageStruct, result, nil, false, true, "")
	}
}

func (*Roll) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}

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
