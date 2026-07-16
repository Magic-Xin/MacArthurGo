package plugins

import (
	"MacArthurGo/plugins/essentials"
	"fmt"
)

// RegisterAll constructs plugins after configuration and infrastructure are
// ready. This replaces package init side effects with an observable startup
// phase that can fail cleanly.
func RegisterAll() error {
	if err := essentials.RegisterBuiltins(); err != nil {
		return err
	}

	registrations := []struct {
		name string
		fn   func() error
	}{
		{name: "bili", fn: registerBili},
		{name: "chatAI", fn: registerChatAI},
		{name: "corpus", fn: registerCorpus},
		{name: "dailyWaifu", fn: registerDailyWaifu},
		{name: "music", fn: registerMusic},
		{name: "originPic", fn: registerOriginPic},
		{name: "picSearch", fn: registerPicSearch},
		{name: "poke", fn: registerPoke},
		{name: "repeat", fn: registerRepeat},
		{name: "roll", fn: registerRoll},
		{name: "statics", fn: registerStatics},
	}
	for _, registration := range registrations {
		if err := registration.fn(); err != nil {
			return fmt.Errorf("register plugin %s: %w", registration.name, err)
		}
	}
	return nil
}
