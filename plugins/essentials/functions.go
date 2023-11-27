package essentials

import (
	_struct "MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"strings"
)

func SendAction(action string, params any, echo string) *[]byte {
	if action == "" {
		return nil
	}

	act := _struct.Action{Action: action, Params: params}
	eAct := _struct.EchoAction{Action: act, Echo: echo}
	jsonMsg, _ := json.Marshal(eAct)

	return &jsonMsg
}

func SendMsg(ctx *map[string]any, message string, at bool, reply bool) *[]byte {
	if message == "" || ctx == nil {
		return nil
	}

	messageArray := []string{message}

	if at && (*ctx)["message_type"] == "group" {
		uid := int64((*ctx)["user_id"].(float64))
		messageArray = append([]string{cqcode.At(uid)}, messageArray...)
	}

	//FIXME
	if reply {
		msgId := int64((*ctx)["message_id"].(float64))
		messageArray = append([]string{cqcode.Reply(msgId)}, messageArray...)
	}

	return constructMessage(ctx, strings.Join(messageArray, ""))
}

func SendPoke(ctx *map[string]any, uid int64) *[]byte {
	message := cqcode.Poke(uid)

	return constructMessage(ctx, message)
}

func SendMusic(ctx *map[string]any, urlType string, id int64) *[]byte {
	message := cqcode.Music(urlType, id)

	return constructMessage(ctx, message)
}

func SendPrivateForward(ctx *map[string]any, data *[]_struct.ForwardNode) *[]byte {
	params := _struct.PrivateForward{
		UserId:   int64((*ctx)["sender"].(map[string]any)["user_id"].(float64)),
		Messages: *data,
	}
	act := _struct.Action{Action: "send_private_forward_msg", Params: params}

	jsonMsg, _ := json.Marshal(act)
	return &jsonMsg
}

func SendGroupForward(ctx *map[string]any, data *[]_struct.ForwardNode) *[]byte {
	params := _struct.GroupForward{
		GroupId:  int64((*ctx)["group_id"].(float64)),
		Messages: *data,
	}
	act := _struct.Action{Action: "send_group_forward_msg", Params: params}

	jsonMsg, _ := json.Marshal(act)
	return &jsonMsg
}

func ConstructForwardNode(data *string, name string, uin int64) *_struct.ForwardNode {
	node := _struct.NewForwardNode(name, uin)
	node.Data.Content = *data
	return node
}

func CheckArgument(ctx *map[string]any, arg string) bool {
	return strings.Fields((*ctx)["raw_message"].(string))[0] == arg
}

func SplitArgument(ctx *map[string]any) []string {
	return strings.Fields((*ctx)["raw_message"].(string))
}

func constructMessage(ctx *map[string]any, message string) *[]byte {
	messageType := (*ctx)["message_type"].(string)
	var (
		userId  int64
		groupId int64
	)
	if (*ctx)["group_id"] == nil {
		userId = int64((*ctx)["sender"].(map[string]any)["user_id"].(float64))
	} else {
		groupId = int64((*ctx)["group_id"].(float64))
	}

	msg := _struct.Message{MessageType: messageType, UserId: userId, GroupId: groupId, Message: message}
	act := _struct.Action{Action: "send_msg", Params: msg}
	jsonMsg, _ := json.Marshal(act)
	return &jsonMsg
}
