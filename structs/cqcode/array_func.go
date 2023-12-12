package cqcode

import (
	"encoding/json"
)

func Text(text string) *ArrayMessage {
	return &ArrayMessage{Type: "text", Data: map[string]any{"text": text}}
}

func At(qq string) *ArrayMessage {
	return &ArrayMessage{Type: "at", Data: map[string]any{"qq": qq}}
}

func Reply(id int64) *ArrayMessage {
	return &ArrayMessage{Type: "reply", Data: map[string]any{"id": id}}
}

func Poke(id int64) *ArrayMessage {
	return &ArrayMessage{Type: "poke", Data: map[string]any{"type": 1, "id": id}}
}

func Music(urlType string, id int64) *ArrayMessage {
	return &ArrayMessage{Type: "music", Data: map[string]any{"type": urlType, "id": id}}
}

func Image(file string) *ArrayMessage {
	return &ArrayMessage{Type: "image", Data: map[string]any{"file": file}}
}

func Unmarshal(message []byte) *[]ArrayMessage {
	var am []ArrayMessage
	err := json.Unmarshal(message, &am)

	if err != nil {
		return nil
	}

	return &am
}
