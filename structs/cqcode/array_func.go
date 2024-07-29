package cqcode

import (
	"encoding/json"
	"regexp"
	"strings"
)

func Text(text string) *ArrayMessage {
	return &ArrayMessage{Type: "text", Data: map[string]any{"text": text}}
}

func At(qq string) *ArrayMessage {
	return &ArrayMessage{Type: "at", Data: map[string]any{"qq": qq}}
}

func Reply(id string) *ArrayMessage {
	return &ArrayMessage{Type: "reply", Data: map[string]any{"id": id}}
}

func Poke(id int64) *ArrayMessage {
	return &ArrayMessage{Type: "touch", Data: map[string]any{"id": id}}
}

func Music(urlType string, id string) *ArrayMessage {
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

func FromStr(str string) *[]ArrayMessage {
	var result []ArrayMessage
	cqCodeRegex := regexp.MustCompile(`\[CQ:([^,[\]]+)((?:,[^,=[\]]+=[^,[\]]*)*)]`)
	splitFn := func(c rune) bool {
		return c == ','
	}
	begin := 0
	for _, match := range cqCodeRegex.FindAllStringSubmatchIndex(str, -1) {
		if begin < match[0] {
			result = append(result, *Text(str[begin:match[0]]))
		}
		data := make(map[string]any)
		for _, kv := range strings.FieldsFunc(str[match[4]:match[5]], splitFn) {
			parts := strings.SplitN(kv, "=", 2)
			data[parts[0]] = parts[1]
		}
		result = append(result, ArrayMessage{Type: str[match[2]:match[3]], Data: data})
		begin = match[1]
	}
	result = append(result, *Text(str[begin:]))
	return &result
}
