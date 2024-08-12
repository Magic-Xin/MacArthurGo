package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type LoginInfo struct {
	NickName   string
	UserId     string
	FriendList []Friend
	GroupList  []Group
	status     int64
}

type Friend struct {
	UserId   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
}

type Group struct {
	GroupId        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	MemberCount    int    `json:"member_count"`
	MaxMemberCount int    `json:"max_member_count"`
}

var Info LoginInfo

func init() {
	Info = LoginInfo{}
	plugin := &Plugin{
		Name:      "info",
		Enabled:   true,
		Args:      []string{"/info", "/help"},
		Interface: &Info,
	}
	PluginArray = append(PluginArray, plugin)
}

func (l *LoginInfo) ReceiveAll() *[]byte {
	switch l.status {
	case 0:
		l.status++
		return SendAction("get_login_info", struct{}{}, "info")
	case 1:
		l.status++
		return SendAction("get_friend_list", struct{}{}, "friendList")
	case 2:
		l.status++
		return SendAction("get_group_list", struct{}{}, "groupList")
	default:
		return nil
	}
}

func (l *LoginInfo) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if messageStruct.Command == "/info" {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		message := "MacArthurGo 运行信息\n\n"

		message += "分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime + "\n"
		message += "已运行时间: " + l.timeToString(time.Now().Unix()-base.Config.StartTime) + "\n\n"

		message += "已添加好友数量: " + strconv.Itoa(len(l.FriendList)) + "\n"
		message += "已加入群组数量: " + strconv.Itoa(len(l.GroupList)) + "\n\n"

		message += "已加载插件: " + strconv.Itoa(len(PluginArray)) + " 个\n"
		message += "Goroutine 数量: " + strconv.Itoa(runtime.NumGoroutine()) + "\n\n"

		message += "内存使用情况:\n"
		message += "Alloc = " + strconv.FormatUint(mem.Alloc/1024/1024, 10) + " MB\n"
		message += "Sys = " + strconv.FormatUint(mem.Sys/1024/1024, 10) + " MB\n"
		message += "HeapAlloc = " + strconv.FormatUint(mem.HeapAlloc/1024/1024, 10) + " MB\n"

		return SendMsg(messageStruct, message, nil, false, false, "")
	} else if messageStruct.Command == "/help" {
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

		return SendMsg(messageStruct, strings.Join(result, "\n"), nil, false, false, "")
	}
	return nil
}

func (l *LoginInfo) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct) *[]byte {
	if echoMessageStruct.Status != "ok" {
		return nil
	}

	sendStruct := structs.MessageStruct{
		MessageType: "private",
		UserId:      base.Config.Admin,
	}

	switch echoMessageStruct.Echo {
	case "info":
		data := echoMessageStruct.Data
		l.NickName, l.UserId = data.Nickname, strconv.FormatInt(data.UserId, 10)
		log.Printf("Get account nickname: %s, id: %s", l.NickName, l.UserId)
		return SendMsg(&sendStruct, "MacArthurGo 已上线", nil, false, false, "")
	case "friendList":
		data := echoMessageStruct.DataArray
		bytesData, err := json.Marshal(data)
		if err != nil {
			log.Printf("FriendList Marshal error: %v", err)
			return nil
		}
		err = json.Unmarshal(bytesData, &l.FriendList)
		if err != nil {
			log.Printf("FriendList Unmarshal error: %v", err)
			return nil
		}

		log.Printf("Get friend list count: %d", len(l.FriendList))
		return SendMsg(&sendStruct, fmt.Sprintf("好友列表加载成功，好友数量: %d", len(l.FriendList)), nil, false, false, "")
	case "groupList":
		data := echoMessageStruct.DataArray
		bytesData, err := json.Marshal(data)
		if err != nil {
			log.Printf("GroupList Marshal error: %v", err)
			return nil
		}
		err = json.Unmarshal(bytesData, &l.GroupList)
		if err != nil {
			log.Printf("GroupList Unmarshal error: %v", err)
			return nil
		}

		log.Printf("Get group list count: %d", len(l.GroupList))
		return SendMsg(&sendStruct, fmt.Sprintf("群组列表加载成功，群组数量: %d", len(l.GroupList)), nil, false, false, "")
	}

	return nil
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
