package essentials

import (
	"MacArthurGo/structs"
	"context"
)

type IPlugin interface {
	ReceiveAll() *[]byte
	ReceiveMessage(*structs.MessageStruct) *[]byte
	ReceiveEcho(*structs.EchoMessageStruct) *[]byte
}

var PluginArray []*Plugin

type Plugin struct {
	Name      string
	Enabled   bool
	Args      []string
	Interface IPlugin
}

func (p *Plugin) GoroutineAll(ctx context.Context) *[]byte {
	if !p.Enabled {
		return nil
	}
	select {
	case <-ctx.Done():
		return nil
	default:
		return p.Interface.ReceiveAll()
	}
}

func (p *Plugin) GoroutineMessage(ctx context.Context, messageStruct *structs.MessageStruct) *[]byte {
	if !p.Enabled {
		return nil
	}
	select {
	case <-ctx.Done():
		return nil
	default:
		return p.Interface.ReceiveMessage(messageStruct)
	}
}

func (p *Plugin) GoroutineEcho(ctx context.Context, echoMessageStruct *structs.EchoMessageStruct) *[]byte {
	if !p.Enabled {
		return nil
	}
	select {
	case <-ctx.Done():
		return nil
	default:
		return p.Interface.ReceiveEcho(echoMessageStruct)
	}
}
