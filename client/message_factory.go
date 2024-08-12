package client

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"
)

func MessageFactory(msg *[]byte, sendPump chan *[]byte) {
	var messageStruct structs.MessageStruct
	err := json.Unmarshal(*msg, &messageStruct)
	if err != nil {
		log.Printf("Unmarshal error: %v", err)
		return
	}

	ch := make(chan *[]byte)
	allCh := make(chan *[]byte)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, p := range essentials.PluginArray {
		go func() {
			r := p.GoroutineAll(ctx)
			if r != nil {
				allCh <- r
			}
		}()
	}

	if messageStruct.Message != nil {
		if essentials.BanList.IsBanned(messageStruct.UserId) {
			return
		}

		messageStruct.CleanMessage, messageStruct.Command = CleanMessage(&messageStruct.Message)

		for _, p := range essentials.PluginArray {
			go func() {
				r := p.GoroutineMessage(ctx, &messageStruct)
				if r != nil {
					ch <- r
				}
			}()
		}
	}

	if messageStruct.Echo != "" {
		var echoMessageStruct structs.EchoMessageStruct
		err := json.Unmarshal(*msg, &echoMessageStruct)
		if err != nil {
			var echoMessageArrayStruct structs.EchoMessageArrayStruct
			err := json.Unmarshal(*msg, &echoMessageArrayStruct)
			if err != nil {
				log.Printf("Unmarshal error: %v", err)
				return
			}
			echoMessageStruct.DataArray = echoMessageArrayStruct.Data
			echoMessageStruct.Echo = echoMessageArrayStruct.Echo
			echoMessageStruct.Status = echoMessageArrayStruct.Status
		}

		for _, p := range essentials.PluginArray {
			go func() {
				r := p.GoroutineEcho(ctx, &echoMessageStruct)
				if r != nil {
					ch <- r
				}
			}()
		}
	}

	for {
		select {
		case r := <-allCh:
			if r != nil {
				sendPump <- r
			}
		case r := <-ch:
			if r != nil {
				cancel()
				sendPump <- r
			}
		case <-ctx.Done():
			return
		}
	}
}

func CleanMessage(message *[]cqcode.ArrayMessage) (*[]cqcode.ArrayMessage, string) {
	var (
		res     []cqcode.ArrayMessage
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
				res = append(res, []cqcode.ArrayMessage{{Type: "text", Data: map[string]any{
					"text": strings.Join(words[1:], " "),
				}}}...)
			}
		} else {
			res = append(res, m)
		}
	}
	return &res, command
}
