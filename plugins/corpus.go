package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"log"
	"os"
	"regexp"
	"strings"
)

type Corpus struct {
	essentials.Plugin
	Data *[]CorpusData
}

type CorpusData struct {
	Regexp  string  `json:"regexp"`
	Reply   string  `json:"reply"`
	IsReply bool    `json:"is_reply"`
	IsAt    bool    `json:"is_at"`
	Scene   string  `json:"scene"`
	Users   []int64 `json:"users"`
	Groups  []int64 `json:"groups"`
	Message *[]cqcode.ArrayMessage
}

func init() {
	f, err := os.Open("corpus.json")
	if err != nil {
		log.Printf("Open corpus.json failed: %v", err)
		return
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Printf("Close corpus.json failed: %v", err)
		}
	}(f)

	var data []CorpusData
	err = json.NewDecoder(f).Decode(&data)
	if err != nil {
		log.Printf("Decode corpus.json failed: %v", err)
		return
	}

	for i, v := range data {
		cq := cqcode.FromStr(v.Reply)
		if cq != nil {
			data[i].Message = cq
		}
	}

	corpus := Corpus{
		Plugin: essentials.Plugin{
			Name:    "语料库回复",
			Enabled: base.Config.Plugins.Corpus.Enable,
		},
		Data: &data,
	}
	essentials.PluginArray = append(essentials.PluginArray, &essentials.PluginInterface{Interface: &corpus})
}

func (c *Corpus) ReceiveAll(*map[string]any, *chan []byte) {}

func (c *Corpus) ReceiveMessage(ctx *map[string]any, send *chan []byte) {
	if !c.Enabled {
		return
	}

	message := essentials.DecodeArrayMessage(ctx)
	if message == nil || (*ctx)["message_type"] == nil {
		return
	}
	var text string
	for _, msg := range *message {
		if msg.Type == "text" {
			text += msg.Data["text"].(string)
		}
	}

	for _, v := range *c.Data {
		if match := regexp.MustCompile(v.Regexp).MatchString(text); match {
			if v.Scene != "a" && v.Scene != "all" {
				if !strings.HasPrefix((*ctx)["message_type"].(string), v.Scene) {
					continue
				}
			}
			if v.Users != nil {
				userId := int64((*ctx)["sender"].(map[string]any)["user_id"].(float64))
				if !c.Contain(v.Users, userId) {
					continue
				}
			}
			if v.Groups != nil && (*ctx)["message_type"].(string) == "group" {
				groupId := int64((*ctx)["group_id"].(float64))
				if !c.Contain(v.Groups, groupId) {
					continue
				}
			}

			if v.Message != nil {
				*send <- *essentials.SendMsg(ctx, "", v.Message, v.IsAt, v.IsReply)
			}
			break
		}
	}
}

func (c *Corpus) ReceiveEcho(*map[string]any, *chan []byte) {}

func (c *Corpus) Contain(arr []int64, item int64) bool {
	for _, v := range arr {
		if v == item {
			return true
		}
	}
	return false
}
