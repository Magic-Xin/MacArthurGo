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
	send  chan<- *[]byte
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

func (d *DailyWaifu) ReceiveAll(send chan<- *[]byte) {
	if d.send == nil && send != nil {
		d.send = send
	}
}

func (d *DailyWaifu) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	if !essentials.CheckArgumentArray(messageStruct.Command, &base.Config.Plugins.Waifu.Args) {
		return
	}

	if messageStruct.GroupId == 0 {
		for _, msg := range *messageStruct.CleanMessage {
			if msg.Type == "text" && msg.Data["text"].(string) == "update" {
				if messageStruct.UserId != base.Config.Admin {
					send <- essentials.SendMsg(messageStruct, "该指令仅限管理员使用", nil, false, true, "")
				} else {
					d.RequireUpdate()
					send <- essentials.SendMsg(messageStruct, "今日老婆信息更新中...", nil, false, true, "")
				}
			}
		}
		return
	}

	const avatarApi = "https://q1.qlogo.cn/g?b=qq&s=100&nk="

	groupCache, ok := d.Cache.Load(messageStruct.GroupId)
	if !ok {
		send <- essentials.SendMsg(messageStruct, "获取群组缓存失败", nil, false, true, "")
		return
	}

	groupCacheMap := groupCache.(map[int64]Waifu)
	userId := messageStruct.UserId
	if _, ok := groupCacheMap[userId]; ok {
		wife := groupCacheMap[userId]
		var msg []structs.ArrayMessage

		if wife.Card != "" {
			msg = append(msg, *structs.Text(fmt.Sprintf("你今天的老婆是: %s(%s)\n%d", wife.Card, wife.NickName, wife.UserId)))
		} else {
			msg = append(msg, *structs.Text(fmt.Sprintf("你今天的老婆是: %s\n%d", wife.NickName, wife.UserId)))
		}
		msg = append(msg, *structs.Image(fmt.Sprintf("%s%d", avatarApi, wife.UserId)))
		send <- essentials.SendMsg(messageStruct, "", &msg, false, true, "")
		return
	}

	send <- essentials.SendMsg(messageStruct, "获取老婆失败, 你今天没老婆了", nil, false, true, "")
}

func (d *DailyWaifu) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, _ chan<- *[]byte) {
	if echoMessageStruct.Status != "ok" || echoMessageStruct.Echo != "groupMemberList" {
		return
	}
	if len(echoMessageStruct.DataArray) == 0 {
		return
	}

	var waifus []Waifu
	groupId := echoMessageStruct.DataArray[0].GroupId
	for _, data := range echoMessageStruct.DataArray {
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

	userIDs := make([]int64, len(echoMessageStruct.DataArray))
	userMap := make(map[int64]Waifu)
	for i, u := range echoMessageStruct.DataArray {
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
		d.send <- essentials.SendAction("get_group_member_list",
			struct {
				GroupId int64 `json:"group_id"`
			}{GroupId: group.GroupId}, "groupMemberList")
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
