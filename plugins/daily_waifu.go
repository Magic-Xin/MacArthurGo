package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	groupMemberListEchoPrefix = "groupMemberList:"
	groupMemberRequestTTL     = 30 * time.Second
)

type Waifu struct {
	UserId   int64
	NickName string
	Card     string
}

type DailyWaifu struct {
	send chan<- []byte
	ctx  context.Context

	Cache sync.Map
	mu    sync.Mutex

	pending  map[int64][]structs.MessageStruct
	inFlight map[int64]time.Time
}

func registerDailyWaifu() error {
	plugin := &essentials.Plugin{
		Name:    "每日老婆",
		Enabled: base.Config.Plugins.Waifu.Enable,
		Args:    base.Config.Plugins.Waifu.Args,
		Handler: &DailyWaifu{},
	}
	return essentials.Register(plugin)
}

func (d *DailyWaifu) Start(ctx context.Context, send chan<- []byte) {
	d.mu.Lock()
	d.ctx, d.send = ctx, send
	d.ensureRequestStateLocked()
	d.mu.Unlock()
	go ScheduleRequireUpdate(ctx, d)
}

func (d *DailyWaifu) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- []byte) {
	if !essentials.CheckArgumentArray(messageStruct.Command, base.Config.Plugins.Waifu.Args) {
		return
	}

	if messageStruct.GroupId == 0 {
		for _, msg := range messageStruct.CleanMessage {
			text, ok := msg.Data["text"].(string)
			if msg.Type == "text" && ok && text == "update" {
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

	groupCache, ok := d.Cache.Load(messageStruct.GroupId)
	if ok {
		groupCacheMap, valid := groupCache.(map[int64]Waifu)
		if valid {
			d.sendWaifuReply(messageStruct, groupCacheMap, send)
			return
		}
		log.Printf("DailyWaifu: invalid cache value for group %d", messageStruct.GroupId)
		d.Cache.Delete(messageStruct.GroupId)
	}

	// Startup, reconnect, and scheduled refresh can all leave a short window in
	// which the group cache has not arrived yet. Queue the original command and
	// lazily fetch that group instead of turning this normal race into an error.
	d.enqueuePending(messageStruct)
	d.requestGroupMembers(messageStruct.GroupId, send)
}

func (d *DailyWaifu) sendWaifuReply(messageStruct *structs.MessageStruct, groupCache map[int64]Waifu, send chan<- []byte) {
	const avatarAPI = "https://q1.qlogo.cn/g?b=qq&s=100&nk="

	wife, ok := groupCache[messageStruct.UserId]
	if !ok {
		send <- essentials.SendMsg(messageStruct, "获取老婆失败, 你今天没老婆了", nil, false, true, "")
		return
	}

	var msg []cqcode.ArrayMessage
	if wife.Card != "" {
		msg = append(msg, *cqcode.Text(fmt.Sprintf("你今天的老婆是: %s(%s)\n%d", wife.Card, wife.NickName, wife.UserId)))
	} else {
		msg = append(msg, *cqcode.Text(fmt.Sprintf("你今天的老婆是: %s\n%d", wife.NickName, wife.UserId)))
	}
	msg = append(msg, *cqcode.Image(fmt.Sprintf("%s%d", avatarAPI, wife.UserId)))
	send <- essentials.SendMsg(messageStruct, "", msg, false, true, "")
}

func (d *DailyWaifu) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, send chan<- []byte) {
	if echoMessageStruct == nil {
		return
	}

	groupID, recognized := groupIDFromMemberEcho(echoMessageStruct)
	if !recognized {
		return
	}
	if echoMessageStruct.Status != "ok" || len(echoMessageStruct.DataArray) == 0 {
		d.failGroupRequest(groupID, send)
		return
	}
	if groupID == 0 {
		groupID = echoMessageStruct.DataArray[0].GroupId
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

	pairings := make(map[int64]Waifu, len(userIDs))

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

	d.Cache.Store(groupID, pairings)
	for _, pending := range d.finishGroupRequest(groupID) {
		request := pending
		d.sendWaifuReply(&request, pairings, send)
	}
}

func (d *DailyWaifu) enqueuePending(messageStruct *structs.MessageStruct) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ensureRequestStateLocked()
	d.pending[messageStruct.GroupId] = append(d.pending[messageStruct.GroupId], *messageStruct)
}

func (d *DailyWaifu) requestGroupMembers(groupID int64, send chan<- []byte) {
	if groupID == 0 || send == nil {
		return
	}

	now := time.Now()
	d.mu.Lock()
	d.ensureRequestStateLocked()
	if requestedAt, ok := d.inFlight[groupID]; ok && now.Sub(requestedAt) < groupMemberRequestTTL {
		d.mu.Unlock()
		return
	}
	d.inFlight[groupID] = now
	ctx := d.ctx
	d.mu.Unlock()

	action := essentials.SendAction("get_group_member_list",
		struct {
			GroupId int64 `json:"group_id"`
		}{GroupId: groupID}, groupMemberEcho(groupID))

	var done <-chan struct{}
	if ctx != nil {
		done = ctx.Done()
	}
	select {
	case send <- action:
	case <-done:
		d.mu.Lock()
		delete(d.inFlight, groupID)
		d.mu.Unlock()
	}
}

func (d *DailyWaifu) ensureRequestStateLocked() {
	if d.pending == nil {
		d.pending = make(map[int64][]structs.MessageStruct)
	}
	if d.inFlight == nil {
		d.inFlight = make(map[int64]time.Time)
	}
}

func (d *DailyWaifu) finishGroupRequest(groupID int64) []structs.MessageStruct {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ensureRequestStateLocked()
	pending := d.pending[groupID]
	delete(d.pending, groupID)
	delete(d.inFlight, groupID)
	return pending
}

func (d *DailyWaifu) failGroupRequest(groupID int64, send chan<- []byte) {
	if groupID == 0 {
		log.Printf("DailyWaifu: group member request failed without a group id")
		return
	}
	for _, pending := range d.finishGroupRequest(groupID) {
		request := pending
		send <- essentials.SendMsg(&request, "获取群成员信息失败，请稍后重试", nil, false, true, "")
	}
}

func groupMemberEcho(groupID int64) string {
	return groupMemberListEchoPrefix + strconv.FormatInt(groupID, 10)
}

func groupIDFromMemberEcho(echoMessageStruct *structs.EchoMessageStruct) (int64, bool) {
	echo := echoMessageStruct.Echo
	if strings.HasPrefix(echo, groupMemberListEchoPrefix) {
		groupID, err := strconv.ParseInt(strings.TrimPrefix(echo, groupMemberListEchoPrefix), 10, 64)
		return groupID, err == nil && groupID != 0
	}
	if echo != "groupMemberList" {
		return 0, false
	}
	if len(echoMessageStruct.DataArray) == 0 {
		return 0, true
	}
	return echoMessageStruct.DataArray[0].GroupId, true
}

func (d *DailyWaifu) RequireUpdate() {
	d.mu.Lock()
	send := d.send
	d.mu.Unlock()
	if send == nil {
		log.Printf("DailyWaifu: send channel is not ready")
		return
	}

	d.Cache.Clear()
	for _, group := range essentials.Info.Groups() {
		d.requestGroupMembers(group.GroupId, send)
	}
}

func ScheduleRequireUpdate(ctx context.Context, d *DailyWaifu) {
	if !base.Config.Plugins.Waifu.Enable {
		return
	}

	if !essentials.Info.WaitForGroups(ctx) {
		return
	}
	d.RequireUpdate()

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	for {
		now := time.Now().In(location)
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			d.RequireUpdate()
		}
	}
}
