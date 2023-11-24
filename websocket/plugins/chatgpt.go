package plugins

import (
	"context"
	"errors"
	"github.com/gookit/config/v2"
	"github.com/sashabaranov/go-openai"
	"github.com/vinta/pangu"
	"log"
	"strings"
)

func ChatGPT(words *[]string) (string, error) {
	if len(*words) < 2 {
		return "", errors.New("not enough arguments")
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
		return "", err
	}

	reply := resp.Choices[0].Message.Content
	if config.Bool("plugins.chatGPT.pangu") {
		reply = pangu.SpacingText(reply)
	}

	return reply, nil
}
