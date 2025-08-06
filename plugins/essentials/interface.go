package essentials

import (
	"MacArthurGo/structs"
	"context"
)

type SendFunc func(api string, body interface{})

type IPlugin interface {
	ReceiveAll(SendFunc)
	ReceiveMessage(*structs.IncomingMessageStruct, SendFunc)
	ReceiveEcho(*structs.FeedbackStruct, SendFunc)
}

var PluginArray []*Plugin

type Plugin struct {
	Name      string
	Enabled   bool
	Args      []string
	Interface IPlugin
}

func (p *Plugin) GoroutineAll(ctx context.Context, send SendFunc) {
	if !p.Enabled {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		p.Interface.ReceiveAll(send)
	}
}

func (p *Plugin) GoroutineMessage(ctx context.Context, messageStruct *structs.IncomingMessageStruct, send SendFunc) {
	if !p.Enabled {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		p.Interface.ReceiveMessage(messageStruct, send)
	}
}

func (p *Plugin) GoroutineEcho(ctx context.Context, feedbackStruct *structs.FeedbackStruct, send SendFunc) {
	if !p.Enabled {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		p.Interface.ReceiveEcho(feedbackStruct, send)
	}
}
