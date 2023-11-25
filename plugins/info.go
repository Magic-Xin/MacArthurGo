package plugins

import (
	_struct "MacArthurGo/structs"
	"encoding/json"
	"github.com/gookit/config/v2"
	"log"
)

type LoginInfo struct {
	NickName string
	UserId   int64
}

var info LoginInfo

func Info(data map[string]any) {
	info.NickName, info.UserId = data["nickname"].(string), int64(data["user_id"].(float64))
	log.Printf("Get account nickname: %s, id: %d", info.NickName, info.UserId)
}

func GetInfo(send *chan []byte) {
	getInfo, _ := json.Marshal(_struct.EchoAction{Action: _struct.Action{Action: "get_login_info"}, Echo: "info"})
	*send <- getInfo
	login, _ := json.Marshal(_struct.Action{Action: "send_msg", Params: _struct.Message{
		MessageType: "private",
		UserId:      config.Int64("admin"),
		Message:     "MacArthurGo 已上线"}})
	*send <- login
}
