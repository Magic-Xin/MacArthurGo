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

func (b *Ban) ReceiveAll() *[]byte {
	return nil
}

func (b *Ban) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if messageStruct.UserId != base.Config.Admin {
		return nil
	}

	words := SplitArgument(&messageStruct.Message)
	if len(words) == 0 {
		return nil
	}
	if len(words) == 1 {
		if words[0] == "/ban-list" {
			message := "被封禁的用户有：\n"
			base.Config.Mutex.RLock()
			for _, v := range base.Config.BannedList {
				message += strconv.FormatInt(v, 10) + "\n"
			}
			base.Config.Mutex.RUnlock()
			return SendMsg(messageStruct, message, nil, false, true, "")
		}
		return nil
	}
	if words[0] == "/ban" {
		target, err := strconv.ParseInt(words[1], 10, 64)
		if err != nil {
			return SendMsg(messageStruct, "参数错误, 无法解析目标 qq 号", nil, false, true, "")
		}
		if b.IsBanned(target) {
			return SendMsg(messageStruct, "该用户已被封禁，请勿重复封禁", nil, false, true, "")
		}
		base.Config.Mutex.Lock()
		base.Config.BannedList = append(base.Config.BannedList, target)
		base.Config.Mutex.Unlock()
		base.Config.UpdateConfig()
		return SendMsg(messageStruct, fmt.Sprintf("已封禁用户: %v", target), nil, false, true, "")
	}
	if words[0] == "/unban" {
		target, err := strconv.ParseInt(words[1], 10, 64)
		if err != nil {
			return SendMsg(messageStruct, "参数错误, 无法解析目标 qq 号", nil, false, true, "")
		}
		if !b.IsBanned(target) {
			return SendMsg(messageStruct, "该用户未被封禁", nil, false, true, "")
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
		return SendMsg(messageStruct, fmt.Sprintf("已解封用户: %v", target), nil, false, true, "")
	}
	return nil
}

func (b *Ban) ReceiveEcho(*structs.EchoMessageStruct) *[]byte {
	return nil
}

func (b *Ban) IsBanned(qq int64) bool {
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
