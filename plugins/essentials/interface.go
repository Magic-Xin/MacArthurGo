package essentials

import "MacArthurGo/structs/cqcode"

type MessageStruct struct {
	Time         int64  `json:"time"`
	MessageType  string `json:"message_type"`
	MessageId    int64  `json:"message_id"`
	GroupId      int64  `json:"group_id"`
	UserId       int64  `json:"user_id"`
	RawMessage   string `json:"raw_message"`
	Echo         string `json:"echo"`
	MessageArray *[]cqcode.ArrayMessage
}

type Plugin struct {
	Name    string
	Enabled bool
	Args    []string
}

type PluginInterface struct {
	Interface IPlugin
}

type IPlugin interface {
	ReceiveAll(*map[string]any, *chan []byte)
	ReceiveMessage(*MessageStruct, *chan []byte)
	ReceiveEcho(*map[string]any, *chan []byte)
}

var PluginArray []*PluginInterface
