package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs"
	"fmt"
	"strconv"
)

type Ban struct{}

var BanList Ban

func init() {
	BanList = Ban{}
	plugin := &Plugin{
		Name:      "ban",
		Enabled:   true,
		Args:      []string{"/ban", "/unban", "/ban-list"},
		Interface: &BanList,
	}
	PluginArray = append(PluginArray, plugin)
}

func (*Ban) ReceiveAll(chan<- *[]byte) {}

func (b *Ban) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	if messageStruct.UserId != base.Config.Admin || !CheckArgumentArray(messageStruct.Command, &[]string{"/ban", "/unban", "/ban-list"}) {
		return
	}

	if messageStruct.Command == "/ban-list" {
		message := "被封禁的用户有：\n"
		base.Config.Mutex.RLock()
		for _, v := range base.Config.BannedList {
			message += strconv.FormatInt(v, 10) + "\n"
		}
		base.Config.Mutex.RUnlock()
		send <- SendMsg(messageStruct, message, nil, false, true, "")
		return
	} else {
		var (
			target int64
			err    error
		)

		for _, m := range *messageStruct.CleanMessage {
			if m.Type == "at" {
				target, err = strconv.ParseInt(m.Data["qq"].(string), 10, 64)
			}
			if m.Type == "text" && target == 0 {
				target, err = strconv.ParseInt(m.Data["text"].(string), 10, 64)
			}
		}
		if err != nil {
			send <- SendMsg(messageStruct, "参数错误, 无法解析目标 qq 号", nil, false, true, "")
			return
		}
		if target == 0 {
			send <- SendMsg(messageStruct, "参数错误, 请指定目标 qq 号", nil, false, true, "")
			return
		}
		if target == base.Config.Admin {
			send <- SendMsg(messageStruct, "无法封禁管理员", nil, false, true, "")
			return
		}

		if messageStruct.Command == "/ban" {
			if b.IsBanned(target) {
				send <- SendMsg(messageStruct, "该用户已被封禁，请勿重复封禁", nil, false, true, "")
				return
			}
			base.Config.Mutex.Lock()
			base.Config.BannedList = append(base.Config.BannedList, target)
			base.Config.Mutex.Unlock()
			base.Config.UpdateConfig()
			send <- SendMsg(messageStruct, fmt.Sprintf("已封禁用户: %v", target), nil, false, true, "")
			return
		}
		if messageStruct.Command == "/unban" {
			if !b.IsBanned(target) {
				send <- SendMsg(messageStruct, "该用户未被封禁", nil, false, true, "")
				return
			}
			base.Config.Mutex.Lock()
			for i, v := range base.Config.BannedList {
				if v == target {
					base.Config.BannedList = append(base.Config.BannedList[:i], base.Config.BannedList[i+1:]...)
					break
				}
			}
			base.Config.Mutex.Unlock()
			base.Config.UpdateConfig()
			send <- SendMsg(messageStruct, fmt.Sprintf("已解封用户: %v", target), nil, false, true, "")
		}
	}
}

func (*Ban) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}

func (*Ban) IsBanned(qq int64) bool {
	if qq == base.Config.Admin {
		return false
	}

	base.Config.Mutex.RLock()
	defer base.Config.Mutex.RUnlock()

	for _, v := range base.Config.BannedList {
		if v == qq {
			return true
		}
	}

	return false
}
