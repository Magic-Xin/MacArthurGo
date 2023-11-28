package essentials

import (
	"github.com/gookit/config/v2"
	"log"
	"reflect"
	"strings"
)

type LoginInfo struct {
	Plugin
	NickName string
	UserId   int64
	Login    bool
}

var Info LoginInfo

func init() {
	info := LoginInfo{
		Plugin: Plugin{
			Name:    "info",
			Enabled: true,
			Arg:     "/test",
		},
	}
	PluginArray = append(PluginArray, &PluginInterface{Interface: &info})
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
	if CheckArgument(ctx, l.Arg) {
		*send <- *SendMsg(ctx, "活着呢", false, true)
	}
	if CheckArgument(ctx, "/help") {
		result := []string{"插件: "}
		for _, p := range PluginArray {
			var res string
			ref := reflect.ValueOf(p.Interface)
			if name := ref.Elem().FieldByName("Name"); name.IsValid() {
				res += name.String()
			} else {
				*send <- *SendMsg(ctx, "插件解析出错", false, false)
				return
			}

			if enable := ref.Elem().FieldByName("Enabled"); enable.IsValid() {
				if !enable.Bool() {
					res += "(已禁用)"
				}
			} else {
				*send <- *SendMsg(ctx, "插件解析出错", false, false)
				return
			}

			if arg := ref.Elem().FieldByName("Arg"); arg.IsValid() {
				res += "		触发指令: "
				if arg.String() != "" {
					res += arg.String()
				} else {
					res += "无"
				}
			} else {
				*send <- *SendMsg(ctx, "插件解析出错", false, false)
				return
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
