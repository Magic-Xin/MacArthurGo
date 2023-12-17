package essentials

import (
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func SendAction(action string, params any, echo string) *[]byte {
	if action == "" {
		return nil
	}

	act := structs.Action{Action: action, Params: params, Echo: echo}
	jsonMsg, _ := json.Marshal(act)

	return &jsonMsg
}

func SendMsg(ctx *map[string]any, message string, messageArray *[]cqcode.ArrayMessage, at bool, reply bool) *[]byte {
	if (message == "" && messageArray == nil) || ctx == nil {
		return nil
	}

	arrayMessage := []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": message}}}
	if messageArray != nil {
		arrayMessage = append(arrayMessage, *messageArray...)
	}

	if at && (*ctx)["message_type"] == "group" {
		uid := strconv.FormatInt(int64((*ctx)["user_id"].(float64)), 10)
		arrayMessage = append([]cqcode.ArrayMessage{*cqcode.At(uid)}, arrayMessage...)
	}
	if reply {
		arrayMessage = append([]cqcode.ArrayMessage{*cqcode.Reply(int64((*ctx)["message_id"].(float64)))}, arrayMessage...)
	}

	return constructMessage(ctx, &arrayMessage)
}

func SendPoke(ctx *map[string]any, uid int64) *[]byte {
	return constructMessage(ctx, &[]cqcode.ArrayMessage{*cqcode.Poke(uid)})
}

func SendMusic(ctx *map[string]any, urlType string, id int64) *[]byte {
	return constructMessage(ctx, &[]cqcode.ArrayMessage{*cqcode.Music(urlType, id)})
}

func SendPrivateForward(ctx *map[string]any, data *[]structs.ForwardNode, echo string) *[]byte {
	params := structs.PrivateForward{
		UserId:   int64((*ctx)["sender"].(map[string]any)["user_id"].(float64)),
		Messages: *data,
	}

	return SendAction("send_private_forward_msg", params, echo)
}

func SendGroupForward(ctx *map[string]any, data *[]structs.ForwardNode, echo string) *[]byte {
	params := structs.GroupForward{
		GroupId:  int64((*ctx)["group_id"].(float64)),
		Messages: *data,
	}

	return SendAction("send_group_forward_msg", params, echo)
}

func ConstructForwardNode(data *[]cqcode.ArrayMessage) *structs.ForwardNode {
	node := structs.NewForwardNode()
	node.Data.Content = *data

	return node
}

func CheckArgument(ctx *map[string]any, arg string) bool {
	if split := SplitArgument(ctx); len(split) > 0 {
		return SplitArgument(ctx)[0] == arg
	}
	return false
}

func CheckArgumentArray(ctx *map[string]any, args *[]string) bool {
	if args == nil {
		return false
	}

	for _, arg := range *args {
		if split := SplitArgument(ctx); len(split) > 0 {
			if SplitArgument(ctx)[0] == arg {
				return true
			}
		}

	}
	return false
}

func SplitArgument(ctx *map[string]any) []string {
	message := DecodeArrayMessage(ctx)
	for _, msg := range *message {
		if msg.Type == "text" {
			return strings.Fields(msg.Data["text"].(string))
		}
	}
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

func HandleBannedHostsArray(str *string) {
	bannedHosts := []string{"danbooru.donmai.us", "konachan.com"}
	*str = strings.Replace(*str, "//", "//\u200B", -1)
	for _, host := range bannedHosts {
		*str = strings.Replace(*str, host, strings.Replace(host, ".", ".\u200B", -1), -1)
	}
	return
}

func GetOriginUrl(url string) *string {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Url parser request error: %v", err)
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Url parser response error: %v", err)
		return nil
	}

	originURL := resp.Request.URL.String()
	return &originURL
}

func DecodeArrayMessage(ctx *map[string]any) *[]cqcode.ArrayMessage {
	msg, err := json.Marshal((*ctx)["message"])
	if err != nil {
		log.Printf("Marshal message error: %v", err)
		return nil
	}
	return cqcode.Unmarshal(msg)
}

func Md5(origin *[]byte) string {
	return fmt.Sprintf("%x", md5.Sum(*origin))
}

func constructMessage(ctx *map[string]any, message *[]cqcode.ArrayMessage) *[]byte {
	messageType := (*ctx)["message_type"].(string)
	var act structs.Action
	if messageType == "private" {
		userId := int64((*ctx)["sender"].(map[string]any)["user_id"].(float64))
		msg := structs.PrivateMessage{UserId: userId, Message: *message}
		act = structs.Action{Action: "send_private_msg", Params: msg}
	} else {
		groupId := int64((*ctx)["group_id"].(float64))
		msg := structs.GroupMessage{GroupId: groupId, Message: *message}
		act = structs.Action{Action: "send_group_msg", Params: msg}
	}

	jsonMsg, _ := json.Marshal(act)
	return &jsonMsg
}
