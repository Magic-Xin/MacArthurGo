package chatai

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/genai"
)

const (
	GeminiFlashModel = "gemini-3.8-flash"
	GeminiProModel   = "gemini-4-argon"
	GeminiImageModel = "gemini-3-pro-image-preview"
)

var geminiThinkingArgument = regexp.MustCompile(`(?i)^\s*(low|mid|medium|high)(?:\s+|$)`)

type Gemini struct {
	Enabled     bool
	ArgsMap     map[string]string
	ApiKey      string
	httpOptions genai.HTTPOptions
}

type RMap struct {
	Data      []cqcode.ArrayMessage
	OriginStr string
	Time      int64
}

type HMap struct {
	History []*genai.Content
	Time    int64
}

func (g *Gemini) RequireAnswer(message []cqcode.ArrayMessage, messageID int64, modelName string) ([]string, []byte) {
	message, thinkingLevel := parseGeminiThinking(message, modelName)
	var parts []*genai.Part

	for _, msg := range message {
		switch msg.Type {
		case "image":
			if url, ok := msg.Data["url"].(string); ok {
				if data, imgType, err := g.ImageProcessing(essentials.GetImageData(url)); err == nil {
					parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: "image/" + imgType}})
				} else {
					log.Printf("Image processing error: %v", err)
				}
			}
		case "reply":
			echo := fmt.Sprintf("gemini|%d|%s", messageID, modelName)
			idStr := msg.Data["id"].(string)
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				log.Printf("Failed to convert id to int64: %v", err)
				continue
			}
			return nil, essentials.SendAction("get_msg", structs.GetMsg{Id: id}, echo)
		case "text":
			if text, ok := msg.Data["text"].(string); ok && text != "" {
				parts = append(parts, &genai.Part{Text: text})
			}
		}
	}

	resp, err := g.GetResponse(parts, modelName, thinkingLevel)
	if err != nil {
		log.Printf("Get response error: %v", err)
	}

	return resp, nil
}

func (g *Gemini) RequireEchoAnswer(originMessage, echoMessage []cqcode.ArrayMessage, modelName string) []string {
	originMessage, thinkingLevel := parseGeminiThinking(originMessage, modelName)
	var parts []*genai.Part

	for _, msg := range originMessage {
		switch msg.Type {
		case "image":
			if url, ok := msg.Data["url"].(string); ok {
				if data, imgType, err := g.ImageProcessing(essentials.GetImageData(url)); err == nil {
					parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: "image/" + imgType}})
				} else {
					log.Printf("Image processing error: %v", err)
				}
			}
		case "text":
			if text, ok := msg.Data["text"].(string); ok && text != "" {
				parts = append(parts, &genai.Part{Text: text})
			}
		}
	}

	for _, msg := range echoMessage {
		switch msg.Type {
		case "image":
			if url, ok := msg.Data["url"].(string); ok {
				if data, imgType, err := g.ImageProcessing(essentials.GetImageData(url)); err == nil {
					parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: "image/" + imgType}})
				} else {
					log.Printf("Image processing error: %v", err)
				}
			}
		case "text":
			if text, ok := msg.Data["text"].(string); ok {
				parts = append(parts, &genai.Part{Text: text})
			}
		}
	}

	resp, err := g.GetResponse(parts, modelName, thinkingLevel)
	if err != nil {
		log.Printf("Get response error: %v", err)
	}

	return resp
}

