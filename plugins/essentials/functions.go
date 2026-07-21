package essentials

import (
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	urlpkg "net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HTTPClient is shared across plugins so connections are pooled and every
// request has a finite upper bound.
var HTTPClient = &http.Client{
	Transport: http.DefaultTransport.(*http.Transport).Clone(),
	Timeout:   60 * time.Second,
}

func SendAction(action string, params any, echo string) []byte {
	if action == "" {
		return nil
	}

	act := structs.Action{Action: action, Params: params, Echo: echo}
	jsonMsg, _ := json.Marshal(act)

	return jsonMsg
}

// SendFile Deprecated
func SendFile(messageStruct *structs.MessageStruct, file string, name string) []byte {
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
	return jsonMsg
}

func SendMsg(messageStruct *structs.MessageStruct, message string, messageArray []cqcode.ArrayMessage, at bool, reply bool, echo string) []byte {
	if (message == "" && messageArray == nil) || messageStruct == nil {
		return nil
	}

	arrayMessage := []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{"text": message}}}
	if messageArray != nil {
		arrayMessage = append(arrayMessage, messageArray...)
	}

	if at && messageStruct.MessageType == "group" {
		uid := strconv.FormatInt(messageStruct.UserId, 10)
		arrayMessage = append([]cqcode.ArrayMessage{*cqcode.At(uid)}, arrayMessage...)
	}
	if reply {
		msgId := strconv.FormatInt(messageStruct.MessageId, 10)
		arrayMessage = append([]cqcode.ArrayMessage{*cqcode.Reply(msgId)}, arrayMessage...)
	}

	return constructMessage(messageStruct, arrayMessage, echo)
}

func SendPoke(messageStruct *structs.MessageStruct, uid int64) []byte {
	switch messageStruct.MessageType {
	case "group":
		return SendAction("group_poke",
			struct {
				GroupId int64 `json:"group_id"`
				UserId  int64 `json:"user_id"`
			}{GroupId: messageStruct.GroupId, UserId: uid}, "")
	case "private":
		return SendAction("friend_poke",
			struct {
				UserId int64 `json:"user_id"`
			}{UserId: uid}, "")
	}
	return nil
}

func SendMusic(messageStruct *structs.MessageStruct, urlType string, id string) []byte {
	return constructMessage(messageStruct, []cqcode.ArrayMessage{*cqcode.Music(urlType, id)}, "")
}

func SendPrivateForward(messageStruct *structs.MessageStruct, data []structs.ForwardNode, echo string) []byte {
	params := structs.PrivateForward{
		UserId:   messageStruct.UserId,
		Messages: data,
	}

	return SendAction("send_private_forward_msg", params, echo)
}

func SendGroupForward(messageStruct *structs.MessageStruct, data []structs.ForwardNode, echo string) []byte {
	params := structs.GroupForward{
		GroupId:  messageStruct.GroupId,
		Messages: data,
	}

	return SendAction("send_group_forward_msg", params, echo)
}

func ConstructForwardNode(uin string, name string, data []cqcode.ArrayMessage) *structs.ForwardNode {
	node := structs.NewForwardNode()
	node.Data.Uin = uin
	node.Data.Name = name
	node.Data.Content = data

	return node
}

func CheckArgumentArray(command string, args []string) bool {
	if args == nil {
		return false
	}

	for _, arg := range args {
		if arg == command {
			return true
		}
	}
	return false
}

func CheckArgumentMap(command string, argsMap map[string]string) (string, bool) {
	if argsMap == nil {
		return "", false
	}

	for key, value := range argsMap {
		if value == command {
			return key, true
		}
	}
	return "", false
}

func SplitArgument(message []cqcode.ArrayMessage) (res []string) {
	for _, msg := range message {
		if msg.Type == "text" {
			if text, ok := msg.Data["text"].(string); ok {
				res = append(res, strings.Fields(text)...)
			}
		}
	}
	return res
}

func GetImageKey(imageURL string) string {
	canonicalURL := imageURL
	if parsed, err := urlpkg.Parse(imageURL); err == nil {
		if query, queryErr := urlpkg.ParseQuery(parsed.RawQuery); queryErr == nil {
			for key := range query {
				// rkey authorizes media downloads and may be shared by different images.
				if strings.EqualFold(key, "rkey") {
					query.Del(key)
				}
			}
			parsed.RawQuery = query.Encode()
			parsed.ForceQuery = false
			parsed.Fragment = ""
			canonicalURL = parsed.String()
		}
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalURL)))
}

func GetImageData(url string) *bytes.Buffer {
	imageData, err := FetchImageData(context.Background(), url)
	if err != nil {
		log.Printf("Image fetch error: %v", err)
		return &bytes.Buffer{}
	}
	return imageData
}

func FetchImageData(ctx context.Context, imageURL string) (*bytes.Buffer, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Image fetch close error: %v", err)
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image request returned %s", resp.Status)
	}

	var imageData bytes.Buffer
	const maxImageSize = 20 << 20
	_, err = io.Copy(&imageData, io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return nil, err
	}
	if imageData.Len() > maxImageSize {
		return nil, fmt.Errorf("image exceeds %d MiB", maxImageSize>>20)
	}

	return &imageData, nil
}

func ImageToBase64(url string) *string {
	imageData, err := FetchImageData(context.Background(), url)
	if err != nil {
		log.Printf("Image base64 fetch error: %v", err)
		return nil
	}
	imageBase64 := "base64://" + base64.StdEncoding.EncodeToString(imageData.Bytes())

	return &imageBase64
}

func GetOriginUrl(url string) *string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Url parser request error: %v", err)
		return nil
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		log.Printf("Url parser response error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	originURL := resp.Request.URL.String()
	return &originURL
}

func Md5(origin []byte) string {
	return fmt.Sprintf("%x", md5.Sum(origin))
}

func constructMessage(messageStruct *structs.MessageStruct, message []cqcode.ArrayMessage, echo string) []byte {
	if messageStruct.MessageType == "" {
		return nil
	}

	var act structs.Action
	msg := structs.Message{
		MessageType: messageStruct.MessageType,
		UserId:      messageStruct.UserId,
		GroupId:     messageStruct.GroupId,
		Message:     message,
	}
	act = structs.Action{Action: "send_msg", Params: msg, Echo: echo}

	jsonMsg, _ := json.Marshal(act)
	return jsonMsg
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
