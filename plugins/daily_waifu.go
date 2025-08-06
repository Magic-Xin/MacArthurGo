package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

type Waifu struct {
	UserId   int64
	NickName string
	Card     string
}

type DailyWaifu struct {
	send  essentials.SendFunc
	Cache sync.Map
}

func init() {
	plugin := &essentials.Plugin{
		Name:      "每日老婆",
		Enabled:   base.Config.Plugins.Waifu.Enable,
		Args:      base.Config.Plugins.Waifu.Args,
		Interface: &DailyWaifu{},
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
	go ScheduleRequireUpdate(plugin.Interface.(*DailyWaifu))
}

func (d *DailyWaifu) ReceiveAll(send essentials.SendFunc) {
	if d.send == nil && send != nil {
		d.send = send
	}
}

func (d *DailyWaifu) ReceiveMessage(incomingMessageStruct *structs.IncomingMessageStruct, send essentials.SendFunc) {
	if !essentials.CheckArgumentArray(incomingMessageStruct.Command, &base.Config.Plugins.Waifu.Args) {
		return
	}

	for _, msg := range *incomingMessageStruct.CleanMessage {
		if msg.Type == "text" && msg.Data["text"].(string) == "update" {
			if incomingMessageStruct.SenderID != base.Config.Admin {
				essentials.SendMsg(incomingMessageStruct, "该指令仅限管理员使用", nil, false, true, send)
			} else {
				d.RequireUpdate()
				essentials.SendMsg(incomingMessageStruct, "今日老婆信息更新中...", nil, false, true, send)
			}
			return
		}
	}

	if incomingMessageStruct.MessageScene != "group" {
		essentials.SendMsg(incomingMessageStruct, "该指令仅限群聊使用", nil, false, true, send)
		return
	}

	const avatarApi = "https://q1.qlogo.cn/g?b=qq&s=100&nk="

	groupCache, ok := d.Cache.Load(incomingMessageStruct.Group.GroupID)
	if !ok {
		essentials.SendMsg(incomingMessageStruct, "获取群组缓存失败", nil, false, true, send)
		return
	}

	groupCacheMap := groupCache.(map[int64]Waifu)
	userId := incomingMessageStruct.SenderID
	if _, ok := groupCacheMap[userId]; ok {
		wife := groupCacheMap[userId]
		var msg []structs.MessageSegment

		if wife.Card != "" {
			msg = append(msg, *structs.Text(fmt.Sprintf("你今天的老婆是: %s(%s)\n%d", wife.Card, wife.NickName, wife.UserId)))
		} else {
			msg = append(msg, *structs.Text(fmt.Sprintf("你今天的老婆是: %s\n%d", wife.NickName, wife.UserId)))
		}
		msg = append(msg, *structs.Image(fmt.Sprintf("%s%d", avatarApi, wife.UserId)))
		essentials.SendMsg(incomingMessageStruct, "", &msg, false, true, send)
		return
	}

	essentials.SendMsg(incomingMessageStruct, "获取老婆失败, 你今天没老婆了", nil, false, true, send)
}

func (d *DailyWaifu) ReceiveEcho(feedbackStruct *structs.FeedbackStruct, _ essentials.SendFunc) {
	if feedbackStruct.Status != "ok" {
		return
	}

	members := feedbackStruct.Data.Members
	if len(members) == 0 {
		return
	}

	var waifus []Waifu
	groupId := members[0].GroupId
	for _, data := range members {
		waifus = append(waifus, Waifu{
			UserId:   data.UserId,
			NickName: data.Nickname,
			Card:     data.Card,
		})
	}

	today := time.Now().In(time.Local)
	seedDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	src := rand.NewSource(seedDate.UnixNano())
	r := rand.New(src)

	userIDs := make([]int64, len(members))
	userMap := make(map[int64]Waifu)
	for i, u := range members {
		userIDs[i] = u.UserId
		userMap[u.UserId] = Waifu{
			UserId:   u.UserId,
			NickName: u.Nickname,
			Card:     u.Card,
		}
	}

	r.Shuffle(len(userIDs), func(i, j int) {
		userIDs[i], userIDs[j] = userIDs[j], userIDs[i]
	})

	pairings := make(map[int64]Waifu, len(waifus))

	for i := 0; i < len(userIDs); i += 2 {

		if i+1 >= len(userIDs) {
			if len(userIDs) > 1 {
				randomPairIdx := r.Intn(i)
				userA := userIDs[i]
				userB := userIDs[randomPairIdx]

				userBInfo := userMap[userB]
				pairings[userA] = Waifu{
					UserId:   userB,
					NickName: userBInfo.NickName,
					Card:     userBInfo.Card,
				}
			}
			break
		}

		userA := userIDs[i]
		userB := userIDs[i+1]

		userBInfo := userMap[userB]
		pairings[userA] = Waifu{
			UserId:   userB,
			NickName: userBInfo.NickName,
			Card:     userBInfo.Card,
		}

		userAInfo := userMap[userA]
		pairings[userB] = Waifu{
			UserId:   userA,
			NickName: userAInfo.NickName,
			Card:     userAInfo.Card,
		}
	}

	d.Cache.Store(groupId, pairings)
}

func (d *DailyWaifu) RequireUpdate() {
	for d.send == nil {
		log.Printf("DailyWaifu: Waiting for send channel...")
		time.Sleep(10 * time.Second)
	}

	for _, group := range essentials.Info.GroupList {
		d.Cache.Clear()
		essentials.SendAction("get_group_member_list", map[string]any{"group_id": group.GroupId}, d.send)
		log.Printf("DailyWaifu: Updating group %d member list", group.GroupId)
	}
}

func ScheduleRequireUpdate(d *DailyWaifu) {
	if !base.Config.Plugins.Waifu.Enable {
		return
	}

	for essentials.Info.UpdateTime[2] == 0 {
		log.Printf("DailyWaifu: Waiting for group list...")
		time.Sleep(10 * time.Second)
	}

	d.RequireUpdate()

	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
	durationUntilMidnight := time.Until(nextMidnight)

	time.AfterFunc(durationUntilMidnight, func() {
		for time.Now().Unix()-essentials.Info.UpdateTime[2] > 86400 {
			log.Printf("DailyWaifu: Waiting for new group list...")
			time.Sleep(10 * time.Second)
		}
		d.RequireUpdate()
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			for time.Now().Unix()-essentials.Info.UpdateTime[2] > 86400 {
				log.Printf("DailyWaifu: Waiting for new group list...")
				time.Sleep(10 * time.Second)
			}
			d.RequireUpdate()
		}
	})
}
