package websocket

import (
	_struct "MacArthurGo/struct"
	"MacArthurGo/struct/cqcode"
	"MacArthurGo/websocket/plugins"
	"encoding/json"
	"log"
	"strings"
)

func MessageFactory(message *[]byte, c *Client) {
	var i any
	err := json.Unmarshal(*message, &i)
	if err != nil {
		return
	}

	ctx := i.(map[string]any)
	if ctx["post_type"] == "message" {
		var message *[]byte
		words := strings.Fields(ctx["raw_message"].(string))
		if len(words) < 1 {
			return
		}

		switch words[0] {
		case "/test":
			message = sendMsg(&ctx, "活着呢", false, true)
		case "/poke":
			message = sendPoke(&ctx, plugins.Poke(&ctx, &words))
		case "/roll":
			message = sendMsg(&ctx, plugins.Roll(&words), false, true)
		case "/chatgpt":
			res, err := plugins.ChatGPT(&words)
			if err != nil {
				break
			}
			message = sendMsg(&ctx, res, false, true)
		}

		if message != nil {
			c.Send <- *message
		}
	}
}

func sendMsg(ctx *map[string]any, message string, at bool, reply bool) *[]byte {
	if message == "" || ctx == nil {
		return nil
	}

	messageArray := []string{message}

	if at && (*ctx)["message_type"] == "group" {
		uid := int64((*ctx)["user_id"].(float64))
		messageArray = append([]string{cqcode.At(uid)}, messageArray...)
	}

	if reply {
		msgId := int64((*ctx)["message_id"].(float64))
		messageArray = append([]string{cqcode.Reply(msgId)}, messageArray...)
	}

	return constructMessage(ctx, strings.Join(messageArray, ""))
}

func sendPoke(ctx *map[string]any, uid int64) *[]byte {
	message := cqcode.Poke(uid)

	return constructMessage(ctx, message)
}

func constructMessage(ctx *map[string]any, message string) *[]byte {
	messageType := (*ctx)["message_type"].(string)
	groupId := -1
	userId := -1
	if (*ctx)["group_id"] == nil {
		userId = int((*ctx)["user_id"].(float64))
	} else {
		groupId = int((*ctx)["group_id"].(float64))
	}

	msg := _struct.Message{MessageType: messageType, UserId: userId, GroupId: groupId, Message: message}
	act := _struct.Action{Action: "send_msg", Params: msg}
	jsonMsg, _ := json.Marshal(act)
	log.Println(string(jsonMsg))
	return &jsonMsg
}
