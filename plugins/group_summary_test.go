package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type groupSummaryAction struct {
	Action string `json:"action"`
	Echo   string `json:"echo"`
	Params struct {
		GroupID      int64                 `json:"group_id"`
		MessageSeq   string                `json:"message_seq"`
		Count        int                   `json:"count"`
		ReverseOrder bool                  `json:"reverse_order"`
		DisableURL   *bool                 `json:"disable_get_url"`
		Message      []cqcode.ArrayMessage `json:"message"`
		Messages     []structs.ForwardNode `json:"messages"`
	} `json:"params"`
}

func summaryAction(t *testing.T, send <-chan []byte) groupSummaryAction {
	t.Helper()
	select {
	case payload := <-send:
		var action groupSummaryAction
		if err := json.Unmarshal(payload, &action); err != nil {
			t.Fatal(err)
		}
		return action
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for group summary action")
		return groupSummaryAction{}
	}
}

func summaryReplyText(action groupSummaryAction) string {
	for _, segment := range action.Params.Message {
		if segment.Type == "text" {
			if text, ok := segment.Data["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

func expectSummaryStarted(t *testing.T, send <-chan []byte) {
	t.Helper()
	action := summaryAction(t, send)
	if action.Action != "send_msg" || len(action.Params.Message) < 2 || action.Params.Message[0].Type != "reply" || action.Params.Message[0].Data["id"] != "321" || !strings.Contains(summaryReplyText(action), "正在读取群聊记录") {
		t.Fatalf("start reply = %#v", action)
	}
}

func summaryForwardContents(t *testing.T, action groupSummaryAction) []cqcode.ArrayMessage {
	t.Helper()
	if action.Action != "send_group_forward_msg" || len(action.Params.Messages) != 3 {
		t.Fatalf("forward action = %#v", action)
	}
	contents := make([]cqcode.ArrayMessage, 3)
	for i := range action.Params.Messages {
		node := action.Params.Messages[i]
		if node.Type != "node" || len(node.Data.Content) != 1 {
			t.Fatalf("forward node %d = %#v", i, node)
		}
		contents[i] = node.Data.Content[0]
	}
	return contents
}

func summaryEvent(groupID, userID int64, content string) *structs.MessageStruct {
	message := &structs.MessageStruct{MessageType: "group", GroupId: groupID, UserId: userID,
		Message: []cqcode.ArrayMessage{*cqcode.Text(content)}}
	message.Sender.Nickname = "昵称"
	message.Sender.Card = "群名片"
	return message
}

func summaryCommand(groupID, userID int64, arg string) *structs.MessageStruct {
	message := summaryEvent(groupID, userID, "/sum "+arg)
	message.MessageId = 321
	message.Command = "/sum"
	message.CleanMessage = []cqcode.ArrayMessage{*cqcode.Text(arg)}
	return message
}

func historyMessage(groupID, id, at int64, content string) structs.MessageStruct {
	message := *summaryEvent(groupID, 2, content)
	message.MessageId = id
	message.Time = at
	return message
}

func replyHistory(t *testing.T, plugin *GroupSummary, action groupSummaryAction, messages []structs.MessageStruct) {
	t.Helper()
	if action.Action != "get_group_msg_history" || action.Params.Count != groupSummaryPageSize || !action.Params.ReverseOrder || action.Params.DisableURL == nil || !*action.Params.DisableURL {
		t.Fatalf("history request = %#v", action)
	}
	data, err := json.Marshal(struct {
		Messages []structs.MessageStruct `json:"messages"`
	}{messages})
	if err != nil {
		t.Fatal(err)
	}
	plugin.ReceiveEcho(&structs.EchoMessageStruct{Echo: action.Echo, Status: "ok", RawData: data}, nil)
}

func newTestGroupSummary(t *testing.T, dir string, clock *atomic.Int64, captured chan<- []summaryMessage) *GroupSummary {
	t.Helper()
	plugin, err := newGroupSummary(dir, "model", "key", func(_ context.Context, lines []summaryMessage) (summaryResult, error) {
		captured <- lines
		return summaryResult{Summary: "**话题：测试**", Roast: "**大家**讨论得很认真。转头又跑题了！"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	plugin.now = func() time.Time { return time.Unix(clock.Load(), 0) }
	return plugin
}

func waitSummaryIdle(t *testing.T, plugin *GroupSummary, groupID int64) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		plugin.mu.Lock()
		idle := plugin.inFlight[groupID] == nil
		plugin.mu.Unlock()
		if idle {
			return
		}
		select {
		case <-deadline:
			t.Fatal("summary did not finish")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestParseSummarySelection(t *testing.T) {
	for _, test := range []struct {
		arg   string
		hours int
		count int
		valid bool
	}{
		{"1h", 1, 0, true}, {"4H", 4, 0, true}, {"25", 0, 25, true}, {"200", 0, 200, true},
		{"", 0, 0, false}, {"0h", 0, 0, false}, {"5h", 0, 0, false},
		{"24", 0, 0, false}, {"201", 0, 0, false}, {"1.5h", 0, 0, false},
	} {
		t.Run(test.arg, func(t *testing.T) {
			selection, err := parseSummarySelection([]string{test.arg})
			if (err == nil) != test.valid || (test.valid && (selection.hours != test.hours || selection.count != test.count)) {
				t.Fatalf("selection = %#v, err = %v", selection, err)
			}
		})
	}
	if _, err := parseSummarySelection(nil); err == nil {
		t.Fatal("bare /sum should show usage")
	}
}

func TestGroupSummaryTimeWindowAndRoast(t *testing.T) {
	previousConfig := base.Config
	base.Config = &base.Configuration{Admin: 1}
	t.Cleanup(func() { base.Config = previousConfig })
	var clock atomic.Int64
	start := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	clock.Store(start.Unix())
	captured := make(chan []summaryMessage, 2)
	plugin := newTestGroupSummary(t, t.TempDir(), &clock, captured)
	send := make(chan []byte, 10)
	plugin.Start(context.Background(), send)
	defer plugin.Stop()
	plugin.ReceiveMessage(summaryCommand(100, 2, "enable"), send)
	if action := summaryAction(t, send); !strings.Contains(summaryReplyText(action), "仅管理员") {
		t.Fatalf("non-admin enable = %#v", action)
	}
	plugin.ReceiveMessage(summaryCommand(100, 1, "enable"), send)
	if action := summaryAction(t, send); !strings.Contains(summaryReplyText(action), "已开启") {
		t.Fatalf("admin enable = %#v", action)
	}
	select {
	case action := <-send:
		t.Fatalf("unexpected automatic action: %s", action)
	default:
	}
	plugin.ReceiveMessage(summaryCommand(100, 2, "1h"), send)
	expectSummaryStarted(t, send)
	request := summaryAction(t, send)
	old := historyMessage(100, 1, start.Add(-time.Hour-time.Second).Unix(), "太早")
	boundary := historyMessage(100, 2, start.Add(-time.Hour).Unix(), "边界话题")
	boundary.Message = append(boundary.Message,
		cqcode.ArrayMessage{Type: "image", Data: map[string]any{"url": "https://example.com/chat.jpg", "file": "image.jpg"}},
		*cqcode.Text("关键内容"))
	inside := historyMessage(100, 3, start.Add(-time.Minute).Unix(), "近期话题")
	imageOnly := historyMessage(100, 6, start.Add(-2*time.Minute).Unix(), "")
	imageOnly.Message = []cqcode.ArrayMessage{{Type: "image", Data: map[string]any{"url": "https://example.com/only.png"}}}
	command := historyMessage(100, 4, start.Add(-30*time.Second).Unix(), "/sum 1h")
	future := historyMessage(100, 5, start.Add(time.Second).Unix(), "未来话题")
	replyHistory(t, plugin, request, []structs.MessageStruct{old, boundary, imageOnly, inside, command, future})
	lines := <-captured
	if len(lines) != 2 || lines[0].ID != 2 || lines[0].Content != "边界话题 关键内容" || lines[1].ID != 3 || lines[1].Content != "近期话题" {
		t.Fatalf("AI input = %#v", lines)
	}
	action := summaryAction(t, send)
	contents := summaryForwardContents(t, action)
	if contents[0].Type != "text" {
		t.Errorf("header type = %q", contents[0].Type)
	}
	header := contents[0].Data["text"].(string)
	if !strings.Contains(header, "2026-09-23 19:00 - 2026-09-23 19:59") || !strings.Contains(header, "共 2 条消息（北京时间 UTC+8）") {
		t.Errorf("header = %q", header)
	}
	if contents[1].Type != "text" || contents[1].Data["text"] != "*话题：测试*" {
		t.Errorf("summary = %#v", contents[1])
	}
	if contents[2].Type != "text" || contents[2].Data["text"] != "AI 锐评：**大家**讨论得很认真。转头又跑题了！" {
		t.Errorf("roast = %#v", contents[2])
	}
	waitSummaryIdle(t, plugin, 100)
	plugin.ReceiveMessage(summaryCommand(100, 3, "1h"), send)
	if action := summaryAction(t, send); !strings.Contains(summaryReplyText(action), "每小时") {
		t.Fatalf("group cooldown = %#v", action)
	}
}

func TestGroupSummaryCountAndCooldownPersistence(t *testing.T) {
	previousConfig := base.Config
	base.Config = &base.Configuration{Admin: 1}
	t.Cleanup(func() { base.Config = previousConfig })
	var clock atomic.Int64
	start := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	clock.Store(start.Unix())
	dir := t.TempDir()
	captured := make(chan []summaryMessage, 3)
	plugin := newTestGroupSummary(t, dir, &clock, captured)
	send := make(chan []byte, 10)
	plugin.Start(context.Background(), send)
	plugin.ReceiveMessage(summaryCommand(200, 1, "enable"), send)
	summaryAction(t, send)
	plugin.ReceiveMessage(summaryCommand(200, 2, "25"), send)
	expectSummaryStarted(t, send)
	request := summaryAction(t, send)
	messages := make([]structs.MessageStruct, 30)
	for i := range messages {
		messages[i] = historyMessage(200, int64(i+1), start.Add(-time.Duration(30-i)*time.Minute).Unix(), "消息")
	}
	replyHistory(t, plugin, request, messages)
	lines := <-captured
	if len(lines) != 25 || lines[0].ID != 6 || lines[24].ID != 30 {
		t.Fatalf("count selection = %#v", lines)
	}
	summaryAction(t, send)
	waitSummaryIdle(t, plugin, 200)
	if err := plugin.Stop(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "summary.db")); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("database permissions = %v, err = %v", info, err)
	}
	reopened := newTestGroupSummary(t, dir, &clock, captured)
	defer reopened.Stop()
	reopened.Start(context.Background(), send)
	reopened.ReceiveMessage(summaryCommand(200, 3, "25"), send)
	if action := summaryAction(t, send); !strings.Contains(summaryReplyText(action), "每小时") {
		t.Fatalf("cooldown after restart = %#v", action)
	}
	// The configured admin can request an overlapping range immediately.
	reopened.ReceiveMessage(summaryCommand(200, 1, "25"), send)
	expectSummaryStarted(t, send)
	request = summaryAction(t, send)
	replyHistory(t, reopened, request, messages)
	if lines = <-captured; len(lines) != 25 {
		t.Fatalf("admin AI input count = %d", len(lines))
	}
	summaryAction(t, send)
	waitSummaryIdle(t, reopened, 200)
	clock.Store(start.Add(time.Hour).Unix())
	reopened.ReceiveMessage(summaryCommand(200, 3, "25"), send)
	expectSummaryStarted(t, send)
	if action := summaryAction(t, send); action.Action != "get_group_msg_history" {
		t.Fatalf("cooldown expiry = %#v", action)
	}
}

func TestGroupSummaryPaginatesCount(t *testing.T) {
	previousConfig := base.Config
	base.Config = &base.Configuration{Admin: 1}
	t.Cleanup(func() { base.Config = previousConfig })
	var clock atomic.Int64
	start := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	clock.Store(start.Unix())
	captured := make(chan []summaryMessage, 1)
	plugin := newTestGroupSummary(t, t.TempDir(), &clock, captured)
	send := make(chan []byte, 10)
	plugin.Start(context.Background(), send)
	defer plugin.Stop()
	plugin.ReceiveMessage(summaryCommand(400, 1, "enable"), send)
	summaryAction(t, send)
	plugin.ReceiveMessage(summaryCommand(400, 1, "200"), send)
	expectSummaryStarted(t, send)
	first := summaryAction(t, send)
	newest := make([]structs.MessageStruct, 100)
	for i := range newest {
		newest[i] = historyMessage(400, int64(i+101), start.Add(-time.Duration(200-i)*time.Second).Unix(), "新消息")
	}
	replyHistory(t, plugin, first, newest)
	next := summaryAction(t, send)
	if next.Params.MessageSeq != "101" {
		t.Fatalf("pagination cursor = %q, want 101", next.Params.MessageSeq)
	}
	older := make([]structs.MessageStruct, 100)
	for i := range older {
		older[i] = historyMessage(400, int64(i+1), start.Add(-time.Duration(300-i)*time.Second).Unix(), "旧消息")
	}
	replyHistory(t, plugin, next, older)
	select {
	case lines := <-captured:
		if len(lines) != 200 || lines[0].ID != 1 || lines[199].ID != 200 {
			t.Fatalf("paginated input count = %d", len(lines))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("paginated summary did not start")
	}
	summaryForwardContents(t, summaryAction(t, send))
}

func TestGroupSummaryMigratesOldState(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE group_summary_groups (group_id INTEGER PRIMARY KEY, enabled INTEGER NOT NULL, generation INTEGER NOT NULL, next_at INTEGER NOT NULL, last_manual_at INTEGER NOT NULL, since_at INTEGER NOT NULL, cursor_id INTEGER NOT NULL)`,
		`CREATE TABLE group_summary_messages (id INTEGER PRIMARY KEY, content TEXT)`,
		`INSERT INTO group_summary_groups VALUES (500, 1, 1, 0, 123, 0, 0)`,
		`INSERT INTO group_summary_messages VALUES (1, 'old private chat')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	plugin, err := newGroupSummary(dir, "model", "key", func(context.Context, []summaryMessage) (summaryResult, error) {
		return summaryResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer plugin.Stop()
	var tableCount int
	if err := plugin.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'group_summary_messages'`).Scan(&tableCount); err != nil || tableCount != 0 {
		t.Fatalf("old message table count = %d, err = %v", tableCount, err)
	}
	plugin.mu.Lock()
	state, err := plugin.stateLocked(500)
	plugin.mu.Unlock()
	if err != nil || !state.enabled || state.lastManual != 123 {
		t.Fatalf("migrated state = %#v, err = %v", state, err)
	}
	var columnCount int
	if err := plugin.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('group_summary_groups')`).Scan(&columnCount); err != nil || columnCount != 3 {
		t.Fatalf("state column count = %d, err = %v", columnCount, err)
	}
}

func TestGroupSummaryDisableCancelsHistory(t *testing.T) {
	previousConfig := base.Config
	base.Config = &base.Configuration{Admin: 1}
	t.Cleanup(func() { base.Config = previousConfig })
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	captured := make(chan []summaryMessage, 1)
	plugin := newTestGroupSummary(t, t.TempDir(), &clock, captured)
	send := make(chan []byte, 10)
	plugin.Start(context.Background(), send)
	defer plugin.Stop()
	plugin.ReceiveMessage(summaryCommand(600, 1, "enable"), send)
	summaryAction(t, send)
	plugin.ReceiveMessage(summaryCommand(600, 1, "1h"), send)
	expectSummaryStarted(t, send)
	request := summaryAction(t, send)
	plugin.ReceiveMessage(summaryCommand(600, 1, "disable"), send)
	summaryAction(t, send)
	replyHistory(t, plugin, request, []structs.MessageStruct{historyMessage(600, 1, clock.Load(), "too late")})
	select {
	case <-captured:
		t.Fatal("disabled group reached AI")
	case action := <-send:
		t.Fatalf("disabled group sent action: %s", action)
	default:
	}
}

func TestGroupSummaryTranscriptAndRoastLimit(t *testing.T) {
	at := time.Date(2026, 9, 23, 12, 34, 0, 0, time.UTC)
	got := formatGroupTranscript([]summaryMessage{{Time: at, Name: "群名片", Content: "第一行\n第二行"}})
	want := "[2026-09-23 20:34] 群名片: 第一行 第二行\n"
	if got != want {
		t.Fatalf("transcript = %q, want %q", got, want)
	}
	if nextDay := formatGroupSummaryTime(time.Date(2026, 9, 23, 20, 0, 0, 0, time.UTC)); nextDay != "2026-09-24 04:00" {
		t.Fatalf("next-day China time = %q", nextDay)
	}
	if got := trimRoastSentences("一句。二句！三句？四句。"); got != "一句。二句！三句？" {
		t.Fatalf("roast = %q", got)
	}
	if got := trimRoastSentences("**一句。**\n\n二句！"); got != "**一句。**\n\n二句！" {
		t.Fatalf("roast markdown = %q", got)
	}
}

func TestSimplifySummaryMarkdown(t *testing.T) {
	input := "### 1. 《鸣潮》日常任务机制\n\n- **初四来福**：提到《鸣潮》的日常任务可以不做。\n# 已是单个标题\n*已有强调*"
	want := "# 1. 《鸣潮》日常任务机制\n\n- *初四来福*：提到《鸣潮》的日常任务可以不做。\n# 已是单个标题\n*已有强调*"
	if got := simplifySummaryMarkdown(input); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestGroupSummaryPlainTextIgnoresNonText(t *testing.T) {
	segments := []cqcode.ArrayMessage{
		*cqcode.Text("先看这张"),
		{Type: "image", Data: map[string]any{"url": "https://example.com/a.jpg", "file": "local.jpg"}},
		{Type: "face", Data: map[string]any{"id": "1"}},
		{Type: "image", Data: map[string]any{"url": "https://example.com/emoji.gif", "emoji_id": "emoji"}},
		{Type: "video", Data: map[string]any{"url": "https://example.com/video.mp4"}},
		{Type: "file", Data: map[string]any{"url": "https://example.com/file.pdf"}},
		{Type: "record", Data: map[string]any{"url": "https://example.com/voice.amr"}},
		{Type: "image", Data: map[string]any{"url": "/local/path", "file": "https://example.com/b.png"}},
		{Type: "image", Data: map[string]any{"file": "base64://not-a-link"}},
	}
	want := "先看这张"
	if got := summaryPlainText(segments); got != want {
		t.Fatalf("chat content = %q, want %q", got, want)
	}
	if got := summaryPlainText(segments[1:2]); got != "" {
		t.Fatalf("image-only content = %q", got)
	}
}

func TestGroupSummaryStringHistoryIgnoresImages(t *testing.T) {
	messages, err := decodeSummaryHistoryMessages([]json.RawMessage{
		json.RawMessage(`{"message_id":1,"time":1,"message":"文字[CQ:image,file=test.jpg,url=https://example.com/c.jpg][CQ:video,file=clip.mp4]后续"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := summaryPlainText(messages[0].Message); got != "文字 后续" {
		t.Fatalf("chat content = %q", got)
	}
}
