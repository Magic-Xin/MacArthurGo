package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/chatai"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
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
		Args:    base.Config.Plugins.ChatAI.Gemini.Args,
		ApiKey:  base.Config.Plugins.ChatAI.Gemini.APIKey,
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
	} else if essentials.CheckArgumentArray(messageStruct.Command, &c.Gemini.Args) && c.Gemini.Enabled {
		var action *[]byte
		messageID := messageStruct.MessageId
		if len(c.Gemini.Args) < 2 {
			res, action = c.Gemini.RequireAnswer(str, &message, messageID, "gemini-2.0-flash-exp", 0)
		} else {
			if messageStruct.Command == c.Gemini.Args[0] {
				res, action = c.Gemini.RequireAnswer(str, &message, messageID, "gemini-2.0-flash-exp", 0)
			} else {
				res, action = c.Gemini.RequireAnswer(str, &message, messageID, "gemini-2.0-flash-thinking-exp", 0)
			}
		}

		if action != nil {
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageID, 10), value)
			send <- action
			return
		}
		echo = "geminisend|" + strconv.FormatInt(messageID, 10)
	} else {
		return
	}

	if res == nil {
		return
	}

	if c.panGu {
		for i, r := range *res {
			(*res)[i] = pangu.SpacingText(r)
		}
	}

	if messageStruct.MessageType == "group" && c.groupForward {
		var data []structs.ForwardNode
		uin := strconv.FormatInt(messageStruct.UserId, 10)
		name := messageStruct.Sender.Nickname
		data = append(data, *essentials.ConstructForwardNode(uin, name, messageStruct.CleanMessage))
		for _, r := range *res {
			data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(r)}))
		}
		send <- essentials.SendGroupForward(messageStruct, &data, echo)
	} else {
		text := strings.Join(*res, "\n")
		send <- essentials.SendMsg(messageStruct, text, nil, false, false, "")
	}
}

func (c *ChatAI) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, send chan<- *[]byte) {
	split := strings.Split(echoMessageStruct.Echo, "|")

	if split[0] == "gemini" && !cmp.Equal(echoMessageStruct.Data, struct{}{}) {
		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Gemini get cache error")
		}
		originCtx := value.(essentials.EchoCache).Value
		if echoMessageStruct.Status != "ok" {
			send <- essentials.SendMsg(&originCtx, "Gemini reply args error", nil, false, false, "")
			return
		}

		data, ok := c.Gemini.ReplyMap.Load(split[1])
		if !ok {
			log.Println("Gemini reply map load error")
			return
		}

		originStr := data.(chatai.RMap).OriginStr
		originMessage := data.(chatai.RMap).Data

		var res *[]string
		message := echoMessageStruct.Data.Message
		messageId, err := strconv.ParseInt(split[1], 10, 64)
		if err != nil {
			log.Printf("Echo id parse error: %v", err)
			return
		}
		res, _ = c.Gemini.RequireAnswer(originStr, &message, messageId, split[2], echoMessageStruct.Data.MessageId)

		if res == nil {
			return
		}

		echo := "geminisend|" + split[1]

		if c.panGu {
			for i, r := range *res {
				(*res)[i] = pangu.SpacingText(r)
			}
		}

		if originCtx.MessageType == "group" && c.groupForward {
			var data []structs.ForwardNode
			uin := strconv.FormatInt(originCtx.UserId, 10)
			name := originCtx.Sender.Nickname
			originMessage = append(originMessage, message...)
			data = append(data, *essentials.ConstructForwardNode(uin, name, &originMessage))
			for _, r := range *res {
				data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(r)}))
			}
			send <- essentials.SendGroupForward(&originCtx, &data, echo)
		} else {
			text := strings.Join(*res, "\n")
			send <- essentials.SendMsg(&originCtx, text, nil, false, false, echo)
		}
	} else if split[0] == "geminisend" {
		// TODO: Fix this
		//key, err := strconv.ParseInt(split[1], 10, 64)
		//if err != nil {
		//	log.Printf("Gemini send id parse error: %v", err)
		//	return nil
		//}
		//value, ok := c.Gemini.HistoryMap.Load(key)
		//if !ok {
		//	log.Println("Gemini history map load error")
		//	return nil
		//}
		//c.Gemini.HistoryMap.Store(echoMessageStruct.Data.MessageId, value)
	}
	return
}
