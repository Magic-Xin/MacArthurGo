package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	binglib "github.com/Harry-zklcdc/bing-lib"
	"github.com/google/generative-ai-go/genai"
	"github.com/sashabaranov/go-openai"
	"github.com/vinta/pangu"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	Enabled  bool
	Args     []string
	apiKey   string
	ReplyMap sync.Map
}

type NewBing struct {
	Enabled bool
	Args    []string
	model   string
}

type ChatAI struct {
	essentials.Plugin
	ChatGPT      *ChatGPT
	QWen         *QWen
	Gemini       *Gemini
	NewBing      *NewBing
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
	newBing := NewBing{
		Enabled: base.Config.Plugins.ChatAI.NewBing.Enable,
		Args:    base.Config.Plugins.ChatAI.NewBing.Args,
		model:   base.Config.Plugins.ChatAI.NewBing.Model,
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
	if newBing.Enabled {
		args = append(args, newBing.Args...)
	}

	chatAI := ChatAI{
		Plugin: essentials.Plugin{
			Name:    "ChatAI",
			Enabled: base.Config.Plugins.ChatAI.Enable,
			Args:    args,
		},
		ChatGPT:      &chatGPT,
		QWen:         &qWen,
		Gemini:       &gemini,
		NewBing:      &newBing,
		groupForward: base.Config.Plugins.ChatAI.GroupForward,
		panGu:        base.Config.Plugins.ChatAI.PanGu,
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &chatAI})
}

