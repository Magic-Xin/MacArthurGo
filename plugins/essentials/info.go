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
	send       chan<- *[]byte
	NickName   string
	UserId     string
	FriendList []Friend
	GroupList  []Group
	IsOnline   bool
	UpdateTime []int64
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
	Info = LoginInfo{
		UpdateTime: []int64{0, 0, 0},
	}
	plugin := &Plugin{
		Name:      "info",
		Enabled:   true,
		Args:      []string{"/info", "/help"},
		Interface: &Info,
	}
	PluginArray = append(PluginArray, plugin)

	go SchedulerRequireUpdate(&Info)
}

func (l *LoginInfo) ReceiveAll(send chan<- *[]byte) {
	if l.send == nil && send != nil {
		l.send = send
	}
}

func (l *LoginInfo) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
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

		send <- SendMsg(messageStruct, message, nil, false, false, "")
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

		send <- SendMsg(messageStruct, strings.Join(result, "\n"), nil, false, false, "")
	} else if messageStruct.Command == "/info_update" {
		if messageStruct.UserId != base.Config.Admin {
			send <- SendMsg(messageStruct, "该指令仅限管理员使用", nil, false, true, "")
			return
		}

		l.RequireUpdate()
		send <- SendMsg(messageStruct, "信息更新请求已发送", nil, false, false, "")
	}
	return
}

func (l *LoginInfo) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, send chan<- *[]byte) {
	if echoMessageStruct.Status != "ok" {
		return
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
		if !l.IsOnline {
			send <- SendMsg(&sendStruct, "MacArthurGo 已上线", nil, false, false, "")
			l.IsOnline = true
		}
		l.UpdateTime[0] = time.Now().Unix()
	case "friendList":
		data := echoMessageStruct.DataArray
		bytesData, err := json.Marshal(data)
		if err != nil {
			log.Printf("FriendList Marshal error: %v", err)
			return
		}
		err = json.Unmarshal(bytesData, &l.FriendList)
		if err != nil {
			log.Printf("FriendList Unmarshal error: %v", err)
			return
		}

		log.Printf("Get friend list count: %d", len(l.FriendList))
		//send <- SendMsg(&sendStruct, fmt.Sprintf("好友列表加载成功，好友数量: %d", len(l.FriendList)), nil, false, false, "")
		l.UpdateTime[1] = time.Now().Unix()
	case "groupList":
		data := echoMessageStruct.DataArray
		bytesData, err := json.Marshal(data)
		if err != nil {
			log.Printf("GroupList Marshal error: %v", err)
			return
		}
		err = json.Unmarshal(bytesData, &l.GroupList)
		if err != nil {
			log.Printf("GroupList Unmarshal error: %v", err)
			return
		}

		log.Printf("Get group list count: %d", len(l.GroupList))
		//send <- SendMsg(&sendStruct, fmt.Sprintf("群组列表加载成功，群组数量: %d", len(l.GroupList)), nil, false, false, "")
		l.UpdateTime[2] = time.Now().Unix()
	}

	return
}

func (*LoginInfo) timeToString(time int64) string {
	if time/60 == 0 {
		return fmt.Sprintf("%d秒", time)
	} else if time/3600 == 0 {
		return fmt.Sprintf("%d分%d秒", time/60, time%60)
	} else if time/86400 == 0 {
		return fmt.Sprintf("%d小时%d分%d秒", time/3600, time%3600/60, time%3600%60)
	}

	return fmt.Sprintf("%d天%d小时%d分%d秒", time/86400, time%86400/3600, time%86400%3600/60, time%86400%3600%60)
}

func (l *LoginInfo) RequireUpdate() {
	if l.send == nil {
		log.Printf("LoginInfo: Waiting for send channel...")
		time.Sleep(10 * time.Second)
	}
	l.send <- SendAction("get_login_info", struct{}{}, "info")
	l.send <- SendAction("get_friend_list", struct{}{}, "friendList")
	l.send <- SendAction("get_group_list", struct{}{}, "groupList")
}

func SchedulerRequireUpdate(l *LoginInfo) {
	l.RequireUpdate()

	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
	durationUntilMidnight := time.Until(nextMidnight)

	time.AfterFunc(durationUntilMidnight, func() {
		l.RequireUpdate()
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			l.RequireUpdate()
		}
	})
}
