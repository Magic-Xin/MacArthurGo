package essentials

import (
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

// SendFile Deprecated
func SendFile(messageStruct *structs.MessageStruct, file string, name string) *[]byte {
	if file == "" || messageStruct == nil {
		return nil
	}

	var act structs.Action
	if messageStruct.MessageType == "group" {
		groupId := messageStruct.GroupId
		params := structs.GroupFile{GroupId: groupId, File: file, Name: name}
		act = structs.Action{Action: "upload_group_file", Params: params}
	} else {
		userId := messageStruct.UserId
		params := structs.PrivateFile{UserId: userId, File: file, Name: name}
		act = structs.Action{Action: "upload_private_file", Params: params}
	}

	jsonMsg, _ := json.Marshal(act)
	return &jsonMsg
}

func SendMsg(messageStruct *structs.MessageStruct, message string, messageArray *[]cqcode.ArrayMessage, at bool, reply bool, echo string) *[]byte {
	if (message == "" && messageArray == nil) || messageStruct == nil {
		return nil
	}

	arrayMessage := []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": message}}}
	if messageArray != nil {
		arrayMessage = append(arrayMessage, *messageArray...)
	}

	if at && messageStruct.MessageType == "group" {
		uid := strconv.FormatInt(messageStruct.UserId, 10)
		arrayMessage = append([]cqcode.ArrayMessage{*cqcode.At(uid)}, arrayMessage...)
	}
	if reply {
		msgId := strconv.FormatInt(messageStruct.MessageId, 10)
		arrayMessage = append([]cqcode.ArrayMessage{*cqcode.Reply(msgId)}, arrayMessage...)
	}

	return constructMessage(messageStruct, &arrayMessage, echo)
}

func SendPoke(messageStruct *structs.MessageStruct, uid int64) *[]byte {
	if messageStruct.MessageType == "group" {
		return SendAction("group_poke",
			struct {
				GroupId int64 `json:"group_id"`
				UserId  int64 `json:"user_id"`
			}{GroupId: messageStruct.GroupId, UserId: uid}, "")
	} else if messageStruct.MessageType == "private" {
		return SendAction("friend_poke",
			struct {
				UserId int64 `json:"user_id"`
			}{UserId: uid}, "")
	}
	return nil
}

func SendMusic(messageStruct *structs.MessageStruct, urlType string, id string) *[]byte {
	return constructMessage(messageStruct, &[]cqcode.ArrayMessage{*cqcode.Music(urlType, id)}, "")
}

func SendPrivateForward(messageStruct *structs.MessageStruct, data *[]structs.ForwardNode, echo string) *[]byte {
	params := structs.PrivateForward{
		UserId:   messageStruct.UserId,
		Messages: *data,
	}

	return SendAction("send_private_forward_msg", params, echo)
}

func SendGroupForward(messageStruct *structs.MessageStruct, data *[]structs.ForwardNode, echo string) *[]byte {
	params := structs.GroupForward{
		GroupId:  messageStruct.GroupId,
		Messages: *data,
	}

	return SendAction("send_group_forward_msg", params, echo)
}

func ConstructForwardNode(uin string, name string, data *[]cqcode.ArrayMessage) *structs.ForwardNode {
	node := structs.NewForwardNode()
	node.Data.Uin = uin
	node.Data.Name = name
	node.Data.Content = *data

	return node
}

func CheckArgumentArray(command string, args *[]string) bool {
	if args == nil {
		return false
	}

	for _, arg := range *args {
		if arg == command {
			return true
		}
	}
	return false
}

func CheckArgumentMap(command string, argsMap *map[string]string) (string, bool) {
	if argsMap == nil {
		return "", false
	}

	for key, value := range *argsMap {
		if value == command {
			return key, true
		}
	}
	return "", false
}

func SplitArgument(message *[]cqcode.ArrayMessage) (res []string) {
	for _, msg := range *message {
		if msg.Type == "text" {
			res = append(res, strings.Fields(msg.Data["text"].(string))...)
		}
	}
	return res
}

func GetImageKey(url string) string {
	const pattern = "rkey=(.*)&?"
	if match := regexp.MustCompile(pattern).FindAllStringSubmatch(url, -1); match != nil {
		return match[0][1]
	}
	return ""
}

func GetImageData(url string) *bytes.Buffer {
	tlsConfig := &tls.Config{
		ServerName: "multimedia.nt.qq.com.cn",
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		},
		InsecureSkipVerify: false,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Transport: transport,
	}

	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Image fetch close error: %v", err)
		}
	}(resp.Body)

	var imageData bytes.Buffer
	_, err = io.Copy(&imageData, resp.Body)
	if err != nil {
		panic(err)
	}

	return &imageData
}

func ImageToBase64(url string) *string {
	imageData := GetImageData(url)
	imageBase64 := "base64://" + base64.StdEncoding.EncodeToString(imageData.Bytes())

	return &imageBase64
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

func Md5(origin *[]byte) string {
	return fmt.Sprintf("%x", md5.Sum(*origin))
}

func constructMessage(messageStruct *structs.MessageStruct, message *[]cqcode.ArrayMessage, echo string) *[]byte {
	if messageStruct.MessageType == "" {
		return nil
	}

	var act structs.Action
	msg := structs.Message{
		MessageType: messageStruct.MessageType,
		UserId:      messageStruct.UserId,
		GroupId:     messageStruct.GroupId,
		Message:     *message,
	}
	act = structs.Action{Action: "send_msg", Params: msg, Echo: echo}

	jsonMsg, _ := json.Marshal(act)
	return &jsonMsg
}

func RemoveMarkdown(input string) string {
	replacements := map[string]string{
		`(?m)^#{1,6}\s*`:          "",   // Headers
		`\*\*([^*]+)\*\*`:         "$1", // Bold
		`\*([^*]+)\*`:             "$1", // Italic
		`\[([^\]]+)\]\([^)]+\)`:   "$1", // Links
		"`([^`]+)`":               "$1", // Inline code
		`~~([^~]+)~~`:             "$1", // Strikethrough
		`!\[([^\]]*)\]\([^)]+\)`:  "$1", // Images
		`(?m)^>\s*`:               "",   // Blockquotes
		`(?m)^(\s*[-*+]\s+)`:      "",   // Unordered lists
		`(?m)^\d+\.\s+`:           "",   // Ordered lists
		`(?m)^(\s*[-*_]{3,}\s*)$`: "",   // Horizontal rules
	}

	for pattern, replacement := range replacements {
		re := regexp.MustCompile(pattern)
		input = re.ReplaceAllString(input, replacement)
	}

	return input
}
