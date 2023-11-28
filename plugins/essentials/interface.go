package essentials

type Plugin struct {
	Name    string
	Enabled bool
	Arg     []string
}

type PluginInterface struct {
	Interface IPlugin
}

type IPlugin interface {
	ReceiveAll(*map[string]any, *chan []byte)
	ReceiveMessage(*map[string]any, *chan []byte)
	ReceiveEcho(*map[string]any, *chan []byte)
}

var PluginArray []*PluginInterface
