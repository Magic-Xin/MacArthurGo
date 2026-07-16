package client

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// MessageFactory decodes one OneBot event and fans it out to enabled plugins.
// It returns as soon as those handlers finish; the previous implementation
// kept every event goroutine alive for a fixed 60 seconds.
func MessageFactory(msg []byte, send chan<- []byte) error {
	var event structs.MessageStruct
	if err := json.Unmarshal(msg, &event); err != nil {
		return fmt.Errorf("decode OneBot event: %w", err)
	}

	plugins := essentials.Plugins()
	if isMessageEvent(&event) && !essentials.BanList.IsBanned(event.UserId) && (!base.Config.Debug || event.UserId == base.Config.Admin) {
		event.CleanMessage, event.Command = CleanMessage(event.Message)
		dispatchPlugins(plugins, func(plugin *essentials.Plugin) {
			plugin.HandleMessage(&event, send)
		})
	}

	if event.Echo != "" {
		echo, err := decodeEcho(msg)
		if err != nil {
			return err
		}
		dispatchPlugins(plugins, func(plugin *essentials.Plugin) {
			plugin.HandleEcho(echo, send)
		})
	}
	return nil
}

// isMessageEvent distinguishes actual OneBot message events from action
// responses. Action responses also have a top-level "message" status field,
// which must never be routed through user-message filtering before their echo
// is dispatched.
func isMessageEvent(event *structs.MessageStruct) bool {
	if event == nil || event.Message == nil {
		return false
	}
	if event.PostType == "message" {
		return true
	}
	// Some OneBot implementations omit post_type but still provide the
	// protocol-required private/group message_type.
	return event.PostType == "" && (event.MessageType == "private" || event.MessageType == "group")
}

func dispatchPlugins(plugins []*essentials.Plugin, handle func(*essentials.Plugin)) {
	var group sync.WaitGroup
	for _, plugin := range plugins {
		if plugin == nil || !plugin.Enabled {
			continue
		}
		group.Add(1)
		go func(plugin *essentials.Plugin) {
			defer group.Done()
			handle(plugin)
		}(plugin)
	}
	group.Wait()
}

func decodeEcho(msg []byte) (*structs.EchoMessageStruct, error) {
	var echo structs.EchoMessageStruct
	if err := json.Unmarshal(msg, &echo); err == nil {
		return &echo, nil
	}

	var array structs.EchoMessageArrayStruct
	if err := json.Unmarshal(msg, &array); err != nil {
		return nil, fmt.Errorf("decode OneBot echo: %w", err)
	}
	echo.DataArray = array.Data
	echo.Echo = array.Echo
	echo.Status = array.Status
	return &echo, nil
}

// CleanMessage extracts the leading slash command while preserving all other
// message segments. The input is not mutated.
func CleanMessage(message []cqcode.ArrayMessage) ([]cqcode.ArrayMessage, string) {
	cleaned := make([]cqcode.ArrayMessage, 0, len(message))
	var command string
	for _, segment := range message {
		if segment.Type != "text" || command != "" {
			cleaned = append(cleaned, segment)
			continue
		}

		text, ok := segment.Data["text"].(string)
		if !ok {
			cleaned = append(cleaned, segment)
			continue
		}
		words := strings.Fields(text)
		if len(words) == 0 || !strings.HasPrefix(words[0], "/") {
			cleaned = append(cleaned, segment)
			continue
		}

		command = words[0]
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		data["text"] = strings.Join(words[1:], " ")
		cleaned = append(cleaned, cqcode.ArrayMessage{Type: segment.Type, Data: data})
	}
	return cleaned, command
}
