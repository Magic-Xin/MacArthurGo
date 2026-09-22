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
	"strconv"
	"strings"

	"google.golang.org/genai"
)

type Gemini struct {
	Enabled bool
	ArgsMap map[string]string
	ApiKey  string
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

	resp, err := g.GetResponse(parts, modelName)
	if err != nil {
		log.Printf("Get response error: %v", err)
	}

	return resp, nil
}

func (g *Gemini) RequireEchoAnswer(originMessage, echoMessage []cqcode.ArrayMessage, modelName string) []string {
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

	resp, err := g.GetResponse(parts, modelName)
	if err != nil {
		log.Printf("Get response error: %v", err)
	}

	return resp
}

func (g *Gemini) GetResponse(parts []*genai.Part, modelName string) ([]string, error) {
	var res []string

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  g.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("Gemini client error: %v", err)
		res = append(res, fmt.Sprintf("Gemini client error: %v", err))
		return res, err
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

	config.Tools = geminiRequestTools(modelName)
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
		res = append(res, fmt.Sprintf("Gemini generate error: %v", err))
		return res, err
	}

	res = append(res, modelName+" response: ")

	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, part := range c.Content.Parts {
			if part.Text != "" {
				res = append(res, essentials.RemoveMarkdown(part.Text))
			}
			if part.InlineData != nil {
				img := base64.StdEncoding.EncodeToString(part.InlineData.Data)
				res = append(res, "base64://"+img)
			}
		}
		if c.GroundingMetadata != nil {
			for _, chunk := range c.GroundingMetadata.GroundingChunks {
				if chunk != nil && chunk.Maps != nil && chunk.Maps.URI != "" {
					res = append(res, "Google Maps: "+chunk.Maps.Title+" "+chunk.Maps.URI)
				}
			}
		}
	}

	return res, nil
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

func geminiRequestTools(modelName string) []*genai.Tool {
	tools := geminiTools(modelName)
	if modelName != "gemini-3.1-pro-preview" {
		return tools
	}

	// Gemini 3.1 Pro rejects Google Maps combined with Search or URL Context.
	compatible := make([]*genai.Tool, 0, len(tools))
	for _, tool := range tools {
		if tool.GoogleMaps == nil {
			compatible = append(compatible, tool)
		}
	}
	return compatible
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
