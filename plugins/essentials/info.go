package essentials

import (
	"github.com/gookit/config/v2"
	"log"
	"strings"
)

type LoginInfo struct {
	NickName string
	UserId   int64
	Login    bool
}

var Info LoginInfo

func init() {
	plugin := Plugin{
		Name:            "info",
		Enabled:         true,
		Arg:             "/test",
		PluginInterface: &Info,
	}
	AllArray = append(AllArray, &plugin)
	MessageArray = append(MessageArray, &plugin)
	EchoArray = append(EchoArray, &plugin)
}

func (l *LoginInfo) ReceiveAll(_ *map[string]any, send *chan []byte) {
	if Info.NickName == "" || Info.UserId == 0 {
		*send <- *SendAction("get_login_info", nil, "info")
	}
	if !Info.Login {
		sendCtx := map[string]any{
			"message_type": "private",
			"sender": map[string]any{
				"user_id": float64(config.Int64("admin")),
			},
		}
		*send <- *SendMsg(&sendCtx, "MacArthurGo 已上线", false, false)
		Info.Login = true
	}
}

func (l *LoginInfo) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if CheckArgument(ctx, "/test") {
		*send <- *SendMsg(ctx, "活着呢", false, true)
	}
	if CheckArgument(ctx, "/help") {
		result := []string{"插件: "}
		for _, p := range PluginArray {
			res := p.Name
			if !p.Enabled {
				res += "(已禁用)"
			}
			res += "	  触发指令: "
			if p.Arg != "" {
				res += p.Arg
			} else {
				res += "无"
			}
			result = append(result, res)
		}

		*send <- *SendMsg(ctx, strings.Join(result, "\n"), false, true)
	}
}

func (l *LoginInfo) ReceiveEcho(ctx *map[string]any, _ *chan []byte) {
	if (*ctx)["echo"].(string) != "info" {
		return
	}
	data := (*ctx)["data"].(map[string]any)
	Info.NickName, Info.UserId = data["nickname"].(string), int64(data["user_id"].(float64))
	log.Printf("Get account nickname: %s, id: %d", Info.NickName, Info.UserId)
}
