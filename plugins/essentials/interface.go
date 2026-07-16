package essentials

import (
	"MacArthurGo/structs"
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

// Handler is the event-facing portion of a plugin. Optional background work
// belongs in Starter so it can share the application's cancellation context.
type Handler interface {
	ReceiveMessage(*structs.MessageStruct, chan<- []byte)
	ReceiveEcho(*structs.EchoMessageStruct, chan<- []byte)
}

type Starter interface {
	Start(context.Context, chan<- []byte)
}

type Stopper interface {
	Stop() error
}

type Plugin struct {
	Name    string
	Enabled bool
	Args    []string
	Handler Handler

	mu sync.Mutex
}

var registry = struct {
	sync.RWMutex
	plugins []*Plugin
	names   map[string]struct{}
}{names: make(map[string]struct{})}

// Register adds a plugin to the process-wide registry. Registration is
// explicit and duplicate names are rejected to keep startup deterministic.
func Register(plugin *Plugin) error {
	if plugin == nil {
		return errors.New("plugin is nil")
	}
	if plugin.Name == "" {
		return errors.New("plugin name is required")
	}
	if plugin.Handler == nil {
		return fmt.Errorf("plugin %q has no handler", plugin.Name)
	}

	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.names[plugin.Name]; exists {
		return fmt.Errorf("plugin %q is already registered", plugin.Name)
	}
	registry.names[plugin.Name] = struct{}{}
	registry.plugins = append(registry.plugins, plugin)
	return nil
}

// Plugins returns a stable snapshot so dispatch does not hold the registry
// lock while plugin code runs.
func Plugins() []*Plugin {
	registry.RLock()
	defer registry.RUnlock()
	return append([]*Plugin(nil), registry.plugins...)
}

func PluginCount() int {
	registry.RLock()
	defer registry.RUnlock()
	return len(registry.plugins)
}

// StartPlugins invokes each enabled plugin's optional lifecycle hook once.
func StartPlugins(ctx context.Context, send chan<- []byte) {
	for _, plugin := range Plugins() {
		if !plugin.Enabled {
			continue
		}
		starter, ok := plugin.Handler.(Starter)
		if !ok {
			continue
		}
		func() {
			defer recoverPlugin(plugin.Name, "start")
			starter.Start(ctx, send)
		}()
	}
	StartCacheJanitor(ctx, time.Hour, 30*time.Minute)
}

// StopPlugins waits for active callbacks and releases plugin-owned resources.
func StopPlugins() {
	plugins := Plugins()
	for index := len(plugins) - 1; index >= 0; index-- {
		plugin := plugins[index]
		if plugin == nil || !plugin.Enabled {
			continue
		}
		stopper, ok := plugin.Handler.(Stopper)
		if !ok {
			continue
		}
		plugin.mu.Lock()
		func() {
			defer recoverPlugin(plugin.Name, "stop")
			if err := stopper.Stop(); err != nil {
				log.Printf("Stop plugin %q: %v", plugin.Name, err)
			}
		}()
		plugin.mu.Unlock()
	}
}

func (p *Plugin) HandleMessage(message *structs.MessageStruct, send chan<- []byte) {
	if p == nil || !p.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer recoverPlugin(p.Name, "message")
	p.Handler.ReceiveMessage(message, send)
}

func (p *Plugin) HandleEcho(message *structs.EchoMessageStruct, send chan<- []byte) {
	if p == nil || !p.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer recoverPlugin(p.Name, "echo")
	p.Handler.ReceiveEcho(message, send)
}

func recoverPlugin(name, operation string) {
	if recovered := recover(); recovered != nil {
		log.Printf("Plugin %q panic during %s: %v\n%s", name, operation, recovered, debug.Stack())
	}
}
