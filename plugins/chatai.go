package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/chatai"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"fmt"
	"log"
	"strconv"
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

func registerChatAI() error {
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
		Name:    "chatAI",
		Enabled: base.Config.Plugins.ChatAI.Enable,
		Args:    args,
		Handler: &chatAI,
	}

	//go gemini.DeleteExpiredCache(3600, 1800)
	return essentials.Register(plugin)
}

func (c *ChatAI) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- []byte) {
	if !essentials.CheckArgumentArray(messageStruct.Command, c.FullArgs) {
		return
	}

	if len(messageStruct.CleanMessage) < 1 {
		return
	}

	message := messageStruct.CleanMessage
	textArray := essentials.SplitArgument(message)
	str := strings.Join(textArray, " ")

	var (
		res  []string
		echo string
	)
	if essentials.CheckArgumentArray(messageStruct.Command, c.ChatGPT.Args) && c.ChatGPT.Enabled {
		res = c.ChatGPT.RequireAnswer(str)
	} else if essentials.CheckArgumentArray(messageStruct.Command, c.QWen.Args) && c.QWen.Enabled {
		res = c.QWen.RequireAnswer(str)
	} else if key, ok := essentials.CheckArgumentMap(messageStruct.Command, c.Gemini.ArgsMap); ok && c.Gemini.Enabled {
		var action []byte
		messageID := messageStruct.MessageId
		switch key {
		case "flash":
			res, action = c.Gemini.RequireAnswer(message, messageID, chatai.GeminiFlashModel)
		case "pro":
			res, action = c.Gemini.RequireAnswer(message, messageID, chatai.GeminiProModel)
		case "image":
			res, action = c.Gemini.RequireAnswer(message, messageID, chatai.GeminiImageModel)
		}
		if action != nil {
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageID, 10), value)
			send <- action
			return
		}
		echo = "geminisend|" + strconv.FormatInt(messageID, 10)
		if key == "flash" || key == "pro" {
			c.sendGeminiTextResponse(messageStruct, res, echo, send)
			return
		}
	} else if key, ok := essentials.CheckArgumentMap(messageStruct.Command, c.Github.ArgsMap); ok && c.Github.Enabled {
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
	} else if essentials.CheckArgumentArray(messageStruct.Command, []string{"/aihelp", "/ai帮助"}) {
		var text string
		if c.ChatGPT.Enabled {
			text += fmt.Sprintf("ChatGPT:\n%s: %s\n\n", c.ChatGPT.Model, c.ChatGPT.Args)
		}
		if c.QWen.Enabled {
			text += fmt.Sprintf("QWen:\n%s: %s\n\n", c.QWen.Model, c.QWen.Args)
		}
		if c.Gemini.Enabled {
			text += fmt.Sprintf("Gemini:\n%s: %s\n%s: %s\n%s: %s\nThinking: %s/%s 后可选 low、mid/medium、high，默认 medium；仅显示 answer，并标注思考等级。\n\n",
				chatai.GeminiFlashModel, c.Gemini.ArgsMap["flash"], chatai.GeminiImageModel, c.Gemini.ArgsMap["image"], chatai.GeminiProModel, c.Gemini.ArgsMap["pro"], c.Gemini.ArgsMap["pro"], c.Gemini.ArgsMap["flash"])
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
		for i, r := range res {
			if strings.HasPrefix(r, "base64://") {
				continue
			}
			res[i] = pangu.SpacingText(r)
		}
	}

	if messageStruct.MessageType == "group" && c.groupForward {
		var data []structs.ForwardNode
		uin := strconv.FormatInt(messageStruct.UserId, 10)
		name := messageStruct.Sender.Nickname
		botID, botName := essentials.Info.Account()

		for _, m := range messageStruct.CleanMessage {
			if m.Type == "image" {
				if url, ok := m.Data["url"].(string); ok {
					m.Data["file"] = essentials.ImageToBase64(url)
				} else {
					log.Println("Image get url error")
				}
			}
		}

		data = append(data, *essentials.ConstructForwardNode(uin, name, messageStruct.CleanMessage))
		for _, r := range res {
			if strings.HasPrefix(r, "base64://") {
				data = append(data, *essentials.ConstructForwardNode(botID, botName, []cqcode.ArrayMessage{*cqcode.Image(r)}))
			} else {
				data = append(data, *essentials.ConstructForwardNode(botID, botName, []cqcode.ArrayMessage{*cqcode.Text(r)}))
			}
		}
		send <- essentials.SendGroupForward(messageStruct, data, echo)
	} else {
		var msg []cqcode.ArrayMessage
		for _, r := range res {
			if strings.HasPrefix(r, "base64://") {
				msg = append(msg, *cqcode.Image(r))
			} else {
				msg = append(msg, *cqcode.Text(r))
			}
		}
		send <- essentials.SendMsg(messageStruct, "", msg, false, false, "")
	}
}

