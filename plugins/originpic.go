package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"fmt"
	"time"
)

type OriginPic struct{}

func init() {
	originPic := OriginPic{}
	plugin := &essentials.Plugin{
		Name:      "原图",
		Enabled:   base.Config.Plugins.OriginPic.Enable,
		Args:      base.Config.Plugins.OriginPic.Args,
		Interface: &originPic,
	}

	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (*OriginPic) ReceiveAll(essentials.SendFunc) {}

func (*OriginPic) ReceiveMessage(incomingMessageStruct *structs.IncomingMessageStruct, send essentials.SendFunc) {
	if !essentials.CheckArgumentArray(incomingMessageStruct.Command, &base.Config.Plugins.OriginPic.Args) {
		return
	}

	message := incomingMessageStruct.Segments
	if message == nil {
		return
	}

	for _, m := range message {
		if m.Type == "reply" {
			id := int64(m.Data["message_seq"].(float64))
			value := essentials.EchoCache{Value: *incomingMessageStruct, Time: time.Now().Unix()}
			essentials.SetCache(fmt.Sprintf("%d|%s", id, "originPic"), value)
			essentials.GetMessage(incomingMessageStruct, id, send)
		}
	}
}

func (o *OriginPic) ReceiveEcho(feedbackStruct *structs.FeedbackStruct, send essentials.SendFunc) {
	if feedbackStruct.Status != "ok" {
		return
	}

	message := feedbackStruct.Data.Message
	messageSeq := message.MessageSeq

	if messageSeq == 0 {
		return
	}

	value, ok := essentials.GetCache(fmt.Sprintf("%d|%s", messageSeq, "originPic"))
	if !ok {
		return
	}

	messageStruct := value.(essentials.EchoCache).Value

	for _, m := range message.Segments {
		if m.Type == "image" {
			msg := fmt.Sprintf("已获取原图链接，请尽快保存:\n %s", m.Data["temp_url"])
			essentials.SendMsg(&messageStruct, msg, nil, false, true, send)
		}
	}
}
