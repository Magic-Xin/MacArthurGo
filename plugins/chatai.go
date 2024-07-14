package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/generative-ai-go/genai"
	"github.com/google/go-cmp/cmp"
	"github.com/sashabaranov/go-openai"
	"github.com/vinta/pangu"
	"google.golang.org/api/option"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChatGPT struct {
	Enabled bool
	Args    []string
	model   string
	apiKey  string
}

type QWen struct {
	Enabled bool
	Args    []string
	model   string
	apiKey  string
}

type Gemini struct {
	Enabled    bool
	Args       []string
	apiKey     string
	ReplyMap   sync.Map
	HistoryMap sync.Map
}

type ChatAI struct {
	ChatGPT      *ChatGPT
	QWen         *QWen
	Gemini       *Gemini
	Args         []string
	groupForward bool
	panGu        bool
}

func init() {
	chatGPT := ChatGPT{
		Enabled: base.Config.Plugins.ChatAI.ChatGPT.Enable,
		Args:    base.Config.Plugins.ChatAI.ChatGPT.Args,
		model:   base.Config.Plugins.ChatAI.ChatGPT.Model,
		apiKey:  base.Config.Plugins.ChatAI.ChatGPT.APIKey,
	}
	qWen := QWen{
		Enabled: base.Config.Plugins.ChatAI.QWen.Enable,
		Args:    base.Config.Plugins.ChatAI.QWen.Args,
		model:   base.Config.Plugins.ChatAI.QWen.Model,
		apiKey:  base.Config.Plugins.ChatAI.QWen.APIKey,
	}
	gemini := Gemini{
		Enabled: base.Config.Plugins.ChatAI.Gemini.Enable,
		Args:    base.Config.Plugins.ChatAI.Gemini.Args,
		apiKey:  base.Config.Plugins.ChatAI.Gemini.APIKey,
	}

	var args []string
	if chatGPT.Enabled {
		args = append(args, chatGPT.Args...)
	}
	if qWen.Enabled {
		args = append(args, qWen.Args...)
	}
	if gemini.Enabled {
		args = append(args, gemini.Args...)
	}

	chatAI := ChatAI{
		ChatGPT:      &chatGPT,
		QWen:         &qWen,
		Gemini:       &gemini,
		Args:         args,
		groupForward: base.Config.Plugins.ChatAI.GroupForward,
		panGu:        base.Config.Plugins.ChatAI.PanGu,
	}
	plugin := &essentials.Plugin{
		Name:      "chatAI",
		Enabled:   base.Config.Plugins.ChatAI.Enable,
		Args:      args,
		Interface: &chatAI,
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)

	go gemini.DeleteExpiredCache(3600, 1800)
}

func (c *ChatAI) ReceiveAll() *[]byte {
	return nil
}

func (c *ChatAI) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if !essentials.CheckArgumentArray(&messageStruct.Message, &c.Args) {
		return nil
	}

	words := essentials.SplitArgument(&messageStruct.Message)
	if len(words) < 2 {
		return nil
	}

	message := messageStruct.Message
	str := strings.Join(words[1:], " ")

	var (
		res  *string
		echo string
	)
	if essentials.CheckArgumentArray(&messageStruct.Message, &c.ChatGPT.Args) && c.ChatGPT.Enabled {
		res = c.ChatGPT.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(&messageStruct.Message, &c.QWen.Args) && c.QWen.Enabled {
		res = c.QWen.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(&messageStruct.Message, &c.Gemini.Args) && c.Gemini.Enabled {
		var action *[]byte
		messageID := messageStruct.MessageId
		if len(c.Gemini.Args) < 2 {
			res, action = c.Gemini.RequireAnswer(str, &message, messageID, "gemini-1.5-flash-latest", 0)
		} else {
			if essentials.CheckArgument(&message, c.Gemini.Args[0]) {
				res, action = c.Gemini.RequireAnswer(str, &message, messageID, "gemini-1.5-pro-latest", 0)
			} else {
				res, action = c.Gemini.RequireAnswer(str, &message, messageID, "gemini-1.5-flash-latest", 0)
			}
		}

		if action != nil {
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageID, 10), value)
			return action
		}
		echo = "geminisend|" + strconv.FormatInt(messageID, 10)
	} else {
		return nil
	}

	if res == nil {
		return nil
	}

	if c.panGu {
		*res = pangu.SpacingText(*res)
	}

	if messageStruct.MessageType == "group" && c.groupForward {
		var data []structs.ForwardNode
		uin := strconv.FormatInt(messageStruct.UserId, 10)
		name := messageStruct.Sender.Nickname
		originMessage := []cqcode.ArrayMessage{*cqcode.Text("@" + name + ": " + str)}
		data = append(data, *essentials.ConstructForwardNode(uin, name, &originMessage), *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(*res)}))
		return essentials.SendGroupForward(messageStruct, &data, echo)
	} else {
		return essentials.SendMsg(messageStruct, *res, nil, false, false, "")
	}
}