func (g *Gemini) GetResponse(parts []*genai.Part, modelName string, thinkingLevel genai.ThinkingLevel) ([]string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:      g.ApiKey,
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: g.httpOptions,
	})
	if err != nil {
		log.Printf("Gemini client error: %v", err)
		return geminiErrorResponse(modelName, fmt.Sprintf("Gemini client error: %v", err)), err
	}

	contents := []*genai.Content{{Parts: parts}}
	config := &genai.GenerateContentConfig{
		SafetySettings: []*genai.SafetySetting{
			{
				Category:  genai.HarmCategoryHateSpeech,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
			{
				Category:  genai.HarmCategoryDangerousContent,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
			{
				Category:  genai.HarmCategoryHarassment,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
			{
				Category:  genai.HarmCategorySexuallyExplicit,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
		},
	}

	config.ThinkingConfig = geminiThinkingConfig(modelName, thinkingLevel)
	config.Tools = geminiTools(modelName)
	if len(config.Tools) > 1 {
		includeToolInvocations := true
		config.ToolConfig = &genai.ToolConfig{IncludeServerSideToolInvocations: &includeToolInvocations}
	}

	if strings.Contains(modelName, "image") {
		config.ResponseModalities = []string{"TEXT", "IMAGE"}
	}

	resp, err := client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		log.Printf("Gemini generate error: %v", err)
		return geminiErrorResponse(modelName, fmt.Sprintf("Gemini generate error: %v", err)), err
	}

	return formatGeminiResponse(resp, modelName), nil
}

func IsGeminiTextModel(modelName string) bool {
	return modelName == GeminiFlashModel || modelName == GeminiProModel
}

// Only the first text argument after the command selects a thinking level.
// Copy the changed segment so quote handling can reuse the original request.
func parseGeminiThinking(message []cqcode.ArrayMessage, modelName string) ([]cqcode.ArrayMessage, genai.ThinkingLevel) {
	if !IsGeminiTextModel(modelName) {
		return message, ""
	}
	for i, segment := range message {
		text, ok := segment.Data["text"].(string)
		if segment.Type != "text" || !ok || strings.TrimSpace(text) == "" {
			continue
		}
		match := geminiThinkingArgument.FindStringSubmatch(text)
		if match == nil {
			break
		}
		level := genai.ThinkingLevel(strings.ToUpper(match[1]))
		if level == "MID" {
			level = genai.ThinkingLevelMedium
		}
		cleaned := append([]cqcode.ArrayMessage(nil), message...)
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		data["text"] = text[len(match[0]):]
		cleaned[i].Data = data
		return cleaned, level
	}
	return message, ""
}

func geminiThinkingConfig(modelName string, requested genai.ThinkingLevel) *genai.ThinkingConfig {
	if !IsGeminiTextModel(modelName) {
		return nil
	}
	// These models cannot turn internal reasoning off. Default to their lowest
	// level and always request a thought summary alongside the answer.
	level := requested
	if level == "" {
		level = genai.ThinkingLevelLow
	}
	return &genai.ThinkingConfig{ThinkingLevel: level, IncludeThoughts: true}
}

func geminiErrorResponse(modelName, message string) []string {
	if IsGeminiTextModel(modelName) {
		return []string{
			fmt.Sprintf("[%s] thinking:\n请求失败，未获取 thinking 摘要", modelName),
			fmt.Sprintf("[%s] answer:\n%s", modelName, message),
		}
	}
	return []string{message}
}

func formatGeminiResponse(resp *genai.GenerateContentResponse, modelName string) []string {
	var thoughts, answer strings.Builder
	var images, mapLinks []string

	if resp != nil {
		for _, c := range resp.Candidates {
			if c == nil || c.Content == nil {
				continue
			}
			for _, part := range c.Content.Parts {
				if part == nil {
					continue
				}
				if part.Thought {
					thoughts.WriteString(part.Text)
					continue
				}
				if part.Text != "" {
					answer.WriteString(part.Text)
				}
				if part.InlineData != nil {
					img := base64.StdEncoding.EncodeToString(part.InlineData.Data)
					images = append(images, "base64://"+img)
				}
			}
			if c.GroundingMetadata != nil {
				for _, chunk := range c.GroundingMetadata.GroundingChunks {
					if chunk != nil && chunk.Maps != nil && chunk.Maps.URI != "" {
						mapLinks = append(mapLinks, "Google Maps: "+chunk.Maps.Title+" "+chunk.Maps.URI)
					}
				}
			}
			// Candidates are alternatives, not consecutive pieces of one answer.
			break
		}
	}

	answerText := strings.TrimSpace(essentials.RemoveMarkdown(answer.String()))
	if len(mapLinks) > 0 {
		answerText = strings.TrimSpace(answerText + "\n" + strings.Join(mapLinks, "\n"))
	}
	if !IsGeminiTextModel(modelName) {
		res := []string{modelName + " response: "}
		if answerText != "" {
			res = append(res, answerText)
		}
		return append(res, images...)
	}
	thinkingText := strings.TrimSpace(essentials.RemoveMarkdown(thoughts.String()))
	if thinkingText == "" {
		thinkingText = "模型未返回 thinking 摘要"
	}
	if answerText == "" {
		answerText = "模型未返回回答"
	}
	return []string{
		fmt.Sprintf("[%s] thinking:\n%s", modelName, thinkingText),
		fmt.Sprintf("[%s] answer:\n%s", modelName, answerText),
	}
}

func geminiTools(modelName string) []*genai.Tool {
	if strings.Contains(modelName, "image") {
		return nil
	}
	return []*genai.Tool{
		{GoogleSearch: &genai.GoogleSearch{}},
		{GoogleMaps: &genai.GoogleMaps{}},
		{URLContext: &genai.URLContext{}},
	}
}

func (*Gemini) ImageProcessing(imgData *bytes.Buffer) ([]byte, string, error) {
	imgBody, err := io.ReadAll(imgData)
	if err != nil {
		return nil, "", err
	}
	switch imgType := http.DetectContentType(imgBody); imgType {
	case "image/jpeg":
		return imgBody, "jpeg", nil
	case "image/png":
		return imgBody, "png", nil
	case "image/gif":
		imgTemp, err := gif.Decode(bytes.NewReader(imgBody))
		if err != nil {
			return nil, "", err
		}
		buf := new(bytes.Buffer)
		err = jpeg.Encode(buf, imgTemp, nil)
		if err != nil {
			return nil, "", err
		}
		imgBody = buf.Bytes()

		return imgBody, "jpeg", nil
	default:
		return nil, "", fmt.Errorf("unsupported image type: %s", imgType)
	}
}
