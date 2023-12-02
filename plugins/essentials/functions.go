package essentials

import (
	_struct "MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"regexp"
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

func SendPrivateForward(ctx *map[string]any, data *[]_struct.ForwardNode, echo string) *[]byte {
	params := _struct.PrivateForward{
		UserId:   int64((*ctx)["sender"].(map[string]any)["user_id"].(float64)),
		Messages: *data,
	}

	return SendAction("send_private_forward_msg", params, echo)
}

func SendGroupForward(ctx *map[string]any, data *[]_struct.ForwardNode, echo string) *[]byte {
	params := _struct.GroupForward{
		GroupId:  int64((*ctx)["group_id"].(float64)),
		Messages: *data,
	}

	return SendAction("send_group_forward_msg", params, echo)
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

func GetUniversalImgURL(url string) (string, string) {
	pattern := regexp.MustCompile(`^https?://(c2cpicdw|gchat)\.qpic\.cn/(offpic|gchatpic)_new/`)
	if pattern.MatchString(url) {
		url = strings.Replace(url, "/c2cpicdw.qpic.cn/offpic_new/", "/gchat.qpic.cn/gchatpic_new/", 1)
		url = regexp.MustCompile(`/\d+/+\d+-\d+-`).ReplaceAllString(url, "/0/0-0-")
		url = strings.TrimSuffix(url, "?.*$")
	}

	uidPattern := regexp.MustCompile(`/0/0-0-(\w+)/`)
	match := uidPattern.FindAllStringSubmatch(url, -1)
	if match != nil {
		return url, match[0][1]
	}

	return url, ""
}

func HandleBannedHostsArray(str *[]string) *[]string {
	bannedHosts := []string{"danbooru.donmai.us", "konachan.com"}
	for _, s := range *str {
		s = strings.Replace(s, "//", "//\u200B", -1)
		for _, host := range bannedHosts {
			s = strings.Replace(s, host, strings.Replace(host, ".", ".\u200B", -1), -1)
		}
	}
	return str
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
