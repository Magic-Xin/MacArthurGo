package chatai

import (
	"bytes"
	"encoding/json"
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
		"messages": [
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

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		res = append(res, fmt.Sprintf("Unmarshal error: %v", err))
		return &res
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		res = append(res, "No choices found")
		return &res
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		res = append(res, "First choice not found")
		return &res
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		res = append(res, "Message field not found")
		return &res
	}

	contentField, ok := message["content"].(string)
	if !ok {
		res = append(res, "Content field not found")
		return &res
	}

	res = append(res, contentField)
	return &res
}
