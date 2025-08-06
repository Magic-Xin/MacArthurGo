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

func (*Ban) ReceiveAll(SendFunc) {}

func (b *Ban) ReceiveMessage(incomingMessage *structs.IncomingMessageStruct, send SendFunc) {
	if incomingMessage.SenderID != base.Config.Admin || !CheckArgumentArray(incomingMessage.Command, &[]string{"/ban", "/unban", "/ban-list"}) {
		return
	}

	if incomingMessage.Command == "/ban-list" {
		message := "被封禁的用户有：\n"
		base.Config.Mutex.RLock()
		for _, v := range base.Config.BannedList {
			message += strconv.FormatInt(v, 10) + "\n"
		}
		base.Config.Mutex.RUnlock()
		SendMsg(incomingMessage, message, nil, false, true, send)
		return
	} else {
		var (
			target int64
			err    error
		)

		for _, m := range *incomingMessage.CleanMessage {
			if m.Type == "at" {
				target, err = strconv.ParseInt(m.Data["qq"].(string), 10, 64)
			}
			if m.Type == "text" && target == 0 {
				target, err = strconv.ParseInt(m.Data["text"].(string), 10, 64)
			}
		}
		if err != nil {
			SendMsg(incomingMessage, "参数错误, 无法解析目标 qq 号", nil, false, true, send)
			return
		}
		if target == 0 {
			SendMsg(incomingMessage, "参数错误, 请指定目标 qq 号", nil, false, true, send)
			return
		}
		if target == base.Config.Admin {
			SendMsg(incomingMessage, "无法封禁管理员", nil, false, true, send)
			return
		}

		if incomingMessage.Command == "/ban" {
			if b.IsBanned(target) {
				SendMsg(incomingMessage, "该用户已被封禁，请勿重复封禁", nil, false, true, send)
				return
			}
			base.Config.Mutex.Lock()
			base.Config.BannedList = append(base.Config.BannedList, target)
			base.Config.Mutex.Unlock()
			base.Config.UpdateConfig()
			SendMsg(incomingMessage, fmt.Sprintf("已封禁用户: %v", target), nil, false, true, send)
			return
		}
		if incomingMessage.Command == "/unban" {
			if !b.IsBanned(target) {
				SendMsg(incomingMessage, "该用户未被封禁", nil, false, true, send)
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
			SendMsg(incomingMessage, fmt.Sprintf("已解封用户: %v", target), nil, false, true, send)
		}
	}
}

func (*Ban) ReceiveEcho(*structs.FeedbackStruct, SendFunc) {}

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
