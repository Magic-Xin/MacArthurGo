package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs/cqcode"
	"regexp"
	"strings"
)

type Corpus struct {
	essentials.Plugin
	rules *[]Rules
}

type Rules struct {
	Regexp  string
	Reply   string
	IsReply bool
	IsAt    bool
	Scene   string
	Users   []int64
	Groups  []int64
	Message *[]cqcode.ArrayMessage
}

func init() {
	var rules []Rules
	for _, v := range base.Config.Plugins.Corpus.Rules {
		rule := Rules{
			Regexp:  v.Regexp,
			Reply:   v.Reply,
			IsReply: v.IsReply,
			IsAt:    v.IsAt,
			Scene:   v.Scene,
			Users:   v.Users,
			Groups:  v.Groups,
		}

		cq := cqcode.FromStr(v.Reply)
		if cq != nil {
			rule.Message = cq
		}

		rules = append(rules, rule)
	}

	corpus := Corpus{
		Plugin: essentials.Plugin{
			Name:    "语料库回复",
			Enabled: base.Config.Plugins.Corpus.Enable,
		},
		rules: &rules,
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

	for _, v := range *c.rules {
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
