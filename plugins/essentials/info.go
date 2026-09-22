package essentials

import (
	"MacArthurGo/base"
	"MacArthurGo/structs"
	"context"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LoginInfo struct {
	mu         sync.RWMutex
	send       chan<- []byte
	ctx        context.Context
	groupReady chan struct{}
	readyOnce  sync.Once
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

func registerInfo() error {
	Info = LoginInfo{}
	Info.ensureState()
	plugin := &Plugin{
		Name:    "info",
		Enabled: true,
		Args:    []string{"/info", "/help"},
		Handler: &Info,
	}
	return Register(plugin)
}

func (l *LoginInfo) Start(ctx context.Context, send chan<- []byte) {
	l.mu.Lock()
	l.ensureStateLocked()
	l.ctx = ctx
	l.send = send
	l.mu.Unlock()
	go SchedulerRequireUpdate(ctx, l)
}

func (l *LoginInfo) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- []byte) {
	switch messageStruct.Command {
	case "/info":
		friendCount, groupCount := l.Counts()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		message := "MacArthurGo 运行信息\n\n"

		message += "分支: " + base.Branch + "\n" + "版本: " + base.Version + "\n" + "编译时间: " + base.BuildTime + "\n"
		message += "已运行时间: " + l.timeToString(time.Now().Unix()-base.Config.StartTime) + "\n\n"

		message += "已添加好友数量: " + strconv.Itoa(friendCount) + "\n"
		message += "已加入群组数量: " + strconv.Itoa(groupCount) + "\n\n"

		message += "已加载插件: " + strconv.Itoa(PluginCount()) + " 个\n"
		message += "Goroutine 数量: " + strconv.Itoa(runtime.NumGoroutine()) + "\n\n"

		message += "内存使用情况:\n"
		message += "Alloc = " + strconv.FormatUint(mem.Alloc/1024/1024, 10) + " MB\n"
		message += "Sys = " + strconv.FormatUint(mem.Sys/1024/1024, 10) + " MB\n"
		message += "HeapAlloc = " + strconv.FormatUint(mem.HeapAlloc/1024/1024, 10) + " MB\n"

		send <- SendMsg(messageStruct, message, nil, false, false, "")
	case "/help":
		result := []string{"插件\t\t\t\t触发指令"}
		for _, p := range Plugins() {
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
	case "/info_update":
		if messageStruct.UserId != base.Config.Admin {
			send <- SendMsg(messageStruct, "该指令仅限管理员使用", nil, false, true, "")
			return
		}

		l.RequireUpdate()
		send <- SendMsg(messageStruct, "信息更新请求已发送", nil, false, false, "")
	}
}

func (l *LoginInfo) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, send chan<- []byte) {
	if echoMessageStruct.Status != "ok" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensureStateLocked()

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
		l.FriendList = make([]Friend, len(data))
		for i, item := range data {
			l.FriendList[i] = Friend{
				UserId:   item.UserId,
				Nickname: item.Nickname,
				Remark:   item.Remark,
			}
		}

		log.Printf("Get friend list count: %d", len(l.FriendList))
		//send <- SendMsg(&sendStruct, fmt.Sprintf("好友列表加载成功，好友数量: %d", friendCount), nil, false, false, "")
		l.UpdateTime[1] = time.Now().Unix()
	case "groupList":
		data := echoMessageStruct.DataArray
		l.GroupList = make([]Group, len(data))
		for i, item := range data {
			l.GroupList[i] = Group{
				GroupId:        item.GroupId,
				GroupName:      item.GroupName,
				MemberCount:    item.MemberCount,
				MaxMemberCount: item.MaxMemberCount,
			}
		}

		log.Printf("Get group list count: %d", len(l.GroupList))
		//send <- SendMsg(&sendStruct, fmt.Sprintf("群组列表加载成功，群组数量: %d", groupCount), nil, false, false, "")
		l.UpdateTime[2] = time.Now().Unix()
		l.readyOnce.Do(func() { close(l.groupReady) })
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
	l.mu.RLock()
	send, ctx := l.send, l.ctx
	l.mu.RUnlock()
	if send == nil {
		log.Printf("LoginInfo: send channel is not ready")
		return
	}
	for _, action := range [][]byte{
		SendAction("get_login_info", struct{}{}, "info"),
		SendAction("get_friend_list", struct{}{}, "friendList"),
		SendAction("get_group_list", struct{}{}, "groupList"),
	} {
		select {
		case send <- action:
		case <-ctx.Done():
			return
		}
	}
}

func SchedulerRequireUpdate(ctx context.Context, l *LoginInfo) {
	l.RequireUpdate()

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	for {
		timer := time.NewTimer(untilNextMidnight(location))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			l.RequireUpdate()
		}
	}
}

func untilNextMidnight(location *time.Location) time.Duration {
	now := time.Now().In(location)
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
	return time.Until(next)
}

func (l *LoginInfo) Counts() (friends, groups int) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.FriendList), len(l.GroupList)
}

func (l *LoginInfo) Groups() []Group {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]Group(nil), l.GroupList...)
}

func (l *LoginInfo) Account() (userID, nickname string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.UserId, l.NickName
}

func (l *LoginInfo) WaitForGroups(ctx context.Context) bool {
	l.mu.Lock()
	l.ensureStateLocked()
	ready := l.groupReady
	l.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-ready:
		return true
	}
}

func (l *LoginInfo) ensureState() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensureStateLocked()
}

func (l *LoginInfo) ensureStateLocked() {
	if l.groupReady == nil {
		l.groupReady = make(chan struct{})
	}
	if len(l.UpdateTime) < 3 {
		updateTime := make([]int64, 3)
		copy(updateTime, l.UpdateTime)
		l.UpdateTime = updateTime
	}
}
