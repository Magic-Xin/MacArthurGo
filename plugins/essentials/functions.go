package essentials

import (
	"MacArthurGo/structs"
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

func GetMessage(incomingMessage *structs.IncomingMessageStruct, messageID int64, send SendFunc) {
	messageScene := incomingMessage.MessageScene
	peerId := incomingMessage.PeerID
	messageSeq := messageID

	SendAction("get_message", map[string]any{"message_scene": messageScene, "peer_id": peerId, "message_seq": messageSeq}, send)
}

func SendAction(api string, params map[string]any, send SendFunc) {
	if api == "" {
		return
	}

	send(api, params)
}

func SendMsg(incomingMessage *structs.IncomingMessageStruct, message string, outgoingMessage *[]structs.MessageSegment, at bool, reply bool, send SendFunc) {
	if (message == "" && outgoingMessage == nil) || incomingMessage == nil {
		return
	}

	outMessage := []structs.MessageSegment{{Type: "text", Data: map[string]any{"text": message}}}
	if outgoingMessage != nil {
		outMessage = append(outMessage, *outgoingMessage...)
	}

	if at && incomingMessage.MessageScene == "group" {
		outMessage = append([]structs.MessageSegment{*structs.Mention(incomingMessage.GroupMember.UserID)}, outMessage...)
	}

	if reply {
		outMessage = append([]structs.MessageSegment{*structs.Reply(incomingMessage.MessageSeq)}, outMessage...)
	}

	constructMessage(incomingMessage, &outMessage, send)
}

func SendGroupNudge(incomingMessage *structs.IncomingMessageStruct, uid int64, send SendFunc) {
	if incomingMessage.MessageScene == "group" {
		params := structs.GroupNudge{
			GroupId: incomingMessage.Group.GroupID,
			UserId:  uid,
		}
		send("send_group_nudge", params)
	} else if incomingMessage.MessageScene == "private" {
		outgoingMessage := []structs.MessageSegment{*structs.Text("戳一戳只支持在群聊中使用")}
		constructMessage(incomingMessage, &outgoingMessage, send)
	}
}

func ConstructForwardedMessage(userID int64, name string, outgoingMessage *[]structs.MessageSegment) *structs.OutgoingForwardedMessage {
	node := structs.OutgoingForwardedMessage{
		UserID:   userID,
		Name:     name,
		Segments: *outgoingMessage,
	}

	return &node
}

//func SendPrivateForward(messageStruct *structs.MessageStruct, data *[]structs.ForwardNode, echo string) *[]byte {
//	params := structs.PrivateForward{
//		UserId:   messageStruct.UserId,
//		Messages: *data,
//	}
//
//	return SendAction("send_private_forward_msg", params, echo)
//}
//
//func SendGroupForward(messageStruct *structs.MessageStruct, data *[]structs.ForwardNode, echo string) *[]byte {
//	params := structs.GroupForward{
//		GroupId:  messageStruct.GroupId,
//		Messages: *data,
//	}
//
//	return SendAction("send_group_forward_msg", params, echo)
//}
//
//func ConstructForwardNode(uin string, name string, data *[]structs.OutgoingForwardedMessage) *structs.ForwardNode {
//	node := structs.NewForwardNode()
//	node.Data.Uin = uin
//	node.Data.Name = name
//	node.Data.Content = *data
//
//	return node
//}

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

func SplitArgument(message *[]structs.MessageSegment) (res []string) {
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

func constructMessage(incomingMessage *structs.IncomingMessageStruct, outgoingMessage *[]structs.MessageSegment, send SendFunc) {
	msg := structs.OutgoingMessage{
		UserId:  incomingMessage.SenderID,
		GroupId: incomingMessage.Group.GroupID,
		Message: *outgoingMessage,
	}

	if incomingMessage.MessageScene == "group" {
		send("send_group_message", msg)
	} else {
		send("send_private_message", msg)
	}
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
