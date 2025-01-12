package essentials

import (
	"MacArthurGo/structs"
	"context"
)

type IPlugin interface {
	ReceiveAll(chan<- *[]byte)
	ReceiveMessage(*structs.MessageStruct, chan<- *[]byte)
	ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte)
}

var PluginArray []*Plugin

type Plugin struct {
	Name      string
	Enabled   bool
	Args      []string
	Interface IPlugin
}

func (p *Plugin) GoroutineAll(ctx context.Context, send chan<- *[]byte) {
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

func (p *Plugin) GoroutineMessage(ctx context.Context, messageStruct *structs.MessageStruct, send chan<- *[]byte) {
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

func (p *Plugin) GoroutineEcho(ctx context.Context, echoMessageStruct *structs.EchoMessageStruct, send chan<- *[]byte) {
	if !p.Enabled {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		p.Interface.ReceiveEcho(echoMessageStruct, send)
	}
}