func (c *ChatAI) sendGeminiTextResponse(message *structs.MessageStruct, responses []string, echo string, send chan<- []byte, quoted ...*structs.MessageStruct) {
	if len(responses) == 0 {
		return
	}
	texts := make([]string, len(responses))
	for i, response := range responses {
		texts[i] = response
		if c.panGu {
			texts[i] = pangu.SpacingText(response)
		}
	}
	if message.MessageType == "group" && c.groupForward {
		botID, botName := essentials.Info.Account()
		nodes := make([]structs.ForwardNode, 0, len(texts)+len(quoted)+1)
		for _, source := range append(quoted, message) {
			if source == nil {
				continue
			}
			content := geminiForwardSourceContent(source.CleanMessage)
			if len(content) == 0 {
				continue
			}
			name := source.Sender.Card
			if name == "" {
				name = source.Sender.Nickname
			}
			uin := strconv.FormatInt(source.UserId, 10)
			if name == "" {
				name = uin
			}
			nodes = append(nodes, *essentials.ConstructForwardNode(uin, name, content))
		}
		for _, text := range texts {
			nodes = append(nodes, *essentials.ConstructForwardNode(botID, botName, []cqcode.ArrayMessage{*cqcode.Text(text)}))
		}
		send <- essentials.SendGroupForward(message, nodes, echo)
		return
	}
	send <- essentials.SendMsg(message, strings.Join(texts, "\n\n"), nil, false, false, echo)
}

func geminiForwardSourceContent(message []cqcode.ArrayMessage) []cqcode.ArrayMessage {
	content := make([]cqcode.ArrayMessage, 0, len(message))
	for _, segment := range message {
		// The referenced message is included as its own node, so a reply segment
		// inside the forwarded prompt is redundant and can reference stale IDs.
		if segment.Type == "reply" {
			continue
		}
		if segment.Type == "text" {
			text, ok := segment.Data["text"].(string)
			if !ok || strings.TrimSpace(text) == "" {
				continue
			}
		}
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		if segment.Type == "image" {
			if url, ok := data["url"].(string); ok && url != "" {
				if encoded := essentials.ImageToBase64(url); encoded != nil {
					data["file"] = *encoded
				} else {
					data["file"] = url
				}
			}
		}
		content = append(content, cqcode.ArrayMessage{Type: segment.Type, Data: data})
	}
	return content
}

func geminiQuotedSource(echo *structs.EchoMessageStruct) *structs.MessageStruct {
	message := &structs.MessageStruct{UserId: echo.Data.Sender.UserId, CleanMessage: echo.Data.Message}
	message.Sender.Card = echo.Data.Sender.Card
	message.Sender.Nickname = echo.Data.Sender.Nickname
	if message.Sender.Nickname == "" {
		message.Sender.Nickname = echo.Data.Nickname
	}
	return message
}

func (c *ChatAI) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, send chan<- []byte) {
	split := strings.Split(echoMessageStruct.Echo, "|")
	if len(split) < 3 || split[0] != "gemini" {
		return
	}

	if echoMessageStruct.Data.Message != nil || echoMessageStruct.Status != "ok" {
		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Gemini get cache error")
			return
		}
		originMessage := value.Value
		if echoMessageStruct.Status != "ok" {
			send <- essentials.SendMsg(&originMessage, "Gemini reply args error", nil, false, false, "")
			return
		}

		var res []string
		message := echoMessageStruct.Data.Message
		res = c.Gemini.RequireEchoAnswer(originMessage.CleanMessage, message, split[2])

		if res == nil {
			return
		}
		if chatai.IsGeminiTextModel(split[2]) {
			c.sendGeminiTextResponse(&originMessage, res, "", send, geminiQuotedSource(echoMessageStruct))
			return
		}

		if c.panGu {
			for i, r := range res {
				if strings.HasPrefix(r, "base64://") {
					continue
				}
				res[i] = pangu.SpacingText(r)
			}
		}

		if originMessage.MessageType == "group" && c.groupForward {
			var data []structs.ForwardNode
			botID, botName := essentials.Info.Account()

			for _, m := range echoMessageStruct.Data.Message {
				if m.Type == "image" {
					if url, ok := m.Data["url"].(string); ok {
						m.Data["file"] = essentials.ImageToBase64(url)
					} else {
						log.Println("Image get url error")
					}
				}
			}
			data = append(data, *essentials.ConstructForwardNode(strconv.FormatInt(echoMessageStruct.Data.Sender.UserId, 10), echoMessageStruct.Data.Nickname, echoMessageStruct.Data.Message))

			for _, m := range originMessage.CleanMessage {
				if m.Type == "image" {
					if url, ok := m.Data["url"].(string); ok {
						m.Data["file"] = essentials.ImageToBase64(url)
					} else {
						log.Println("Image get url error")
					}
				}
			}
			data = append(data, *essentials.ConstructForwardNode(strconv.FormatInt(originMessage.UserId, 10), originMessage.Sender.Nickname, originMessage.CleanMessage))

			for _, r := range res {
				if strings.HasPrefix(r, "base64://") {
					data = append(data, *essentials.ConstructForwardNode(botID, botName, []cqcode.ArrayMessage{*cqcode.Image(r)}))
				} else {
					data = append(data, *essentials.ConstructForwardNode(botID, botName, []cqcode.ArrayMessage{*cqcode.Text(r)}))
				}
			}

			send <- essentials.SendGroupForward(&originMessage, data, "")
		} else {
			var msg []cqcode.ArrayMessage
			for _, r := range res {
				if strings.HasPrefix(r, "base64://") {
					msg = append(msg, *cqcode.Image(r))
				} else {
					msg = append(msg, *cqcode.Text(r))
				}
			}
			send <- essentials.SendMsg(&originMessage, "", msg, false, false, "")
		}
	}
}
