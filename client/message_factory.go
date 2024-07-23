package client

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"context"
	"encoding/json"
	"log"
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

		messageStruct.CleanMessage, messageStruct.Command = essentials.CleanMessage(&messageStruct.Message)

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
			log.Printf("Unmarshal error: %v", err)
			return
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
