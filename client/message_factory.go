package client

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"
)

func MessageFactory(msg *[]byte, send essentials.SendFunc, event bool) {
	if msg == nil || send == nil {
		return
	}

	var message structs.IncomingMessageStruct
	var feedback structs.FeedbackStruct

	if event {
		var eventStruct structs.EventStruct
		err := json.Unmarshal(*msg, &eventStruct)
		if err != nil {
			log.Printf("Unmarshal error: %v", err)
			return
		}
		if eventStruct.EventType == "message_receive" {
			bytesData, err := json.Marshal(eventStruct.Data)
			if err != nil {
				log.Printf("Marshal error: %v", err)
				return
			}
			err = json.Unmarshal(bytesData, &message)
			if err != nil {
				log.Printf("Unmarshal error: %v", err)
				return
			}
		}
	} else {
		err := json.Unmarshal(*msg, &feedback)
		if err != nil {
			log.Printf("Unmarshal error: %v", err)
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, p := range essentials.PluginArray {
		go func(plugin *essentials.Plugin) {
			plugin.GoroutineAll(ctx, send)
		}(p)
	}

	if event {
		if essentials.BanList.IsBanned(message.SenderID) {
			return
		}

		message.CleanMessage, message.Command = CleanMessage(&message.Segments)

		for _, p := range essentials.PluginArray {
			go func(plugin *essentials.Plugin) {
				plugin.GoroutineMessage(ctx, &message, send)
			}(p)
		}
	} else {
		for _, p := range essentials.PluginArray {
			go func(plugin *essentials.Plugin) {
				plugin.GoroutineEcho(ctx, &feedback, send)
			}(p)
		}
	}

	<-ctx.Done()
}

func CleanMessage(message *[]structs.MessageSegment) (*[]structs.MessageSegment, string) {
	var (
		res     []structs.MessageSegment
		command string
	)
	for _, m := range *message {
		if m.Type == "text" && command == "" {
			words := strings.Fields(m.Data["text"].(string))
			if len(words) == 0 {
				continue
			}
			if strings.HasPrefix(words[0], "/") {
				command = words[0]
				res = append(res, []structs.MessageSegment{{Type: "text", Data: map[string]any{
					"text": strings.Join(words[1:], " "),
				}}}...)
			}
		} else {
			res = append(res, m)
		}
	}
	return &res, command
}
