package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/chatai"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"fmt"
	"strings"
	"time"

	"github.com/vinta/pangu"
)

type ChatAI struct {
	ChatGPT      *chatai.ChatGPT
	QWen         *chatai.QWen
	Gemini       *chatai.Gemini
	Github       *chatai.Github
	Args         []string
	FullArgs     []string
	groupForward bool
	panGu        bool
}

func init() {
	chatGPT := chatai.ChatGPT{
		Enabled: base.Config.Plugins.ChatAI.ChatGPT.Enable,
		Args:    base.Config.Plugins.ChatAI.ChatGPT.Args,
		Model:   base.Config.Plugins.ChatAI.ChatGPT.Model,
		ApiKey:  base.Config.Plugins.ChatAI.ChatGPT.APIKey,
	}
	qWen := chatai.QWen{
		Enabled: base.Config.Plugins.ChatAI.QWen.Enable,
		Args:    base.Config.Plugins.ChatAI.QWen.Args,
		Model:   base.Config.Plugins.ChatAI.QWen.Model,
		ApiKey:  base.Config.Plugins.ChatAI.QWen.APIKey,
	}
	gemini := chatai.Gemini{
		Enabled: base.Config.Plugins.ChatAI.Gemini.Enable,
		ArgsMap: map[string]string{
			"flash": base.Config.Plugins.ChatAI.Gemini.ArgsMap["flash"],
			"think": base.Config.Plugins.ChatAI.Gemini.ArgsMap["think"],
			"pro":   base.Config.Plugins.ChatAI.Gemini.ArgsMap["pro"],
			"image": base.Config.Plugins.ChatAI.Gemini.ArgsMap["image"],
		},
		ApiKey: base.Config.Plugins.ChatAI.Gemini.APIKey,
	}
	github := chatai.Github{
		Enabled: base.Config.Plugins.ChatAI.Github.Enable,
		ArgsMap: map[string]string{
			"4o":    base.Config.Plugins.ChatAI.Github.ArgsMap["4o"],
			"o1p":   base.Config.Plugins.ChatAI.Github.ArgsMap["o1p"],
			"o3m":   base.Config.Plugins.ChatAI.Github.ArgsMap["o3m"],
			"llama": base.Config.Plugins.ChatAI.Github.ArgsMap["llama"],
			"phi4":  base.Config.Plugins.ChatAI.Github.ArgsMap["phi4"],
			"r1":    base.Config.Plugins.ChatAI.Github.ArgsMap["r1"],
		},
		Token: base.Config.Plugins.ChatAI.Github.Token,
	}

	args := []string{"/aihelp", "/ai帮助"}

	var fullArgs []string
	fullArgs = append(fullArgs, args...)
	if chatGPT.Enabled {
		fullArgs = append(fullArgs, chatGPT.Args...)
	}
	if qWen.Enabled {
		fullArgs = append(fullArgs, qWen.Args...)
	}
	if gemini.Enabled {
		for _, v := range gemini.ArgsMap {
			fullArgs = append(fullArgs, v)
		}
	}
	if github.Enabled {
		for _, v := range github.ArgsMap {
			fullArgs = append(fullArgs, v)
		}
	}

	chatAI := ChatAI{
		ChatGPT:      &chatGPT,
		QWen:         &qWen,
		Gemini:       &gemini,
		Github:       &github,
		Args:         args,
		FullArgs:     fullArgs,
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

	//go gemini.DeleteExpiredCache(3600, 1800)
}

func (*ChatAI) ReceiveAll(essentials.SendFunc) {}

func (c *ChatAI) ReceiveMessage(incomingMessage *structs.IncomingMessageStruct, send essentials.SendFunc) {
	if !essentials.CheckArgumentArray(incomingMessage.Command, &c.FullArgs) {
		return
	}

	if len(*incomingMessage.CleanMessage) < 1 {
		return
	}

	message := *incomingMessage.CleanMessage
	textArray := essentials.SplitArgument(&message)
	str := strings.Join(textArray, " ")

	var (
		res       *[]string
		id        int64
		modelName string
	)
	if essentials.CheckArgumentArray(incomingMessage.Command, &c.ChatGPT.Args) && c.ChatGPT.Enabled {
		res = c.ChatGPT.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(incomingMessage.Command, &c.QWen.Args) && c.QWen.Enabled {
		res = c.QWen.RequireAnswer(str)
	} else if key, ok := essentials.CheckArgumentMap(incomingMessage.Command, &c.Gemini.ArgsMap); ok && c.Gemini.Enabled {
		switch key {
		case "flash":
			modelName = "gemini-2.5-flash"
		case "think":
			modelName = "gemini-2.5-flash"
		case "pro":
			modelName = "gemini-2.5-pro"
		case "image":
			modelName = "gemini-2.0-flash-exp-image-generation"
		}

		res, id = c.Gemini.RequireAnswer(incomingMessage, modelName, send)

		if id != 0 {
			storeMessage := *incomingMessage
			storeMessage.Command = modelName
			essentials.SetCache(fmt.Sprintf("%d|%s", id, "gemini"), essentials.EchoCache{Value: *incomingMessage, Time: time.Now().Unix()})
		}
	} else if key, ok := essentials.CheckArgumentMap(incomingMessage.Command, &c.Github.ArgsMap); ok && c.Github.Enabled {
		switch key {
		case "4o":
			res = c.Github.RequireAnswer(str, "gpt-4o")
		case "o1p":
			res = c.Github.RequireAnswer(str, "o1-preview")
		case "o3m":
			res = c.Github.RequireAnswer(str, "o3-mini")
		case "llama":
			res = c.Github.RequireAnswer(str, "Meta-Llama-3.1-405B-Instruct")
		case "phi4":
			res = c.Github.RequireAnswer(str, "Phi-4")
		case "r1":
			res = c.Github.RequireAnswer(str, "DeepSeek-R1")
		default:
			return
		}
	} else if essentials.CheckArgumentArray(incomingMessage.Command, &[]string{"/aihelp", "/ai帮助"}) {
		var text string
		if c.ChatGPT.Enabled {
			text += fmt.Sprintf("ChatGPT:\n%s: %s\n\n", c.ChatGPT.Model, c.ChatGPT.Args)
		}
		if c.QWen.Enabled {
			text += fmt.Sprintf("QWen:\n%s: %s\n\n", c.QWen.Model, c.QWen.Args)
		}
		if c.Gemini.Enabled {
			text += fmt.Sprintf("Gemini:\nGemini-2.5-flash: %s\nGemini-2.0-flash image-generation: %s\nGemini-2.5-flash: %s\nGemini-2.5-pro: %s\n\n",
				c.Gemini.ArgsMap["flash"], c.Gemini.ArgsMap["image"], c.Gemini.ArgsMap["think"], c.Gemini.ArgsMap["pro"])
		}
		if c.Github.Enabled {
			text += fmt.Sprintf("Github:\nChatGPT 4o: %s\nChatGPT o1-preview: %s\nChatGPT o3-mini: %s\nLlama-3.1-405B: %s\nPhi-4: %s\nDeepSeek-R1: %s\n",
				c.Github.ArgsMap["4o"], c.Github.ArgsMap["o1p"], c.Github.ArgsMap["o3m"], c.Github.ArgsMap["llama"],
				c.Github.ArgsMap["phi4"], c.Github.ArgsMap["r1"])
		}
		essentials.SendMsg(incomingMessage, text, nil, false, false, send)
		return
	} else {
		return
	}

	if res == nil {
		return
	}

	if c.panGu {
		for i, r := range *res {
			if r[:9] == "base64://" {
				continue
			}
			(*res)[i] = pangu.SpacingText(r)
		}
	}

	if incomingMessage.MessageScene == "group" && c.groupForward {
		var data []structs.OutgoingForwardedMessage

		uin := incomingMessage.SenderID
		name := incomingMessage.GroupMember.Nickname

		for _, m := range *incomingMessage.CleanMessage {
			if m.Type == "image" {
				m.Data["uri"] = essentials.ImageToBase64(m.Data["uri"].(string))
			}
		}

		data = append(data, *essentials.ConstructForwardedMessage(uin, name, incomingMessage.CleanMessage))
		for _, r := range *res {
			if r[:9] == "base64://" {
				data = append(data, *essentials.ConstructForwardedMessage(essentials.Info.UserId, essentials.Info.NickName, &[]structs.MessageSegment{*structs.Image(r)}))
			} else {
				data = append(data, *essentials.ConstructForwardedMessage(essentials.Info.UserId, essentials.Info.NickName, &[]structs.MessageSegment{*structs.Text(r)}))
			}
		}

		outgoingMessage := structs.MessageSegment{Type: "forward", Data: map[string]any{"messages": data}}
		essentials.SendMsg(incomingMessage, "", &[]structs.MessageSegment{outgoingMessage}, false, false, send)
	} else {
		var msg []structs.MessageSegment
		for _, r := range *res {
			if r[:9] == "base64://" {
				msg = append(msg, *structs.Image(r))
			} else {
				msg = append(msg, *structs.Text(r))
			}
		}
		essentials.SendMsg(incomingMessage, "", &msg, false, false, send)
	}
}

func (c *ChatAI) ReceiveEcho(feedbackStruct *structs.FeedbackStruct, send essentials.SendFunc) {
	if feedbackStruct.Status != "ok" {
		return
	}

	message := feedbackStruct.Data.Message
	messageSeq := message.MessageSeq

	if messageSeq == 0 {
		return
	}

	value, ok := essentials.GetCache(fmt.Sprintf("%d|%s", messageSeq, "gemini"))
	if !ok {
		return
	}

	originMessage := value.(essentials.EchoCache).Value

	res := c.Gemini.RequireEchoAnswer(originMessage.CleanMessage, &message.Segments, originMessage.Command)

	if res == nil {
		essentials.SendMsg(&originMessage, "Gemini reply args error", nil, false, false, send)
		return
	}

	if c.panGu {
		for i, r := range *res {
			if r[:9] == "base64://" {
				continue
			}
			(*res)[i] = pangu.SpacingText(r)
		}
	}

	if originMessage.MessageScene == "group" && c.groupForward {
		var data []structs.OutgoingForwardedMessage

		uin := originMessage.SenderID
		name := originMessage.GroupMember.Nickname

		for _, m := range *originMessage.CleanMessage {
			if m.Type == "image" {
				m.Data["uri"] = essentials.ImageToBase64(m.Data["uri"].(string))
			}
		}

		data = append(data, *essentials.ConstructForwardedMessage(uin, name, originMessage.CleanMessage))

		uin = message.SenderID
		name = message.GroupMember.Nickname

		for _, m := range message.Segments {
			if m.Type == "image" {
				m.Data["uri"] = essentials.ImageToBase64(m.Data["uri"].(string))
			}
		}
		data = append(data, *essentials.ConstructForwardedMessage(uin, name, &message.Segments))

		for _, r := range *res {
			if r[:9] == "base64://" {
				data = append(data, *essentials.ConstructForwardedMessage(essentials.Info.UserId, essentials.Info.NickName, &[]structs.MessageSegment{*structs.Image(r)}))
			} else {
				data = append(data, *essentials.ConstructForwardedMessage(essentials.Info.UserId, essentials.Info.NickName, &[]structs.MessageSegment{*structs.Text(r)}))
			}
		}

		outgoingMessage := structs.MessageSegment{Type: "forward", Data: map[string]any{"messages": data}}
		essentials.SendMsg(&originMessage, "", &[]structs.MessageSegment{outgoingMessage}, false, false, send)
	} else {
		var msg []structs.MessageSegment
		for _, r := range *res {
			if r[:9] == "base64://" {
				msg = append(msg, *structs.Image(r))
			} else {
				msg = append(msg, *structs.Text(r))
			}
		}
		essentials.SendMsg(&originMessage, "", &msg, false, false, send)
	}
}
