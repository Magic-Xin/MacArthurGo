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
	Send       SendFunc
	NickName   string
	UserId     int64
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
	Name           string `json:"name"`
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

func (l *LoginInfo) ReceiveAll(SendFunc) {}

func (l *LoginInfo) ReceiveMessage(incomingMessage *structs.IncomingMessageStruct, send SendFunc) {
	if incomingMessage.Command == "/info" {
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

		SendMsg(incomingMessage, message, nil, false, false, send)
	} else if incomingMessage.Command == "/help" {
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

		SendMsg(incomingMessage, strings.Join(result, "\n"), nil, false, false, send)
	} else if incomingMessage.Command == "/info_update" {
		if incomingMessage.SenderID != base.Config.Admin {
			SendMsg(incomingMessage, "该指令仅限管理员使用", nil, false, true, send)
			return
		}

		l.RequireUpdate()
		SendMsg(incomingMessage, "信息更新请求已发送", nil, false, false, send)
	}
}

func (l *LoginInfo) ReceiveEcho(feedbackStruct *structs.FeedbackStruct, send SendFunc) {
	if feedbackStruct.Status != "ok" {
		return
	}

	sendStruct := structs.IncomingMessageStruct{
		MessageScene: "private",
		SenderID:     base.Config.Admin,
	}

	data := feedbackStruct.Data

	if data.Nickname != "" && data.Uin != 0 {
		l.NickName, l.UserId = data.Nickname, data.Uin
		log.Printf("Get account nickname: %s, id: %d", l.NickName, l.UserId)
		if !l.IsOnline {
			SendMsg(&sendStruct, "MacArthurGo 已上线", nil, false, false, send)
			l.IsOnline = true
		}
	} else if len(data.Friends) > 0 {
		l.FriendList = make([]Friend, len(data.Friends))
		for i, friend := range data.Friends {
			l.FriendList[i] = Friend{
				UserId:   friend.UserId,
				Nickname: friend.Nickname,
				Remark:   friend.Remark,
			}
		}
		log.Printf("Get friend list count: %d", len(l.FriendList))
		SendMsg(&sendStruct, fmt.Sprintf("好友列表加载成功，好友数量: %d", len(l.FriendList)), nil, false, false, send)
		l.UpdateTime[1] = time.Now().Unix()
	} else if len(data.Groups) > 0 {
		l.GroupList = make([]Group, len(data.Groups))
		for i, group := range data.Groups {
			l.GroupList[i] = Group{
				GroupId:        group.GroupId,
				Name:           group.Name,
				MemberCount:    group.MemberCount,
				MaxMemberCount: group.MaxMemberCount,
			}
		}
		log.Printf("Get group list count: %d", len(l.GroupList))
		SendMsg(&sendStruct, fmt.Sprintf("群组列表加载成功，群组数量: %d", len(l.GroupList)), nil, false, false, send)
		l.UpdateTime[2] = time.Now().Unix()
	}
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
	for l.Send == nil {
		log.Printf("LoginInfo: Waiting for Send channel...")
		time.Sleep(10 * time.Second)
	}

	SendAction("get_login_info", map[string]any{}, l.Send)
	SendAction("get_friend_list", map[string]any{"no_cache": true}, l.Send)
	SendAction("get_group_list", map[string]any{"no_cache": true}, l.Send)
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
