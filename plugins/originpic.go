package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"fmt"
	"log"
	"strconv"
	"strings"
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

func (*OriginPic) ReceiveAll() *[]byte {
	return nil
}

func (*OriginPic) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	if !essentials.CheckArgumentArray(messageStruct.Command, &base.Config.Plugins.OriginPic.Args) {
		return nil
	}

	message := messageStruct.Message
	if message == nil {
		return nil
	}

	for _, m := range message {
		if m.Type == "reply" {
			echo := fmt.Sprintf("originPic|%d", messageStruct.MessageId)
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageStruct.MessageId, 10), value)
			return essentials.SendAction("get_msg", structs.GetMsg{Id: m.Data["id"].(string)}, echo)
		}
	}

	return nil
}

func (o *OriginPic) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct) *[]byte {
	if echoMessageStruct.Status != "ok" {
		return nil
	}

	echo := echoMessageStruct.Echo
	split := strings.Split(echo, "|")

	if split[0] == "originPic" {
		contexts := echoMessageStruct.Data
		message := contexts.Message
		if message == nil {
			return nil
		}

		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Origin picture cache not found")
			return nil
		}
		messageStruct := value.(essentials.EchoCache).Value

		for _, m := range message {
			if m.Type == "image" {
				msg := fmt.Sprintf("已获取原图链接，请尽快保存:\n %s", m.Data["url"])
				return essentials.SendMsg(&messageStruct, msg, nil, false, true, "")
			}
		}
	}
	return nil
}
