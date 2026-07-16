package chatai

import (
	"MacArthurGo/plugins/essentials"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type QWen struct {
	Enabled bool
	Args    []string
	Model   string
	ApiKey  string
}

func (q *QWen) RequireAnswer(str string) []string {
	var res []string
	const api = "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"

	payload := map[string]interface{}{
		"model": q.Model,
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
		res = append(res, fmt.Sprintf("QWen marshal error: %v", err))
		return res
	}

	req, err := http.NewRequest("POST", api, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("QWen request error: %v", err)
		res = append(res, fmt.Sprintf("QWen request error: %v", err))
		return res
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", q.ApiKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := essentials.HTTPClient.Do(req)
	if err != nil {
		log.Printf("QWen response error: %v", err)
		res = append(res, fmt.Sprintf("QWen response error: %v", err))
		return res
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("QWen close error: %v", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res = append(res, fmt.Sprintf("QWen response error: %s", resp.Status))
		return res
	}

	var result struct {
		Output struct {
			Text string `json:"text"`
		} `json:"output"`
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("QWen unmarshal error: %v", err)
		res = append(res, fmt.Sprintf("QWen unmarshal error: %v", err))
		return res
	}
	if result.Output.Text != "" {
		res = append(res, q.Model+": "+result.Output.Text)
		return res
	}
	res = append(res, fmt.Sprintf("QWen response error: code=%s message=%s request_id=%s", result.Code, result.Message, result.RequestID))
	return res
}