func (c *ChatAI) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct) *[]byte {
	split := strings.Split(echoMessageStruct.Echo, "|")

	if split[0] == "gemini" && !cmp.Equal(echoMessageStruct.Data, struct{}{}) {
		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Gemini get cache error")
		}
		originCtx := value.(essentials.EchoCache).Value
		if echoMessageStruct.Status != "ok" {
			return essentials.SendMsg(&originCtx, "Gemini reply args error", nil, false, false, "")
		}

		data, ok := c.Gemini.ReplyMap.Load(split[1])
		if !ok {
			log.Println("Gemini reply map load error")
			return nil
		}

		originStr := data.(struct {
			Data      []cqcode.ArrayMessage
			OriginStr string
			Time      int64
		}).OriginStr

		var res *string
		message := echoMessageStruct.Data.Message
		messageId, err := strconv.ParseInt(split[1], 10, 64)
		if err != nil {
			log.Printf("Echo id parse error: %v", err)
			return nil
		}
		res, _ = c.Gemini.RequireAnswer(originStr, &message, messageId, split[2], echoMessageStruct.Data.MessageId)

		if res == nil {
			return nil
		}

		echo := "geminisend|" + split[1]

		if c.panGu {
			*res = pangu.SpacingText(*res)
		}

		if originCtx.MessageType == "group" && c.groupForward {
			var data []structs.ForwardNode
			uin := strconv.FormatInt(originCtx.UserId, 10)
			name := originCtx.Sender.Nickname
			message := append([]cqcode.ArrayMessage{*cqcode.Text("@" + name + ": " + originStr)}, message...)
			data = append(data, *essentials.ConstructForwardNode(uin, name, &message))
			data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(*res)}))
			return essentials.SendGroupForward(&originCtx, &data, echo)
		} else {
			return essentials.SendMsg(&originCtx, *res, nil, false, false, echo)
		}
	} else if split[0] == "geminisend" {
		type HMap struct {
			History []*genai.Content
			Time    int64
		}
		key, err := strconv.ParseInt(split[1], 10, 64)
		if err != nil {
			log.Printf("Gemini send id parse error: %v", err)
			return nil
		}
		value, ok := c.Gemini.HistoryMap.Load(key)
		if !ok {
			log.Println("Gemini history map load error")
			return nil
		}
		c.Gemini.HistoryMap.Store(echoMessageStruct.Data.MessageId, value)
	}
	return nil
}

func (c *ChatGPT) RequireAnswer(str string) *string {
	client := openai.NewClient(c.apiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: c.model,
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
		res := fmt.Sprintf("ChatCompletion error: %v", err)
		return &res
	}

	res := c.model + ": " + resp.Choices[0].Message.Content
	return &res
}

func (q *QWen) RequireAnswer(str string) *string {
	const api = "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"

	payload := map[string]interface{}{
		"model": q.model,
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
		res := fmt.Sprintf("QWen marshal error: %v", err)
		return &res
	}

	req, err := http.NewRequest("POST", api, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("QWen request error: %v", err)
		res := fmt.Sprintf("QWen request error: %v", err)
		return &res
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", q.apiKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("QWen response error: %v", err)
		res := fmt.Sprintf("QWen response error: %v", err)
		return &res
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("QWen close error: %v", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("QWen read body error: %v", err)
		res := fmt.Sprintf("QWen read body error: %v", err)
		return &res
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("QWen unmarshal error: %v", err)
		res := fmt.Sprintf("QWen unmarshal error: %v", err)
		return &res
	}
	ctx := i.(map[string]any)
	if ctx["output"] != nil {
		if ctx["output"].(map[string]any)["text"] != nil {
			res := q.model + ": " + ctx["output"].(map[string]any)["text"].(string)
			return &res
		}
	}
	res := "QWen json error"
	return &res
}

func (g *Gemini) RequireAnswer(str string, message *[]cqcode.ArrayMessage, messageID int64, modelName string, echoId int64) (*string, *[]byte) {
	var (
		images []struct {
			Data    *[]byte
			ImgType string
		}

		prompts []genai.Part
		model   *genai.GenerativeModel
		history []*genai.Content
		res     string
		reply   string
	)

	type HMap struct {
		History []*genai.Content
		Time    int64
	}

	for _, msg := range *message {
		if msg.Type == "image" && msg.Data["url"] != nil {
			imgUrl, _ := essentials.GetUniversalImgURL(msg.Data["url"].(string))
			data, imgType, err := g.ImageProcessing(imgUrl)
			if err != nil {
				log.Printf("Image processing error: %v", err)
				continue
			}
			images = append(images, struct {
				Data    *[]byte
				ImgType string
			}{Data: data, ImgType: imgType})
		}
		if msg.Type == "reply" {
			reply = msg.Data["id"].(string)
		}
	}

	if reply != "" && echoId == 0 {
		g.ReplyMap.Store(strconv.FormatInt(messageID, 10), struct {
			Data      []cqcode.ArrayMessage
			OriginStr string
			Time      int64
		}{Data: *message, OriginStr: str, Time: time.Now().Unix()})

		echo := fmt.Sprintf("gemini|%d|%s", messageID, modelName)
		return nil, essentials.SendAction("get_msg", structs.GetMsg{Id: reply}, echo)
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
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
			prompts = append(prompts, genai.ImageData(img.ImgType, *img.Data))
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

func (g *Gemini) ImageProcessing(url string) (*[]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Image fetch close error: %v", err)
		}
	}(resp.Body)

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	switch imgType := http.DetectContentType(imgData); imgType {
	case "image/jpeg":
		return &imgData, "jpeg", nil
	case "image/png":
		return &imgData, "png", nil
	case "image/gif":
		imgTemp, err := gif.Decode(bytes.NewReader(imgData))
		if err != nil {
			return nil, "", err
		}
		buf := new(bytes.Buffer)
		err = jpeg.Encode(buf, imgTemp, nil)
		if err != nil {
			return nil, "", err
		}
		imgData = buf.Bytes()

		return &imgData, "jpeg", nil
	default:
		return nil, "", fmt.Errorf("unsupported image type: %s", imgType)
	}
}

func (g *Gemini) DeleteExpiredCache(expiration int64, interval int64) {
	type RMap struct {
		Data      []cqcode.ArrayMessage
		OriginStr string
		Time      int64
	}

	type HMap struct {
		History []*genai.Content
		Time    int64
	}

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
