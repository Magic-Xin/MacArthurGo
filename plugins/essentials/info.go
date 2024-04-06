package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs"
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
	PluginArray = append(PluginArray, &Plugin{Interface: &Info})
}

func (l *LoginInfo) ReceiveAll() *[]byte {
	if !l.Login {
		l.Login = true
		return SendAction("get_login_info", struct{}{}, "info")
	}
	return nil
}

func (l *LoginInfo) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if CheckArgument(&messageStruct.Message, l.Args[0]) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		message := "MacArthurGo 运行信息\n\n"

		message += "分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime + "\n"
		message += "已运行时间: " + l.timeToString(time.Now().Unix()-base.Config.StartTime) + "\n\n"

		message += "已加载插件: " + strconv.Itoa(len(PluginArray)) + " 个\n"
		message += "Goroutine 数量: " + strconv.Itoa(runtime.NumGoroutine()) + "\n\n"

		message += "内存使用情况:\n"
		message += "Alloc = " + strconv.FormatUint(mem.Alloc/1024/1024, 10) + " MB\n"
		message += "Sys = " + strconv.FormatUint(mem.Sys/1024/1024, 10) + " MB\n"
		message += "HeapAlloc = " + strconv.FormatUint(mem.HeapAlloc/1024/1024, 10) + " MB\n"

		return SendMsg(messageStruct, message, nil, false, false)
	} else if CheckArgument(&messageStruct.Message, l.Args[1]) {
		result := []string{"插件				触发指令"}
		for _, p := range PluginArray {
			var res string
			ref := reflect.ValueOf(p.Interface)
			if name := ref.Elem().FieldByName("Name"); name.IsValid() {
				res += name.String()
			} else {
				return SendMsg(messageStruct, "插件解析出错", nil, false, false)
			}

			if enable := ref.Elem().FieldByName("Enabled"); enable.IsValid() {
				if !enable.Bool() {
					res += "(已禁用)"
				}
			} else {
				return SendMsg(messageStruct, "插件解析出错", nil, false, false)
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
				return SendMsg(messageStruct, "插件解析出错", nil, false, false)
			}
			result = append(result, res)
		}

		return SendMsg(messageStruct, strings.Join(result, "\n"), nil, false, false)
	}
	return nil
}

func (l *LoginInfo) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct) *[]byte {
	if echoMessageStruct.Echo != "info" {
		return nil
	}

	data := echoMessageStruct.Data
	l.NickName, l.UserId = data.Nickname, strconv.FormatInt(data.UserId, 10)
	log.Printf("Get account nickname: %s, id: %s", l.NickName, l.UserId)

	sendStruct := structs.MessageStruct{
		MessageType: "private",
		UserId:      base.Config.Admin,
	}

	return SendMsg(&sendStruct, "MacArthurGo 已上线", nil, false, false)
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
