package plugins

import (
	"MacArthurGo/plugins/essentials"
	_struct "MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gookit/config/v2"
	"github.com/sashabaranov/go-openai"
	"github.com/vinta/pangu"
	"io"
	"log"
	"net/http"
	"strings"
)

type ChatGPT struct {
	Enabled bool
	Args    []string
	model   string
	apiKey  string
}

type QWen struct {
	Enabled bool
	Args    []string
	model   string
	apiKey  string
}

type ChatAI struct {
	essentials.Plugin
	ChatGPT      *ChatGPT
	QWen         *QWen
	groupForward bool
	panGu        bool
}

func init() {
	chatGPT := ChatGPT{
		Enabled: config.Bool("plugins.chatAI.chatGPT.enable"),
		Args:    config.Strings("plugins.chatAI.chatGPT.args"),
		model:   config.String("plugins.chatAI.chatGPT.model"),
		apiKey:  config.String("plugins.chatAI.chatGPT.apiKey"),
	}
	qWen := QWen{
		Enabled: config.Bool("plugins.chatAI.qWen.enable"),
		Args:    config.Strings("plugins.chatAI.qWen.args"),
		model:   config.String("plugins.chatAI.qWen.model"),
		apiKey:  config.String("plugins.chatAI.qWen.apiKey"),
	}
	var args []string
	if chatGPT.Enabled {
		args = append(args, chatGPT.Args...)
	}
	if qWen.Enabled {
		args = append(args, qWen.Args...)
	}
	chatAI := ChatAI{
		Plugin: essentials.Plugin{
			Name:    "ChatAI",
			Enabled: config.Bool("plugins.chatAI.enable"),
			Args:    args,
		},
		ChatGPT:      &chatGPT,
		QWen:         &qWen,
		groupForward: config.Bool("plugins.chatAI.groupForward"),
		panGu:        config.Bool("plugins.chatAI.pangu"),
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &chatAI})
}

func (c *ChatAI) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (c *ChatAI) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !c.Enabled {
		return
	}

	words := essentials.SplitArgument(ctx)
	if len(words) < 2 {
		return
	}

	var reply string
	str := strings.Join(words[1:], "")
	if essentials.CheckArgumentArray(ctx, &c.ChatGPT.Args) {
		reply = *c.ChatGPT.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(ctx, &c.QWen.Args) {
		reply = *c.QWen.RequireAnswer(str)
	} else {
		return
	}

	if reply == "" {
		return
	}

	if c.panGu {
		reply = pangu.SpacingText(reply)
	}

	if (*ctx)["message_type"].(string) == "group" && c.groupForward {
		var data []_struct.ForwardNode
		originStr := (*ctx)["sender"].(map[string]any)["nickname"].(string) + "：" + str
		data = append(data, *essentials.ConstructForwardNode(&[]cqcode.ArrayMessage{*cqcode.Text(originStr)}),
			*essentials.ConstructForwardNode(&[]cqcode.ArrayMessage{*cqcode.Text(reply)}))
		*send <- *essentials.SendGroupForward(ctx, &data, "")
	} else {
		*send <- *essentials.SendMsg(ctx, reply, nil, false, false)
	}
}

func (c *ChatAI) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}

func (c *ChatGPT) RequireAnswer(str string) *string {
	if !c.Enabled {
		res := "ChatGPT disabled"
		return &res
	}
	client := openai.NewClient(c.apiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: c.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: str,
				},
			},
		},
	)

	if err != nil {
		log.Printf("ChatCompletion error: %v", err)
		res := fmt.Sprintf("ChatCompletion error: %v", err)
		return &res
	}

	return &resp.Choices[0].Message.Content
}

func (q *QWen) RequireAnswer(str string) *string {
	if q.Enabled {
		res := "QWen disabled"
		return &res
	}

	const api = "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"

	payload := map[string]interface{}{
		"model": q.model,
		"input": map[string][]map[string]string{
			"messages": {
				{
					"role":    "user",
					"content": str,
				},
			},
		},
		"params": map[string]any{
			"enable_search": true,
		},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("QWen marshal error: %v", err)
		res := fmt.Sprintf("QWen marshal error: %v", err)
		return &res
	}

	req, err := http.NewRequest("POST", api, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("QWen request error: %v", err)
		res := fmt.Sprintf("QWen request error: %v", err)
		return &res
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", q.apiKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("QWen response error: %v", err)
		res := fmt.Sprintf("QWen response error: %v", err)
		return &res
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("QWen close error: %v", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("QWen read body error: %v", err)
		res := fmt.Sprintf("QWen read body error: %v", err)
		return &res
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("QWen unmarshal error: %v", err)
		res := fmt.Sprintf("QWen unmarshal error: %v", err)
		return &res
	}
	ctx := i.(map[string]any)
	if ctx["output"] != nil {
		if ctx["output"].(map[string]any)["text"] != nil {
			res := ctx["output"].(map[string]any)["text"].(string)
			return &res
		}
	}
	res := "QWen json error"
	return &res
}
