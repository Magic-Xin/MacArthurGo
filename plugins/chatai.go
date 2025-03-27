package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/chatai"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/vinta/pangu"
	"log"
	"strconv"
	"strings"
	"time"
)

type ChatAI struct {
	ChatGPT      *chatai.ChatGPT
	QWen         *chatai.QWen
	Gemini       *chatai.Gemini
	Github       *chatai.Github
	Args         []string
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
	if chatGPT.Enabled {
		args = append(args, chatGPT.Args...)
	}
	if qWen.Enabled {
		args = append(args, qWen.Args...)
	}
	if gemini.Enabled {
		for _, v := range gemini.ArgsMap {
			args = append(args, v)
		}
	}
	if github.Enabled {
		for _, v := range github.ArgsMap {
			args = append(args, v)
		}
	}

	chatAI := ChatAI{
		ChatGPT:      &chatGPT,
		QWen:         &qWen,
		Gemini:       &gemini,
		Github:       &github,
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

	//go gemini.DeleteExpiredCache(3600, 1800)
}

func (*ChatAI) ReceiveAll(chan<- *[]byte) {}

func (c *ChatAI) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	if !essentials.CheckArgumentArray(messageStruct.Command, &c.Args) {
		return
	}

	if len(*messageStruct.CleanMessage) < 1 {
		return
	}

	message := *messageStruct.CleanMessage
	textArray := essentials.SplitArgument(&message)
	str := strings.Join(textArray, " ")

	var (
		res  *[]string
		echo string
	)
	if essentials.CheckArgumentArray(messageStruct.Command, &c.ChatGPT.Args) && c.ChatGPT.Enabled {
		res = c.ChatGPT.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(messageStruct.Command, &c.QWen.Args) && c.QWen.Enabled {
		res = c.QWen.RequireAnswer(str)
	} else if key, ok := essentials.CheckArgumentMap(messageStruct.Command, &c.Gemini.ArgsMap); ok && c.Gemini.Enabled {
		var action *[]byte
		messageID := messageStruct.MessageId
		switch key {
		case "flash":
			res, action = c.Gemini.RequireAnswer(&message, messageID, "gemini-2.0-flash-exp")
		case "think":
			res, action = c.Gemini.RequireAnswer(&message, messageID, "gemini-2.0-flash-thinking-exp-01-21")
		case "pro":
			res, action = c.Gemini.RequireAnswer(&message, messageID, "gemini-2.5-pro-exp-03-25")
		case "image":
			res, action = c.Gemini.RequireAnswer(&message, messageID, "gemini-2.0-flash-exp-image-generation")
		}
		if action != nil {
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageID, 10), value)
			send <- action
			return
		}
		echo = "geminisend|" + strconv.FormatInt(messageID, 10)
	} else if key, ok := essentials.CheckArgumentMap(messageStruct.Command, &c.Github.ArgsMap); ok && c.Github.Enabled {
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
	} else if essentials.CheckArgumentArray(messageStruct.Command, &[]string{"/aihelp", "/ai帮助"}) {
		var text string
		if c.ChatGPT.Enabled {
			text += fmt.Sprintf("ChatGPT:\n%s: %s\n\n", c.ChatGPT.Model, c.ChatGPT.Args)
		}
		if c.QWen.Enabled {
			text += fmt.Sprintf("QWen:\n%s: %s\n\n", c.QWen.Model, c.QWen.Args)
		}
		if c.Gemini.Enabled {
			text += fmt.Sprintf("Gemini:\nGemini-2.0-flash-exp: %s\nGemini-2.0-flash-thinking-exp: %s\nGemini-2.0-pro-exp: %s\n\n",
				c.Gemini.ArgsMap["flash"], c.Gemini.ArgsMap["think"], c.Gemini.ArgsMap["pro"])
		}
		if c.Github.Enabled {
			text += fmt.Sprintf("Github:\nChatGPT 4o: %s\nChatGPT o1-preview: %s\nChatGPT o3-mini: %s\nLlama-3.1-405B: %s\nPhi-4: %s\nDeepSeek-R1: %s\n",
				c.Github.ArgsMap["4o"], c.Github.ArgsMap["o1p"], c.Github.ArgsMap["o3m"], c.Github.ArgsMap["llama"],
				c.Github.ArgsMap["phi4"], c.Github.ArgsMap["r1"])
		}
		send <- essentials.SendMsg(messageStruct, text, nil, false, false, "")
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

	if messageStruct.MessageType == "group" && c.groupForward {
		var data []structs.ForwardNode
		uin := strconv.FormatInt(messageStruct.UserId, 10)
		name := messageStruct.Sender.Nickname

		for _, m := range *messageStruct.CleanMessage {
			if m.Type == "image" {
				m.Data["file"] = essentials.ImageToBase64(m.Data["file"].(string))
			}
		}

		data = append(data, *essentials.ConstructForwardNode(uin, name, messageStruct.CleanMessage))
		for _, r := range *res {
			if r[:9] == "base64://" {
				data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Image(r)}))
			} else {
				data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(r)}))
			}
		}
		send <- essentials.SendGroupForward(messageStruct, &data, echo)
	} else {
		var msg []cqcode.ArrayMessage
		for _, r := range *res {
			if r[:9] == "base64://" {
				msg = append(msg, *cqcode.Image(r))
			} else {
				msg = append(msg, *cqcode.Text(r))
			}
		}
		send <- essentials.SendMsg(messageStruct, "", &msg, false, false, "")
	}
}

func (c *ChatAI) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, send chan<- *[]byte) {
	split := strings.Split(echoMessageStruct.Echo, "|")

	if split[0] == "gemini" && !cmp.Equal(echoMessageStruct.Data, struct{}{}) {
		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Gemini get cache error")
		}
		originMessage := value.(essentials.EchoCache).Value
		if echoMessageStruct.Status != "ok" {
			send <- essentials.SendMsg(&originMessage, "Gemini reply args error", nil, false, false, "")
			return
		}

		var res *[]string
		message := echoMessageStruct.Data.Message
		res = c.Gemini.RequireEchoAnswer(originMessage.CleanMessage, &message, split[2])

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

		if originMessage.MessageType == "group" && c.groupForward {
			var data []structs.ForwardNode

			for _, m := range echoMessageStruct.Data.Message {
				if m.Type == "image" {
					m.Data["file"] = essentials.ImageToBase64(m.Data["file"].(string))
				}
			}
			data = append(data, *essentials.ConstructForwardNode(strconv.FormatInt(echoMessageStruct.Data.Sender.UserId, 10), echoMessageStruct.Data.Nickname, &echoMessageStruct.Data.Message))

			for _, m := range *originMessage.CleanMessage {
				if m.Type == "image" {
					m.Data["file"] = essentials.ImageToBase64(m.Data["file"].(string))
				}
			}
			data = append(data, *essentials.ConstructForwardNode(strconv.FormatInt(originMessage.UserId, 10), originMessage.Sender.Nickname, originMessage.CleanMessage))

			for _, r := range *res {
				if r[:9] == "base64://" {
					data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Image(r)}))
				} else {
					data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(r)}))
				}
			}

			send <- essentials.SendGroupForward(&originMessage, &data, "")
		} else {
			var msg []cqcode.ArrayMessage
			for _, r := range *res {
				if r[:9] == "base64://" {
					msg = append(msg, *cqcode.Image(r))
				} else {
					msg = append(msg, *cqcode.Text(r))
				}
			}
			send <- essentials.SendMsg(&originMessage, "", &msg, false, false, "")
		}
	}
	return
}
