package essentials

import (
	"MacArthurGo/base"
	"fmt"
	"log"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type LoginInfo struct {
	Plugin
	NickName string
	UserId   string
	Login    bool
}

var Info LoginInfo

func init() {
	Info = LoginInfo{
		Plugin: Plugin{
			Name:    "info",
			Enabled: true,
			Args:    []string{"/info", "/help"},
		},
	}
	PluginArray = append(PluginArray, &PluginInterface{Interface: &Info})
}

func (l *LoginInfo) ReceiveAll(_ *map[string]any, send *chan []byte) {
	if !l.Login {
		*send <- *SendAction("get_login_info", struct{}{}, "info")
		l.Login = true
	}
}

func (l *LoginInfo) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if CheckArgument(ctx, l.Args[0]) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		message := "MacArthurGo 运行信息\n\n"

		message += "分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime + "\n"
		message += "已运行时间: " + l.timeToString(time.Now().Unix()-base.Config.StartTime) + "\n\n"

		message += "内存使用情况:\n"
		message += "Alloc = " + strconv.FormatUint(mem.Alloc/1024/1024, 10) + " MB\n"
		message += "Sys = " + strconv.FormatUint(mem.Sys/1024/1024, 10) + " MB\n"
		message += "HeapAlloc = " + strconv.FormatUint(mem.HeapAlloc/1024/1024, 10) + " MB\n"

		*send <- *SendMsg(ctx, message, nil, false, false)
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
}

func (l *LoginInfo) ReceiveEcho(ctx *map[string]any, send *chan []byte) {
	if (*ctx)["echo"].(string) != "info" || (*ctx)["data"] == nil {
		return
	}

	data := (*ctx)["data"].(map[string]any)
	l.NickName, l.UserId = data["nickname"].(string), strconv.FormatInt(int64(data["user_id"].(float64)), 10)
	log.Printf("Get account nickname: %s, id: %s", l.NickName, l.UserId)
	sendCtx := map[string]any{
		"message_type": "private",
		"sender": map[string]any{
			"user_id": float64(base.Config.Admin),
		},
	}
	*send <- *SendMsg(&sendCtx, "MacArthurGo 已上线", nil, false, false)
}

func (l *LoginInfo) timeToString(time int64) string {
	if time/60 == 0 {
		return fmt.Sprintf("%d秒", time)
	} else if time/3600 == 0 {
		return fmt.Sprintf("%d分%d秒", time/60, time%60)
	} else if time/86400 == 0 {
		return fmt.Sprintf("%d小时%d分%d秒", time/3600, time%3600/60, time%3600%60)
	}

	return fmt.Sprintf("%d天%d小时%d分%d秒", time/86400, time%86400/3600, time%86400%3600/60, time%86400%3600%60)
}
