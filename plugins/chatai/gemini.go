package chatai

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"fmt"
	"google.golang.org/genai"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"strings"
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

func (g *Gemini) RequireAnswer(message *[]cqcode.ArrayMessage, messageID int64, modelName string) (*[]string, *[]byte) {
	var parts []*genai.Part

	for _, msg := range *message {
		switch msg.Type {
		case "image":
			if url, ok := msg.Data["url"].(string); ok {
				if data, imgType, err := g.ImageProcessing(essentials.GetImageData(url)); err == nil {
					parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: *data, MIMEType: "image/" + imgType}})
				} else {
					log.Printf("Image processing error: %v", err)
				}
			}
		case "reply":
			echo := fmt.Sprintf("gemini|%d|%s", messageID, modelName)
			return nil, essentials.SendAction("get_msg", structs.GetMsg{Id: msg.Data["id"].(string)}, echo)
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

func (g *Gemini) RequireEchoAnswer(originMessage *[]cqcode.ArrayMessage, echoMessage *[]cqcode.ArrayMessage, modelName string) *[]string {
	var parts []*genai.Part

	for _, msg := range *originMessage {
		if msg.Type == "image" {
			if url, ok := msg.Data["url"].(string); ok {
				if data, imgType, err := g.ImageProcessing(essentials.GetImageData(url)); err == nil {
					parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: *data, MIMEType: "image/" + imgType}})
				} else {
					log.Printf("Image processing error: %v", err)
				}
			}
		} else if msg.Type == "text" {
			if text, ok := msg.Data["text"].(string); ok && text != "" {
				parts = append(parts, &genai.Part{Text: text})
			}
		}
	}

	for _, msg := range *echoMessage {
		if msg.Type == "image" {
			if url, ok := msg.Data["url"].(string); ok {
				if data, imgType, err := g.ImageProcessing(essentials.GetImageData(url)); err == nil {
					parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: *data, MIMEType: "image/" + imgType}})
				} else {
					log.Printf("Image processing error: %v", err)
				}
			}
		} else if msg.Type == "text" {
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

func (g *Gemini) GetResponse(parts []*genai.Part, modelName string) (*[]string, error) {
	var res []string

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  g.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("Gemini client error: %v", err)
		res = append(res, fmt.Sprintf("Gemini client error: %v", err))
		return &res, err
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
			{
				Category:  genai.HarmCategoryCivicIntegrity,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
		},
	}

	if !strings.Contains(modelName, "thinking") {
		config.Tools = []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		}
	}

	resp, err := client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		log.Printf("Gemini generate error: %v", err)
		res = append(res, fmt.Sprintf("Gemini generate error: %v", err))
		return &res, err
	}

	res = append(res, modelName+" response: ")

	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, part := range c.Content.Parts {
			res = append(res, essentials.RemoveMarkdown(part.Text))
		}
	}

	return &res, nil
}

func (*Gemini) ImageProcessing(imgData *bytes.Buffer) (*[]byte, string, error) {
	imgBody, err := io.ReadAll(imgData)
	if err != nil {
		return nil, "", err
	}
	switch imgType := http.DetectContentType(imgBody); imgType {
	case "image/jpeg":
		return &imgBody, "jpeg", nil
	case "image/png":
		return &imgBody, "png", nil
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

		return &imgBody, "jpeg", nil
	default:
		return nil, "", fmt.Errorf("unsupported image type: %s", imgType)
	}
}
