package websocket

import (
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"context"
	"encoding/json"
	"log"
)

func MessageFactory(msg *[]byte) *[]byte {
	var messageStruct structs.MessageStruct
	err := json.Unmarshal(*msg, &messageStruct)
	if err != nil {
		log.Printf("Unmarshal error: %v", err)
		return nil
	}

	ch := make(chan *[]byte)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, p := range essentials.PluginArray {
		go func() {
			r := p.GoroutineAll(ctx)
			if r != nil {
				ch <- r
			}
		}()
	}

	if messageStruct.Message != nil {
		if essentials.BanList.IsBanned(messageStruct.UserId) {
			return nil
		}

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
			return nil
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
		case r := <-ch:
			if r != nil {
				cancel()
				close(ch)
				return r
			}
		case <-ctx.Done():
			return nil
		}
	}
}
