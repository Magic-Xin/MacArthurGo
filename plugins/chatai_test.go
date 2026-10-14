package plugins

import (
	"MacArthurGo/plugins/chatai"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatAIGeminiForwardBlocks(t *testing.T) {
	for _, model := range []string{chatai.GeminiProModel, chatai.GeminiFlashModel} {
		t.Run(model, func(t *testing.T) {
			plugin := &ChatAI{groupForward: true}
			responses := []string{
				"[" + model + "] thinking:\nthought summary",
				"[" + model + "] answer:\nanswer",
			}
			send := make(chan []byte, 1)
			origin := &structs.MessageStruct{MessageType: "group", MessageId: 88, GroupId: 123, UserId: 456, CleanMessage: []cqcode.ArrayMessage{*cqcode.Text("original prompt")}}
			origin.Sender.Nickname = "nickname"
			origin.Sender.Card = "群名片"
			plugin.sendGeminiTextResponse(origin, responses, send)
			var action struct {
				Action, Echo string
				Params       structs.GroupForward
			}
			if err := json.Unmarshal(<-send, &action); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { essentials.TakeCache(action.Echo) })
			if action.Action != "send_group_forward_msg" || !strings.HasPrefix(action.Echo, "geminisend|88|") || action.Params.GroupId != 123 || len(action.Params.Messages) != len(responses)+1 {
				t.Fatalf("forward = %#v", action)
			}
			prompt := action.Params.Messages[0]
			if prompt.Data.Uin != "456" || prompt.Data.Name != "群名片" || len(prompt.Data.Content) != 1 || prompt.Data.Content[0].Data["text"] != "original prompt" {
				t.Fatalf("original prompt node = %#v", prompt)
			}
			for i, node := range action.Params.Messages[1:] {
				if len(node.Data.Content) != 1 || node.Data.Content[0].Type != "text" || node.Data.Content[0].Data["text"] != responses[i] {
					t.Errorf("node %d = %#v", i, node)
				}
			}
		})
	}
}

func TestChatAIGeminiPrivateBlocksStaySeparated(t *testing.T) {
	plugin := &ChatAI{groupForward: true}
	send := make(chan []byte, 1)
	responses := []string{"[gemini-4-argon] thinking:\nthought summary", "[gemini-4-argon] answer:\nanswer"}
	plugin.sendGeminiTextResponse(&structs.MessageStruct{MessageType: "private", UserId: 42}, responses, send)
	var action struct {
		Action string
		Params structs.Message
	}
	if err := json.Unmarshal(<-send, &action); err != nil {
		t.Fatal(err)
	}
	if action.Action != "send_msg" || len(action.Params.Message) != 1 || action.Params.Message[0].Data["text"] != strings.Join(responses, "\n\n") {
		t.Fatalf("private reply = %#v", action)
	}
}

func TestChatAIGeminiForwardKeepsQuotedSource(t *testing.T) {
	var sourceEcho structs.EchoMessageStruct
	if err := json.Unmarshal([]byte(`{"status":"ok","data":{"sender":{"user_id":42,"nickname":"quoted nickname","card":"引用者群名片"},"message":[{"type":"text","data":{"text":"quoted source"}}]}}`), &sourceEcho); err != nil {
		t.Fatal(err)
	}
	origin := &structs.MessageStruct{MessageType: "group", MessageId: 99, GroupId: 123, UserId: 456,
		CleanMessage: []cqcode.ArrayMessage{*cqcode.Reply("42"), *cqcode.Text(""), *cqcode.Text("high original prompt")}}
	origin.Sender.Nickname = "request nickname"
	plugin := &ChatAI{groupForward: true}
	send := make(chan []byte, 1)
	plugin.sendGeminiTextResponse(origin, []string{"thinking", "answer"}, send, geminiQuotedSource(&sourceEcho))
	var action struct {
		Echo   string
		Params structs.GroupForward
	}
	if err := json.Unmarshal(<-send, &action); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { essentials.TakeCache(action.Echo) })
	if len(action.Params.Messages) != 4 {
		t.Fatalf("quoted forward nodes = %#v", action.Params.Messages)
	}
	quoted, prompt := action.Params.Messages[0], action.Params.Messages[1]
	if quoted.Data.Uin != "42" || quoted.Data.Name != "引用者群名片" || quoted.Data.Content[0].Data["text"] != "quoted source" {
		t.Errorf("quoted source = %#v", quoted)
	}
	if prompt.Data.Uin != "456" || prompt.Data.Name != "request nickname" || len(prompt.Data.Content) != 1 || prompt.Data.Content[0].Data["text"] != "high original prompt" {
		t.Errorf("request source = %#v", prompt)
	}
	if len(origin.CleanMessage) != 3 || origin.CleanMessage[0].Type != "reply" {
		t.Fatal("forward preparation mutated the original message")
	}
}

