package plugins

import (
	_struct "MacArthurGo/structs"
	"context"
	"github.com/gookit/config/v2"
	"github.com/sashabaranov/go-openai"
	"github.com/vinta/pangu"
	"log"
	"strings"
)

func ChatGPT(ctx *map[string]any, words *[]string, send *chan []byte) {
	if !config.Bool("plugins.chatGPT.enable") || (*words)[0] != config.String("plugins.chatGPT.args") || len(*words) < 2 {
		return
	}

	client := openai.NewClient(config.String("plugins.chatGPT.apiKey"))
	str := strings.Join((*words)[1:], " ")

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
		data = append(data, *ConstructForwardNode(&str, sender["nickname"].(string), int64(sender["user_id"].(float64))),
			*ConstructForwardNode(&reply, info.NickName, info.UserId))
		*send <- *SendGroupForward(ctx, &data)
	} else {
		*send <- *SendMsg(ctx, reply, false, false)
	}
}
