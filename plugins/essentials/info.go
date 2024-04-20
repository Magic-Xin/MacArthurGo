package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type LoginInfo struct {
	NickName string
	UserId   string
	Login    bool
}

var Info LoginInfo

func init() {
	plugin := &Plugin{
		Name:      "info",
		Enabled:   true,
		Args:      []string{"/info", "/help"},
		Interface: &LoginInfo{},
	}
	PluginArray = append(PluginArray, plugin)
}

func (l *LoginInfo) ReceiveAll() *[]byte {
	if !l.Login {
		l.Login = true
		return SendAction("get_login_info", struct{}{}, "info")
	}
	return nil
}

func (l *LoginInfo) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if CheckArgument(&messageStruct.Message, "/info") {
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
	} else if CheckArgument(&messageStruct.Message, "/help") {
		result := []string{"插件\t\t\t\t触发指令"}
		for _, p := range PluginArray {
			var res string
			res += p.Name
			if !p.Enabled {
				res += "(已禁用)"
			}

			res += "\t\t\t\t"
			if p.Args == nil {
				res += "无"
			} else {
				for _, arg := range p.Args {
					res += arg + "\t"
				}
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
