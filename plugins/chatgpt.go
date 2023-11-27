package plugins

import (
	"MacArthurGo/plugins/essentials"
	_struct "MacArthurGo/structs"
	"context"
	"github.com/gookit/config/v2"
	"github.com/sashabaranov/go-openai"
	"github.com/vinta/pangu"
	"log"
	"strings"
)

type ChatGPT struct{}

func init() {
	chatGPT := essentials.Plugin{
		Name:            "ChatGPT",
		Enabled:         config.Bool("plugins.chatGPT.enable"),
		Arg:             config.String("plugins.chatGPT.args"),
		PluginInterface: &ChatGPT{},
	}
	essentials.PluginArray = append(essentials.PluginArray, &chatGPT)

	essentials.MessageArray = append(essentials.MessageArray, &chatGPT)
}

func (c *ChatGPT) ReceiveAll(ctx *map[string]any, send *chan []byte) {}

func (c *ChatGPT) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !essentials.CheckArgument(ctx, config.String("plugins.chatGPT.args")) || !config.Bool("plugins.chatGPT.enable") {
		return
	}

	words := essentials.SplitArgument(ctx)
	if len(words) < 2 {
		return
	}

	client := openai.NewClient(config.String("plugins.chatGPT.apiKey"))
	str := strings.Join((words)[1:], " ")

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: config.String("plugins.chatGPT.model"),
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
		return
	}

	reply := resp.Choices[0].Message.Content
	if config.Bool("plugins.chatGPT.pangu") {
		reply = pangu.SpacingText(reply)
	}

	if (*ctx)["message_type"].(string) == "group" && config.Bool("plugins.chatGPT.groupForward") {
		var data []_struct.ForwardNode
		sender := (*ctx)["sender"].(map[string]any)
		data = append(data, *essentials.ConstructForwardNode(&str, sender["nickname"].(string), int64(sender["user_id"].(float64))),
			*essentials.ConstructForwardNode(&reply, essentials.Info.NickName, essentials.Info.UserId))
		*send <- *essentials.SendGroupForward(ctx, &data)
	} else {
		*send <- *essentials.SendMsg(ctx, reply, false, false)
	}
}

func (c *ChatGPT) ReceiveEcho(ctx *map[string]any, send *chan []byte) {}
