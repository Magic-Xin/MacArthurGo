package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"fmt"
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
	if messageStruct.GroupId == 0 {
		return
	}
	if !essentials.CheckArgumentArray(messageStruct.Command, &base.Config.Plugins.Waifu.Args) {
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
		var msg []cqcode.ArrayMessage

		if wife.Card != "" {
			msg = append(msg, *cqcode.Text(fmt.Sprintf("你今天的老婆是: %s(%s)\n%d", wife.Card, wife.NickName, wife.UserId)))
		} else {
			msg = append(msg, *cqcode.Text(fmt.Sprintf("你今天的老婆是: %s\n%d", wife.NickName, wife.UserId)))
		}
		msg = append(msg, *cqcode.Image(fmt.Sprintf("%s%d", avatarApi, wife.UserId)))
		send <- essentials.SendMsg(messageStruct, "", &msg, false, true, "")
		return
	}

	send <- essentials.SendMsg(messageStruct, "获取老婆失败", nil, false, true, "")
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

	src := rand.NewSource(time.Now().UnixNano())
	r := rand.New(src)
	r.Shuffle(len(waifus), func(i, j int) {
		waifus[i], waifus[j] = waifus[j], waifus[i]
	})

	pairings := make(map[int64]Waifu, len(waifus))
	visited := make(map[int64]bool)

	for _, u := range echoMessageStruct.DataArray {
		if visited[u.UserId] {
			continue
		}

		targetIdx := -1
		for i := 0; i < len(waifus); i++ {
			idx := (r.Int() + i) % len(waifus)
			target := waifus[idx]
			if !visited[target.UserId] && target.UserId != u.UserId {
				targetIdx = idx
				break
			}
		}

		if targetIdx == -1 {
			continue
		}

		target := waifus[targetIdx]
		pairings[u.UserId] = target
		visited[u.UserId] = true
		visited[target.UserId] = true

		pairings[target.UserId] = Waifu{
			UserId:   u.UserId,
			NickName: u.Nickname,
			Card:     u.Card,
		}
	}

	d.Cache.Store(groupId, pairings)
}

func (d *DailyWaifu) RequireUpdate() {
	for d.send == nil {
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
	d.RequireUpdate()

	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
	durationUntilMidnight := time.Until(nextMidnight)

	time.AfterFunc(durationUntilMidnight, func() {
		d.RequireUpdate()
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			d.RequireUpdate()
		}
	})
}
