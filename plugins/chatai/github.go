package chatai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type Github struct {
	Enabled bool
	ArgsMap map[string]string
	Token   string
}

func (c *Github) RequireAnswer(str string, model string) *[]string {
	const api = "https://models.github.ai/inference/chat/completions"

	var res []string

	payload := fmt.Sprintf(`{
		"message": [
			{
				"role": "user",
				"content": "%s"
			}
		],
		"model": "%s"
	}`, str, model)

	req, err := http.NewRequest("POST", api, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		res = append(res, fmt.Sprintf("NewRequest error: %v", err))
		return &res
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		res = append(res, fmt.Sprintf("Do error: %v", err))
		return &res
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			res = append(res, fmt.Sprintf("Body close error: %v", err))
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		res = append(res, fmt.Sprintf("Read body error: %v", err))
		return &res
	}

	res = append(res, string(body))
	return &res
}
