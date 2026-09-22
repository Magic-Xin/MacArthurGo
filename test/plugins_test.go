package test

import (
	"MacArthurGo/base"
	"MacArthurGo/internal/urlmatch"
	botplugins "MacArthurGo/plugins"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestDailyWaifu_CacheMissLoadsOnceAndRepliesToPendingCommands(t *testing.T) {
	oldArgs := append([]string(nil), base.Config.Plugins.Waifu.Args...)
	base.Config.Plugins.Waifu.Args = []string{"/w"}
	t.Cleanup(func() { base.Config.Plugins.Waifu.Args = oldArgs })

	daily := &botplugins.DailyWaifu{}
	send := make(chan []byte, 8)
	first := &structs.MessageStruct{
		MessageType: "group",
		MessageId:   11,
		GroupId:     100,
		UserId:      1,
		Command:     "/w",
	}
	second := &structs.MessageStruct{
		MessageType: "group",
		MessageId:   12,
		GroupId:     100,
		UserId:      2,
		Command:     "/w",
	}

	daily.ReceiveMessage(first, send)
	daily.ReceiveMessage(second, send)
	if got := len(send); got != 1 {
		t.Fatalf("outbound requests after two cache misses = %d, want 1", got)
	}

	var action struct {
		Action string `json:"action"`
		Params struct {
			GroupID int64 `json:"group_id"`
		} `json:"params"`
		Echo string `json:"echo"`
	}
	if err := json.Unmarshal(<-send, &action); err != nil {
		t.Fatalf("decode member-list action: %v", err)
	}
	if action.Action != "get_group_member_list" || action.Params.GroupID != 100 || action.Echo != "groupMemberList:100" {
		t.Fatalf("member-list action = %#v", action)
	}

	echo := groupMemberListEcho(t, 100, 1, 2)
	daily.ReceiveEcho(echo, send)
	if got := len(send); got != 2 {
		t.Fatalf("pending replies = %d, want 2", got)
	}
	for range 2 {
		response := string(<-send)
		if !strings.Contains(response, `"action":"send_msg"`) || !strings.Contains(response, "你今天的老婆是") {
			t.Fatalf("unexpected pending response: %s", response)
		}
		if strings.Contains(response, "获取群组缓存失败") {
			t.Fatalf("cache race leaked to user: %s", response)
		}
	}

	third := *first
	third.MessageId = 13
	daily.ReceiveMessage(&third, send)
	response := string(<-send)
	if !strings.Contains(response, `"action":"send_msg"`) || strings.Contains(response, "get_group_member_list") {
		t.Fatalf("cached response = %s", response)
	}
}

func TestDailyWaifu_FailedLazyLoadRepliesWithRetryableError(t *testing.T) {
	oldArgs := append([]string(nil), base.Config.Plugins.Waifu.Args...)
	base.Config.Plugins.Waifu.Args = []string{"/w"}
	t.Cleanup(func() { base.Config.Plugins.Waifu.Args = oldArgs })

	daily := &botplugins.DailyWaifu{}
	send := make(chan []byte, 2)
	request := &structs.MessageStruct{
		MessageType: "group",
		MessageId:   21,
		GroupId:     200,
		UserId:      1,
		Command:     "/w",
	}
	daily.ReceiveMessage(request, send)
	<-send

	daily.ReceiveEcho(&structs.EchoMessageStruct{
		Echo:   "groupMemberList:200",
		Status: "failed",
	}, send)
	response := string(<-send)
	if !strings.Contains(response, "获取群成员信息失败，请稍后重试") {
		t.Fatalf("failure response = %s", response)
	}
}

func TestDailyWaifu_GroupListTriggersMemberRefresh(t *testing.T) {
	oldEnabled := base.Config.Plugins.Waifu.Enable
	base.Config.Plugins.Waifu.Enable = true
	essentials.Info = essentials.LoginInfo{}
	t.Cleanup(func() {
		base.Config.Plugins.Waifu.Enable = oldEnabled
		essentials.Info = essentials.LoginInfo{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	send := make(chan []byte, 2)
	daily := &botplugins.DailyWaifu{}
	daily.Start(ctx, send)

	var groupListArray structs.EchoMessageArrayStruct
	if err := json.Unmarshal([]byte(`{"status":"ok","echo":"groupList","data":[{"group_id":300,"group_name":"test"}]}`), &groupListArray); err != nil {
		t.Fatalf("decode group list: %v", err)
	}
	groupList := &structs.EchoMessageStruct{
		DataArray: groupListArray.Data,
		Status:    groupListArray.Status,
		Echo:      groupListArray.Echo,
	}
	essentials.Info.ReceiveEcho(groupList, send)

	var action struct {
		Action string `json:"action"`
		Params struct {
			GroupID int64 `json:"group_id"`
		} `json:"params"`
		Echo string `json:"echo"`
	}
	if err := json.Unmarshal(receiveWithin(t, send), &action); err != nil {
		t.Fatalf("decode member refresh action: %v", err)
	}
	if action.Action != "get_group_member_list" || action.Params.GroupID != 300 || action.Echo != "groupMemberList:300" {
		t.Fatalf("member refresh action = %#v", action)
	}
}

func TestBiliURLMatchers(t *testing.T) {
	tests := []struct {
		name  string
		match func(string) (string, bool)
		input string
		want  string
	}{
		{name: "desktop video", match: urlmatch.BiliVideoID, input: "https://www.bilibili.com/video/BV1abc", want: "BV1abc"},
		{name: "mobile video", match: urlmatch.BiliVideoID, input: "https://m.bilibili.com/video/av123", want: "av123"},
		{name: "live room", match: urlmatch.BiliLiveRoomID, input: "https://live.bilibili.com/12345", want: "12345"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := test.match(test.input)
			if !ok || got != test.want {
				t.Fatalf("match = %q, %v; want %q, true", got, ok, test.want)
			}
		})
	}
}

func TestMusicURLMatchers(t *testing.T) {
	tests := []struct {
		name  string
		match func(string) (string, bool)
		input string
		want  string
	}{
		{name: "netease mobile", match: urlmatch.NeteaseMobileSongID, input: "https://y.music.163.com/m/song/12345", want: "12345"},
		{name: "message id", match: urlmatch.MusicQueryID, input: "https://music.163.com/song?id=67890&foo=bar", want: "67890"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := test.match(test.input)
			if !ok || got != test.want {
				t.Fatalf("match = %q, %v; want %q, true", got, ok, test.want)
			}
		})
	}
}

func groupMemberListEcho(t *testing.T, groupID int64, userIDs ...int64) *structs.EchoMessageStruct {
	t.Helper()

	members := make([]map[string]any, 0, len(userIDs))
	for _, userID := range userIDs {
		members = append(members, map[string]any{
			"group_id": groupID,
			"user_id":  userID,
			"nickname": "member",
			"card":     "",
		})
	}
	payload, err := json.Marshal(map[string]any{
		"status": "ok",
		"echo":   "groupMemberList:" + strconv.FormatInt(groupID, 10),
		"data":   members,
	})
	if err != nil {
		t.Fatalf("encode echo: %v", err)
	}
	var array structs.EchoMessageArrayStruct
	if err := json.Unmarshal(payload, &array); err != nil {
		t.Fatalf("decode echo: %v", err)
	}
	return &structs.EchoMessageStruct{
		DataArray: array.Data,
		Echo:      array.Echo,
		Status:    array.Status,
	}
}
