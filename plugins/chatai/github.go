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

type Results struct {
	Choices []struct {
		ContentFilterResults struct {
			Hate struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"hate"`
			ProtectedMaterialCode struct {
				Filtered bool `json:"filtered"`
				Detected bool `json:"detected"`
			} `json:"protected_material_code"`
			ProtectedMaterialText struct {
				Filtered bool `json:"filtered"`
				Detected bool `json:"detected"`
			} `json:"protected_material_text"`
			SelfHarm struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"self_harm"`
			Sexual struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"sexual"`
			Violence struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"violence"`
		} `json:"content_filter_results"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
		Message      struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"message"`
	} `json:"choices"`
	Created             int    `json:"created"`
	ID                  string `json:"id"`
	Model               string `json:"model"`
	Object              string `json:"object"`
	PromptFilterResults []struct {
		PromptIndex          int `json:"prompt_index"`
		ContentFilterResults struct {
			Hate struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"hate"`
			Jailbreak struct {
				Filtered bool `json:"filtered"`
				Detected bool `json:"detected"`
			} `json:"jailbreak"`
			SelfHarm struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"self_harm"`
			Sexual struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"sexual"`
			Violence struct {
				Filtered bool   `json:"filtered"`
				Severity string `json:"severity"`
			} `json:"violence"`
		} `json:"content_filter_results"`
	} `json:"prompt_filter_results"`
	SystemFingerprint string `json:"system_fingerprint"`
	Usage             struct {
		CompletionTokens        int `json:"completion_tokens"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
		PromptTokens        int `json:"prompt_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *Github) RequireAnswer(str string, model string) *[]string {
	const api = "https://models.inference.ai.azure.com/chat/completions"

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

	var result Results
	if err := json.Unmarshal(body, &result); err != nil {
		res = append(res, fmt.Sprintf("Unmarshal error: %v", err))
		return &res
	}

	if len(result.Choices) == 0 {
		res = append(res, "No choices found")
		return &res
	}

	firstChoice := result.Choices[0]
	message := firstChoice.Message

	if message.Content == "" {
		res = append(res, "Content field not found")
		return &res
	}

	res = append(res, fmt.Sprintf("%s response:", model))
	res = append(res, message.Content)

	return &res
}
