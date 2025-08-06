package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"encoding/json"
	"log"
	"math/rand"
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

func (*Repeat) ReceiveAll(essentials.SendFunc) {}

func (r *Repeat) ReceiveMessage(incomingMessageStruct *structs.IncomingMessageStruct, send essentials.SendFunc) {
	if incomingMessageStruct.MessageScene != "group" || incomingMessageStruct.Segments == nil || incomingMessageStruct.Command != "" {
		return
	}

	message := incomingMessageStruct.Segments

	if message[0].Type == "text" && message[0].Data["text"].(string) == "[该接龙表情不支持查看，请使用QQ最新版本]" {
		return
	}

	msg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Repeat json marshal error: %v", err)
		return
	}

	groupId := incomingMessageStruct.Group.GroupID
	md5 := essentials.Md5(&msg)
	cache, ok := r.repeatMap.Load(groupId)
	if !ok {
		r.repeatMap.Store(groupId, []any{md5, 1})
		return
	}

	if cache.([]any)[0].(string) == md5 {
		if cache.([]any)[1].(int) >= int(r.Times) && r.getRand(false) {
			r.repeatMap.Store(groupId, []any{md5, 1})
			essentials.SendMsg(incomingMessageStruct, "", &message, false, false, send)
			return
		} else {
			r.repeatMap.Store(groupId, []any{md5, cache.([]any)[1].(int) + 1})
		}
	} else {
		r.repeatMap.Store(groupId, []any{md5, 1})
	}

	if r.getRand(true) {
		essentials.SendMsg(incomingMessageStruct, "", &message, false, false, send)
	}
}

func (*Repeat) ReceiveEcho(*structs.FeedbackStruct, essentials.SendFunc) {}

func (r *Repeat) getRand(common bool) bool {
	if common {
		return r.CommonProbability > rand.Float64()
	}
	return r.Probability > rand.Float64()
}
