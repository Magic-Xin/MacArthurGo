package websocket

import (
	"MacArthurGo/struct"
	"MacArthurGo/struct/cqcode"
	"MacArthurGo/websocket/plugins"
	"encoding/json"
	"github.com/gookit/config/v2"
	"log"
	"strings"
)

func MessageFactory(msg *[]byte, c *Client) {
	var (
		i        any
		message  *[]byte
		messages []*[]byte
	)
	err := json.Unmarshal(*msg, &i)
	if err != nil {
		return
	}

	ctx := i.(map[string]any)
	if ctx["post_type"] == "message" {
		words := strings.Fields(ctx["raw_message"].(string))
		if len(words) < 1 {
			return
		}

		switch words[0] {
		case "/test":
			message = sendMsg(&ctx, "活着呢", false, true)
		case "/poke":
			if config.Bool("plugins.poke.enable") {
				message = sendPoke(&ctx, plugins.Poke(&ctx, &words))
			}
		case "/roll":
			if config.Bool("plugins.roll.enable") {
				message = sendMsg(&ctx, plugins.Roll(&words), false, true)
			}
		case "/chatgpt":
			if config.Bool("plugins.chatGPT.enable") {
				res, err := plugins.ChatGPT(&words)
				if err != nil {
					break
				}
				message = sendMsg(&ctx, res, false, true)
			}
		}

		if message == nil && config.Bool("plugins.music.enable") {
			urlType, id, ok := plugins.Music(ctx["raw_message"].(string))
			if ok {
				message = sendMusic(&ctx, urlType, id)
			}
		}

		if config.Bool("plugins.picSearch.enable") {
			var (
				str *[]string
				btr *[]byte
			)
			str, btr = plugins.PicSearch(ctx["raw_message"].(string), false)

			if str != nil {
				for _, s := range *str {
					messages = append(messages, sendMsg(&ctx, s, false, false))
				}
			}
			if btr != nil {
				c.Send <- *btr
			}
		}
	}

	if ctx["echo"] != nil {
		switch ctx["echo"].(string) {
		case "picSearch":
			if ctx["data"].(map[string]any) != nil {
				ctx = ctx["data"].(map[string]any)
				if ctx["message"] != nil {
					str, _ := plugins.PicSearch(ctx["message"].(string), true)
					if str != nil {
						for _, s := range *str {
							messages = append(messages, sendMsg(&ctx, s, false, false))
						}
					}
				}
			}
		}
	}

	if message != nil {
		log.Println(string(*message))
		c.Send <- *message
	}
	if len(messages) > 0 {
		for _, m := range messages {
			log.Println(string(*m))
			c.Send <- *m
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

func sendMusic(ctx *map[string]any, urlType string, id int64) *[]byte {
	message := cqcode.Music(urlType, id)

	return constructMessage(ctx, message)
}

func constructMessage(ctx *map[string]any, message string) *[]byte {
	messageType := (*ctx)["message_type"].(string)
	groupId := -1
	userId := -1
	if (*ctx)["group_id"] == nil {
		userId = int((*ctx)["sender"].(map[string]any)["user_id"].(float64))
	} else {
		groupId = int((*ctx)["group_id"].(float64))
	}

	msg := _struct.Message{MessageType: messageType, UserId: userId, GroupId: groupId, Message: message}
	act := _struct.Action{Action: "send_msg", Params: msg}
	jsonMsg, _ := json.Marshal(act)
	return &jsonMsg
}
