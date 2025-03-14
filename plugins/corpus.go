package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"regexp"
	"strings"
)

type Corpus struct {
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
		rules: &rules,
	}
	plugin := &essentials.Plugin{
		Name:      "语料库回复",
		Enabled:   base.Config.Plugins.Corpus.Enable,
		Interface: &corpus,
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (*Corpus) ReceiveAll(chan<- *[]byte) {}

func (c *Corpus) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	message := messageStruct.Message
	if message == nil || messageStruct.MessageType == "" {
		return
	}
	var text string
	for _, msg := range message {
		if msg.Type == "text" {
			text += msg.Data["text"].(string)
		}
	}

	for _, v := range *c.rules {
		if match := regexp.MustCompile(v.Regexp).MatchString(text); match {
			if v.Scene != "a" && v.Scene != "all" {
				if !strings.HasPrefix(messageStruct.MessageType, v.Scene) {
					continue
				}
			}
			if v.Users != nil {
				userId := messageStruct.UserId
				if !c.Contain(v.Users, userId) {
					continue
				}
			}
			if v.Groups != nil && messageStruct.MessageType == "group" {
				groupId := messageStruct.GroupId
				if !c.Contain(v.Groups, groupId) {
					continue
				}
			}

			if v.Message != nil {
				send <- essentials.SendMsg(messageStruct, "", v.Message, v.IsAt, v.IsReply, "")
			}
			break
		}
	}
	return
}

func (*Corpus) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}

func (*Corpus) Contain(arr []int64, item int64) bool {
	for _, v := range arr {
		if v == item {
			return true
		}
	}
	return false
}
