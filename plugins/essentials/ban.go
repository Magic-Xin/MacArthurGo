package essentials

import (
	"MacArthurGo/base"
	"fmt"
	"strconv"
)

type Ban struct {
	Plugin
}

var BanList Ban

func init() {
	BanList = Ban{
		Plugin: Plugin{
			Name:    "禁止响应指定用户",
			Enabled: true,
		},
	}

	PluginArray = append(PluginArray, &PluginInterface{Interface: &BanList})
}

func (b *Ban) ReceiveAll(*map[string]any, *chan []byte) {}

func (b *Ban) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !b.Plugin.Enabled {
		return
	}
	if int64((*ctx)["user_id"].(float64)) != base.Config.Admin {
		return
	}

	words := SplitArgument(ctx)
	if len(words) == 0 {
		return
	}
	if len(words) == 1 {
		if words[0] == "/ban-list" {
			message := "被封禁的用户有：\n"
			base.Config.Mutex.RLock()
			for _, v := range base.Config.BannedList {
				message += strconv.FormatInt(v, 10) + "\n"
			}
			base.Config.Mutex.RUnlock()
			*send <- *SendMsg(ctx, message, nil, false, true)
		}
		return
	}
	if words[0] == "/ban" {
		target, err := strconv.ParseInt(words[1], 10, 64)
		if err != nil {
			*send <- *SendMsg(ctx, "参数错误, 无法解析目标 qq 号", nil, false, true)
			return
		}
		if b.IsBanned(target) {
			*send <- *SendMsg(ctx, "该用户已被封禁，请勿重复封禁", nil, false, true)
			return
		}
		base.Config.Mutex.Lock()
		base.Config.BannedList = append(base.Config.BannedList, target)
		base.Config.Mutex.Unlock()
		base.Config.UpdateConfig()
		*send <- *SendMsg(ctx, fmt.Sprintf("已封禁用户: %v", target), nil, false, true)
	}
	if words[0] == "/unban" {
		target, err := strconv.ParseInt(words[1], 10, 64)
		if err != nil {
			*send <- *SendMsg(ctx, "参数错误, 无法解析目标 qq 号", nil, false, true)
			return
		}
		if !b.IsBanned(target) {
			*send <- *SendMsg(ctx, "该用户未被封禁", nil, false, true)
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
		*send <- *SendMsg(ctx, fmt.Sprintf("已解封用户: %v", target), nil, false, true)
	}
}

func (b *Ban) ReceiveEcho(*map[string]any, *chan []byte) {}

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
