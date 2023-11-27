package essentials

type Plugin struct {
	Name            string
	Enabled         bool
	Arg             string
	PluginInterface IPlugin
}

type IPlugin interface {
	ReceiveAll(ctx *map[string]any, send *chan []byte)
	ReceiveMessage(ctx *map[string]any, send *chan []byte)
	ReceiveEcho(ctx *map[string]any, send *chan []byte)
}

// PluginArray for traversal
var PluginArray []*Plugin

var AllArray []*Plugin
var MessageArray []*Plugin
var EchoArray []*Plugin
