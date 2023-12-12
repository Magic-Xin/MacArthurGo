package plugins

import (
	"MacArthurGo/plugins/essentials"
	_struct "MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"context"
	"github.com/gookit/config/v2"
	"github.com/sashabaranov/go-openai"
	"github.com/vinta/pangu"
	"log"
	"strings"
)

type ChatGPT struct {
	essentials.Plugin
	groupForward bool
	panGu        bool
	model        string
	apiKey       string
}

func init() {
	chatGPT := ChatGPT{
		Plugin: essentials.Plugin{
			Name:    "ChatGPT",
			Enabled: config.Bool("plugins.chatGPT.enable"),
			Args:    config.Strings("plugins.chatGPT.args"),
		},
		groupForward: config.Bool("plugins.chatGPT.groupForward"),
		panGu:        config.Bool("plugins.chatGPT.pangu"),
		model:        config.String("plugins.chatGPT.model"),
		apiKey:       config.String("plugins.chatGPT.apiKey"),
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &chatGPT})
}

func (c *ChatGPT) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (c *ChatGPT) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !essentials.CheckArgumentArray(ctx, &c.Args) || !c.Enabled {
		return
	}

	words := essentials.SplitArgument(ctx)
	if len(words) < 2 {
		return
	}

	client := openai.NewClient(c.apiKey)
	str := strings.Join((words)[1:], " ")

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
		*send <- *essentials.SendMsg(ctx, err.Error(), nil, false, false)
		return
	}

	reply := resp.Choices[0].Message.Content
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

func (c *ChatGPT) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}
