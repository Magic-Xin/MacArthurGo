package chatai

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
)

type ChatGPT struct {
	Enabled bool
	Args    []string
	Model   string
	ApiKey  string
}

func (c *ChatGPT) RequireAnswer(str string) *[]string {
	var res []string
	client := openai.NewClient(c.ApiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: c.Model,
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
		res = append(res, fmt.Sprintf("ChatCompletion error: %v", err))
		return &res
	}

	res = append(res, c.Model+": "+resp.Choices[0].Message.Content)
	return &res
}
