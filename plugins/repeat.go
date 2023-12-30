package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"sync"
)

type Repeat struct {
	essentials.Plugin
	Times             int64
	Probability       float64
	CommonProbability float64
	repeatMap         sync.Map
}

func init() {
	repeat := Repeat{
		Plugin: essentials.Plugin{
			Name:    "随机复读",
			Enabled: base.Config.Plugins.Repeat.Enable,
		},
		Times:             base.Config.Plugins.Repeat.Times,
		Probability:       base.Config.Plugins.Repeat.Probability,
		CommonProbability: base.Config.Plugins.Repeat.CommonProbability,
	}

	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &repeat})
}

func (r *Repeat) ReceiveAll(_ *map[string]any, _ *chan []byte) {}

func (r *Repeat) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !r.Enabled {
		return
	}

	if (*ctx)["message_type"].(string) != "group" || (*ctx)["message"] == nil || (*ctx)["group_id"] == nil {
		return
	}

	msg, err := json.Marshal((*ctx)["message"])
	if err != nil {
		log.Printf("Repeat json marshal error: %v", err)
		return
	}
	message := essentials.DecodeArrayMessage(ctx)
	if message == nil {
		return
	}

	groupId := strconv.FormatInt(int64((*ctx)["group_id"].(float64)), 10)
	md5 := essentials.Md5(&msg)
	cache, ok := r.repeatMap.Load(groupId)
	if !ok {
		r.repeatMap.Store(groupId, []any{md5, 1})
		return
	}

	if cache.([]any)[0].(string) == md5 {
		if cache.([]any)[1].(int) >= int(r.Times) && r.getRand(false) {
			r.repeatMap.Store(groupId, []any{md5, 1})
			*send <- *essentials.SendMsg(ctx, "", message, false, false)
			return
		} else {
			r.repeatMap.Store(groupId, []any{md5, cache.([]any)[1].(int) + 1})
		}
	} else {
		r.repeatMap.Store(groupId, []any{md5, 1})
	}

	if r.getRand(true) {
		*send <- *essentials.SendMsg(ctx, "", message, false, false)
	}
}

func (r *Repeat) ReceiveEcho(_ *map[string]any, _ *chan []byte) {}

func (r *Repeat) getRand(common bool) bool {
	if common {
		return r.CommonProbability > rand.Float64()
	}
	return r.Probability > rand.Float64()
}
