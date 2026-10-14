package essentials

import (
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSendGroupForwardBoundsCardPreviewsAndPreservesContent(t *testing.T) {
	longText := "[gemini-4-argon] thinking:\n" + strings.Repeat("长回复🙂\n", 1000)
	nodes := []structs.ForwardNode{
		*ConstructForwardNode("123", "群友", []cqcode.ArrayMessage{*cqcode.Text("原始问题")}),
		*ConstructForwardNode("456", "机器人", []cqcode.ArrayMessage{*cqcode.Text(longText)}),
		*ConstructForwardNode("456", "机器人", []cqcode.ArrayMessage{*cqcode.Text("回答\n" + longText)}),
		*ConstructForwardNode("123", "群友", []cqcode.ArrayMessage{*cqcode.Image("https://example.com/image.jpg")}),
		*ConstructForwardNode("456", "机器人", []cqcode.ArrayMessage{*cqcode.Text("第五条消息仍然保留")}),
	}
	var request struct {
		Action string               `json:"action"`
		Params structs.GroupForward `json:"params"`
		Echo   string               `json:"echo"`
	}
	payload := SendGroupForward(&structs.MessageStruct{GroupId: 789}, nodes, "forward-test")
	if !json.Valid(payload) || !utf8.Valid(payload) {
		t.Fatal("forward request must be valid UTF-8 JSON")
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	if request.Action != "send_group_forward_msg" || request.Params.GroupId != 789 || request.Echo != "forward-test" {
		t.Fatalf("unexpected forward request: %+v", request)
	}
	if !reflect.DeepEqual(request.Params.Messages, nodes) {
		t.Fatal("card previews must not change the forwarded messages or sender metadata")
	}
	if len(request.Params.News) != 4 {
		t.Fatalf("got %d previews, want 4", len(request.Params.News))
	}
	for _, preview := range request.Params.News {
		if utf8.RuneCountInString(preview.Text) > 50 || strings.ContainsAny(preview.Text, "\r\n") {
			t.Fatalf("preview is too long or contains line breaks: %q", preview.Text)
		}
	}
	if request.Params.News[0].Text != "群友: 原始问题" || request.Params.News[3].Text != "群友: [图片]" {
		t.Fatalf("unexpected previews: %+v", request.Params.News)
	}
}

func TestRemoveMarkdownOrder(t *testing.T) {
	input := "## **标题**\n![图片](https://example.com/a.jpg)\n[链接](https://example.com)"
	want := "标题\n图片\n链接"
	for i := 0; i < 100; i++ {
		if got := RemoveMarkdown(input); got != want {
			t.Fatalf("RemoveMarkdown() = %q, want %q", got, want)
		}
	}
}
