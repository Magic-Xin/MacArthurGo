package websocket

import (
	_struct "MacArthurGo/struct"
	"MacArthurGo/websocket/plugins"
	"encoding/json"
	"log"
	"strings"
)

func MessageFactory(message []byte, c *Client) {
	var i interface{}
	err := json.Unmarshal(message, &i)
	if err != nil {
		return
	}
	msg := i.(map[string]interface{})
	if msg["post_type"] == "message" {
		messageType := msg["message_type"].(string)
		groupId := -1
		userId := -1
		tempMessage := ""
		if msg["group_id"] == nil {
			userId = int(msg["user_id"].(float64))
		} else {
			groupId = int(msg["group_id"].(float64))
		}

		words := strings.Fields(msg["raw_message"].(string))

		if int(msg["user_id"].(float64)) == 649362775 {
			if msg["raw_message"] == "测试" || msg["raw_message"] == "/test" {
				tempMessage = "活着呢"
			}
		}

		switch words[0] {
		case "/test":
			tempMessage = "活着呢"
		case "/roll":
			tempMessage = plugins.Roll(words)
		}

		if tempMessage != "" {
			sendMsg := _struct.Message{MessageType: messageType, UserId: userId, GroupId: groupId, Message: tempMessage}
			act := _struct.Action{Action: "send_msg", Params: sendMsg}
			jsonMsg, _ := json.Marshal(act)
			log.Println(string(jsonMsg))
			c.Send <- jsonMsg
		}
	}
}
