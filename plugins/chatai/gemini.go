package chatai

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"fmt"
	"github.com/chai2010/webp"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"strconv"
	"sync"
	"time"
)

type Gemini struct {
	Enabled    bool
	Args       []string
	ApiKey     string
	ReplyMap   sync.Map
	HistoryMap sync.Map
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

func (g *Gemini) RequireAnswer(str string, message *[]cqcode.ArrayMessage, messageID int64, modelName string, echoId int64) (*string, *[]byte) {
	var (
		images  []*genai.Blob
		prompts []genai.Part
		model   *genai.GenerativeModel
		history []*genai.Content
		res     string
		reply   string
	)

	for _, msg := range *message {
		if msg.Type == "image" && msg.Data["url"] != nil {
			imgData := essentials.GetImageData(msg.Data["url"].(string))
			data := g.ImageToWebp(imgData)
			img := genai.ImageData("webp", *data)
			images = append(images, &img)
		}
		if msg.Type == "reply" {
			reply = msg.Data["id"].(string)
		}
		if echoId != 0 && msg.Type == "text" {
			prompts = append(prompts, genai.Text(msg.Data["text"].(string)))
		}
	}

	if reply != "" && echoId == 0 {
		g.ReplyMap.Store(strconv.FormatInt(messageID, 10), RMap{Data: *message, OriginStr: str, Time: time.Now().Unix()})

		echo := fmt.Sprintf("gemini|%d|%s", messageID, modelName)
		return nil, essentials.SendAction("get_msg", structs.GetMsg{Id: reply}, echo)
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(g.ApiKey))
	if err != nil {
		log.Printf("Gemini client error: %v", err)
		res = fmt.Sprintf("Gemini client error: %v", err)
		return &res, nil
	}
	defer func(client *genai.Client) {
		err = client.Close()
		if err != nil {
			log.Printf("Gemini client close error: %v", err)
		}
	}(client)

	prompts = append(prompts, genai.Text(str))
	if len(images) > 0 {
		for _, img := range images {
			prompts = append(prompts, img)
		}
	}
	res = modelName + ": "

	model = client.GenerativeModel("models/" + modelName)
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
	}
	cs := model.StartChat()
	if echoId != 0 {
		value, ok := g.HistoryMap.Load(echoId)
		if ok {
			cs.History = value.(HMap).History
		}
	}

	resp, err := cs.SendMessage(ctx, prompts...)
	if err != nil {
		log.Printf("Gemini generate error: %v", err)
		res = fmt.Sprintf("Gemini generate error: %v", err)
		return &res, nil
	}

	var cts []*genai.Content

	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, part := range c.Content.Parts {
			res += fmt.Sprintf("%s", part)
		}
		cts = append(cts, c.Content)
	}

	history = append(history, &genai.Content{
		Parts: prompts,
		Role:  "user",
	})
	history = append(history, cts...)

	g.HistoryMap.Store(messageID, HMap{History: history, Time: time.Now().Unix()})

	return &res, nil
}

func (*Gemini) ImageToWebp(imageData *bytes.Buffer) *[]byte {
	img, _, err := image.Decode(imageData)
	if err != nil {
		log.Printf("Image decode error: %v", err)
		return nil
	}

	webpBytes, err := webp.EncodeRGBA(img, 50)
	if err != nil {
		log.Printf("Webp encode error: %v", err)
		return nil
	}

	return &webpBytes
}

func (g *Gemini) DeleteExpiredCache(expiration int64, interval int64) {
	for {
		g.ReplyMap.Range(func(key, value any) bool {
			if time.Now().Unix()-value.(RMap).Time > expiration {
				g.ReplyMap.Delete(key)
			}
			return true
		})
		g.HistoryMap.Range(func(key, value any) bool {
			if time.Now().Unix()-value.(HMap).Time > expiration {
				g.HistoryMap.Delete(key)
			}
			return true
		})
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
