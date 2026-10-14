package plugins

import (
	"MacArthurGo/plugins/chatai"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
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
			plugin.sendGeminiTextResponse(&structs.MessageStruct{MessageType: "group", GroupId: 123, CleanMessage: []cqcode.ArrayMessage{*cqcode.Text("original prompt")}}, responses, "response-echo", send)
			var action struct {
				Action, Echo string
				Params       structs.GroupForward
			}
			if err := json.Unmarshal(<-send, &action); err != nil {
				t.Fatal(err)
			}
			if action.Action != "send_group_forward_msg" || action.Echo != "response-echo" || action.Params.GroupId != 123 || len(action.Params.Messages) != len(responses) {
				t.Fatalf("forward = %#v", action)
			}
			for i, node := range action.Params.Messages {
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
	plugin.sendGeminiTextResponse(&structs.MessageStruct{MessageType: "private", UserId: 42}, responses, "", send)
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
