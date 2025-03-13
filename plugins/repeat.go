package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"sync"
)

type Repeat struct {
	Times             int64
	Probability       float64
	CommonProbability float64
	repeatMap         sync.Map
}

func init() {
	repeat := Repeat{
		Times:             base.Config.Plugins.Repeat.Times,
		Probability:       base.Config.Plugins.Repeat.Probability,
		CommonProbability: base.Config.Plugins.Repeat.CommonProbability,
	}
	plugin := &essentials.Plugin{
		Name:      "随机复读",
		Enabled:   base.Config.Plugins.Repeat.Enable,
		Interface: &repeat,
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (*Repeat) ReceiveAll(chan<- *[]byte) {}

func (r *Repeat) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	if messageStruct.MessageType != "group" || messageStruct.Message == nil ||
		messageStruct.GroupId == 0 || len(messageStruct.Message) == 0 ||
		messageStruct.Command != "" {
		return
	}

	message := messageStruct.Message

	if message[0].Type == "text" && message[0].Data["text"].(string) == "[该接龙表情不支持查看，请使用QQ最新版本]" {
		return
	}

	msg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Repeat json marshal error: %v", err)
		return
	}

	groupId := strconv.FormatInt(messageStruct.GroupId, 10)
	md5 := essentials.Md5(&msg)
	cache, ok := r.repeatMap.Load(groupId)
	if !ok {
		r.repeatMap.Store(groupId, []any{md5, 1})
		return
	}

	if cache.([]any)[0].(string) == md5 {
		if cache.([]any)[1].(int) >= int(r.Times) && r.getRand(false) {
			r.repeatMap.Store(groupId, []any{md5, 1})
			send <- essentials.SendMsg(messageStruct, "", &message, false, false, "")
			return
		} else {
			r.repeatMap.Store(groupId, []any{md5, cache.([]any)[1].(int) + 1})
		}
	} else {
		r.repeatMap.Store(groupId, []any{md5, 1})
	}

	if r.getRand(true) {
		send <- essentials.SendMsg(messageStruct, "", &message, false, false, "")
	}
}

func (*Repeat) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}

func (r *Repeat) getRand(common bool) bool {
	if common {
		return r.CommonProbability > rand.Float64()
	}
	return r.Probability > rand.Float64()
}