func TestChatAIGeminiForwardSourceImageDoesNotMutateOriginal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()
	original := []cqcode.ArrayMessage{{Type: "image", Data: map[string]any{"file": "original.jpg", "url": server.URL}}}
	prepared := geminiForwardSourceContent(original)
	if len(prepared) != 1 || prepared[0].Data["file"] != "base64://aW1hZ2U=" || original[0].Data["file"] != "original.jpg" {
		t.Fatalf("prepared = %#v, original = %#v", prepared, original)
	}
}

func TestChatAIGeminiForwardFailureOnlyNotifiesOnce(t *testing.T) {
	plugin := &ChatAI{groupForward: true}
	origin := &structs.MessageStruct{MessageType: "group", MessageId: 88, GroupId: 123, UserId: 456,
		CleanMessage: []cqcode.ArrayMessage{*cqcode.Text("original prompt")}}
	responses := []string{"[gemini-4-argon] thinking:\n" + strings.Repeat("中", 2100), "[gemini-4-argon] answer:\nanswer"}
	send := make(chan []byte, 8)
	plugin.sendGeminiTextResponse(origin, responses, send)
	var forward struct{ Echo string }
	if err := json.Unmarshal(<-send, &forward); err != nil {
		t.Fatal(err)
	}
	failure := &structs.EchoMessageStruct{Echo: forward.Echo, Status: "failed", Retcode: 1200, Message: "发送转发消息失败"}
	plugin.ReceiveEcho(failure, send)
	var action struct {
		Action, Echo string
		Params       structs.Message
	}
	select {
	case payload := <-send:
		if err := json.Unmarshal(payload, &action); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("missing failure notification")
	}
	if action.Action != "send_msg" || action.Echo != "" || action.Params.GroupId != origin.GroupId || len(action.Params.Message) != 2 || action.Params.Message[0].Type != "reply" || action.Params.Message[0].Data["id"] != "88" || action.Params.Message[1].Data["text"] != "合并转发发送失败" {
		t.Fatalf("failure notification = %#v", action)
	}
	plugin.ReceiveEcho(failure, send)
	select {
	case payload := <-send:
		t.Fatalf("failure sent an extra message: %s", payload)
	default:
	}
}

func TestChatAIGeminiForwardSuccessReleasesCache(t *testing.T) {
	plugin := &ChatAI{groupForward: true}
	send := make(chan []byte, 1)
	plugin.sendGeminiTextResponse(&structs.MessageStruct{MessageType: "group", MessageId: 77, GroupId: 123}, []string{"thinking", "answer"}, send)
	var action struct{ Echo string }
	if err := json.Unmarshal(<-send, &action); err != nil {
		t.Fatal(err)
	}
	plugin.ReceiveEcho(&structs.EchoMessageStruct{Echo: action.Echo, Status: "ok"}, send)
	if _, exists := essentials.GetCache(action.Echo); exists || len(send) != 0 {
		t.Fatal("successful forward retained its request or sent a failure notification")
	}
}

func TestChatAIGeminiCommandModels(t *testing.T) {
	plugin := &ChatAI{
		ChatGPT: &chatai.ChatGPT{}, QWen: &chatai.QWen{}, Github: &chatai.Github{},
		Gemini:   &chatai.Gemini{Enabled: true, ArgsMap: map[string]string{"pro": "/gp", "flash": "/gf"}},
		FullArgs: []string{"/gp", "/gf"},
	}
	for _, test := range []struct{ command, model string }{{"/gp", chatai.GeminiProModel}, {"/gf", chatai.GeminiFlashModel}} {
		t.Run(test.command, func(t *testing.T) {
			send := make(chan []byte, 1)
			message := &structs.MessageStruct{MessageType: "group", MessageId: 987654321, Command: test.command,
				CleanMessage: []cqcode.ArrayMessage{*cqcode.Reply("42"), *cqcode.Text("high explain")}}
			plugin.ReceiveMessage(message, send)
			var action struct{ Action, Echo string }
			if err := json.Unmarshal(<-send, &action); err != nil || action.Action != "get_msg" || action.Echo != "gemini|987654321|"+test.model {
				t.Fatalf("command action = %#v, err = %v", action, err)
			}
		})
	}
}
