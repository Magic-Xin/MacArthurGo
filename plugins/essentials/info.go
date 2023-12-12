package essentials

import (
	"MacArthurGo/base"
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
			Args:    []string{"/test", "/help", "/info"},
		},
	}
	PluginArray = append(PluginArray, &PluginInterface{Interface: &info})
}

func (l *LoginInfo) ReceiveAll(_ *map[string]any, send *chan []byte) {
	if !Info.Login {
		*send <- *SendAction("get_login_info", nil, "info")
		sendCtx := map[string]any{
			"message_type": "private",
			"sender": map[string]any{
				"user_id": float64(config.Int64("admin")),
			},
		}
		*send <- *SendMsg(&sendCtx, "MacArthurGo 已上线", nil, false, false)
		Info.Login = true
	}
}

func (l *LoginInfo) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if CheckArgument(ctx, l.Args[0]) {
		*send <- *SendMsg(ctx, "战斗，爽！", nil, false, true)
	}
	if CheckArgument(ctx, l.Args[1]) {
		result := []string{"插件				触发指令"}
		for _, p := range PluginArray {
			var res string
			ref := reflect.ValueOf(p.Interface)
			if name := ref.Elem().FieldByName("Name"); name.IsValid() {
				res += name.String()
			} else {
				*send <- *SendMsg(ctx, "插件解析出错", nil, false, false)
				return
			}

			if enable := ref.Elem().FieldByName("Enabled"); enable.IsValid() {
				if !enable.Bool() {
					res += "(已禁用)"
				}
			} else {
				*send <- *SendMsg(ctx, "插件解析出错", nil, false, false)
				return
			}

			if arg := ref.Elem().FieldByName("Args"); arg.IsValid() {
				res += "			"
				if arg.Interface().([]string) != nil {
					for _, a := range arg.Interface().([]string) {
						res += a + "	"
					}
				} else {
					res += "无"
				}
			} else {
				*send <- *SendMsg(ctx, "插件解析出错", nil, false, false)
				return
			}
			result = append(result, res)
		}

		*send <- *SendMsg(ctx, strings.Join(result, "\n"), nil, false, false)
	}
	if CheckArgument(ctx, l.Args[2]) {
		*send <- *SendMsg(ctx, "分支: "+base.Branch+"\n"+"版本: "+base.Version+"\n"+"编译时间: "+base.BuildTime, nil, false, false)
	}
}

func (l *LoginInfo) ReceiveEcho(ctx *map[string]any, _ *chan []byte) {
	if (*ctx)["echo"] != nil {
		if (*ctx)["echo"].(string) != "info" {
			return
		}
	}
	data := (*ctx)["data"].(map[string]any)
	Info.NickName, Info.UserId = data["nickname"].(string), int64(data["user_id"].(float64))
	log.Printf("Get account nickname: %s, id: %d", Info.NickName, Info.UserId)
}