func (c *ChatAI) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (c *ChatAI) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !c.Enabled {
		return
	}

	words := essentials.SplitArgument(ctx)
	if len(words) < 2 {
		return
	}

	message := essentials.DecodeArrayMessage(ctx)
	var (
		rmArg bool
		str   string
	)
	for _, msg := range *message {
		if msg.Type == "text" && msg.Data["text"] != nil {
			if !rmArg {
				rmArg = true
				t := msg.Data["text"].(string)
				for _, arg := range c.Args {
					t = strings.Replace(t, arg, "", -1)
				}
				msg.Data["text"] = t
				str += t + " "
				continue
			}
			str += msg.Data["text"].(string) + " "
		}
	}

	var res *string
	if essentials.CheckArgumentArray(ctx, &c.ChatGPT.Args) && c.ChatGPT.Enabled {
		res = c.ChatGPT.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(ctx, &c.QWen.Args) && c.QWen.Enabled {
		res = c.QWen.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(ctx, &c.Gemini.Args) && c.Gemini.Enabled {
		var action *[]byte
		messageID := int64((*ctx)["message_id"].(float64))
		res, action = c.Gemini.RequireAnswer(str, message, messageID)
		if action != nil {
			*send <- *action
			return
		}
	} else if essentials.CheckArgumentArray(ctx, &c.NewBing.Args) && c.NewBing.Enabled {
		*send <- *essentials.SendMsg(ctx, "NewBing 回复生成中，速度较慢请勿重复发送请求", nil, false, true)
		res = c.NewBing.RequireAnswer(str)
	} else {
		return
	}

	if res == nil {
		return
	}

	if c.panGu {
		*res = pangu.SpacingText(*res)
	}

	if (*ctx)["message_type"].(string) == "group" && c.groupForward {
		var data []structs.ForwardNode
		originStr := append([]cqcode.ArrayMessage{*cqcode.Text("@" + (*ctx)["sender"].(map[string]any)["nickname"].(string) + ": ")}, *message...)
		data = append(data, *essentials.ConstructForwardNode(&originStr), *essentials.ConstructForwardNode(&[]cqcode.ArrayMessage{*cqcode.Text(*res)}))
		*send <- *essentials.SendGroupForward(ctx, &data, "")
	} else {
		*send <- *essentials.SendMsg(ctx, *res, nil, false, false)
	}
}

func (c *ChatAI) ReceiveEcho(ctx *map[string]any, send *chan []byte) {
	if !c.Enabled {
		return
	}

	echo := (*ctx)["echo"].(string)
	split := strings.Split(echo, "|")

	if split[0] == "gemini" && (*ctx)["data"] != nil {
		contexts := (*ctx)["data"].(map[string]any)
		if (*ctx)["status"] != "ok" {
			*send <- *essentials.SendMsg(&contexts, "Gemini reply args error", nil, false, false)
			return
		}

		var res *string
		message := essentials.DecodeArrayMessage(&contexts)
		data, ok := c.Gemini.ReplyMap.Load(split[1])
		if !ok {
			log.Println("Gemini reply map load error")
			return
		}

		originMsg, originStr := data.(struct {
			Data      []cqcode.ArrayMessage
			OriginStr string
		}).Data, data.(struct {
			Data      []cqcode.ArrayMessage
			OriginStr string
		}).OriginStr

		*message = append(originMsg, *message...)
		res, _ = c.Gemini.RequireAnswer(originStr, message, 0)

		if res == nil {
			return
		}

		if c.panGu {
			*res = pangu.SpacingText(*res)
		}

		if contexts["message_type"].(string) == "group" && c.groupForward {
			var data []structs.ForwardNode
			originStr := append([]cqcode.ArrayMessage{*cqcode.Text("@" + contexts["sender"].(map[string]any)["nickname"].(string) + ": ")}, *message...)
			data = append(data, *essentials.ConstructForwardNode(&originStr), *essentials.ConstructForwardNode(&[]cqcode.ArrayMessage{*cqcode.Text(*res)}))
			*send <- *essentials.SendGroupForward(&contexts, &data, "")
		} else {
			*send <- *essentials.SendMsg(&contexts, *res, nil, false, false)
		}
	}
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

func (g *Gemini) RequireAnswer(str string, message *[]cqcode.ArrayMessage, messageID int64) (*string, *[]byte) {
	var (
		images []struct {
			Data    *[]byte
			ImgType string
		}
		prompts []genai.Part
		model   *genai.GenerativeModel
		res     string
		reply   int64
	)

	for _, msg := range *message {
		if msg.Type == "image" && msg.Data["url"] != nil {
			data, imgType, err := g.ImageProcessing(msg.Data["url"].(string))
			if err != nil {
				log.Printf("Image processing error: %v", err)
				continue
			}
			images = append(images, struct {
				Data    *[]byte
				ImgType string
			}{Data: data, ImgType: imgType})
		}
		if msg.Type == "reply" && messageID != 0 {
			reply = int64(msg.Data["id"].(float64))
		}
	}

	if reply != 0 && messageID != 0 {
		g.ReplyMap.Store(strconv.FormatInt(messageID, 10), struct {
			Data      []cqcode.ArrayMessage
			OriginStr string
		}{Data: *message, OriginStr: str})

		echo := fmt.Sprintf("gemini|%d", messageID)
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

	if len(images) != 0 {
		model = client.GenerativeModel("gemini-pro-vision")
		res = "gemini-pro-vision: "
		for _, img := range images {
			prompts = append(prompts, genai.ImageData(img.ImgType, *img.Data))
		}
	} else {
		model = client.GenerativeModel("gemini-pro")
		res = "gemini-pro: "
	}
	prompts = append(prompts, genai.Text(str))

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

	iter := model.GenerateContentStream(ctx, prompts...)
	if iter == nil {
		log.Printf("Gemini generate error: %v", err)
		res = fmt.Sprintf("Gemini generate error: %v", err)
		return &res, nil
	}

	for {
		resp, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Printf("Gemini iterator error: %v", err)
			res += fmt.Sprintf("Gemini iterator error: %v", err)
			return &res, nil
		}

		for _, cand := range resp.Candidates {
			for _, part := range cand.Content.Parts {
				res += fmt.Sprintf("%s", part)
			}
		}
	}

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

func (n *NewBing) RequireAnswer(str string) *string {
	const cookie = "1f1e33"

	c := binglib.NewChat(cookie)
	err := c.NewConversation()
	if err != nil {
		log.Printf("NewBing new conversation error: %v", err)
		res := fmt.Sprintf("NewBing new conversation error: %v", err)
		return &res
	}
	c.SetStyle(n.model)
	r, err := c.Chat("", str)
	if err != nil {
		log.Printf("NewBing chat error: %v", err)
		res := fmt.Sprintf("NewBing chat error: %v", err)
		return &res
	}

	r = n.model + ": " + regexp.MustCompile(`\(\^\d\^\)|\[\^[^\]]*\^\]`).ReplaceAllString(r, "")

	return &r
}
