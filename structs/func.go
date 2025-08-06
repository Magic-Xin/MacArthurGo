package structs

import (
	"regexp"
	"strings"
)

func Text(text string) *MessageSegment {
	return &MessageSegment{Type: "text", Data: map[string]any{"text": text}}
}

func Mention(userID int64) *MessageSegment {
	return &MessageSegment{Type: "mention", Data: map[string]any{"user_id": userID}}
}

func MentionAll() *MessageSegment {
	return &MessageSegment{Type: "mention_all", Data: map[string]any{}}
}

func Reply(messageSeq int64) *MessageSegment {
	return &MessageSegment{Type: "reply", Data: map[string]any{"message_seq": messageSeq}}
}

//func Poke(id int64) *MessageSegment {
//	return &MessageSegment{Type: "touch", Data: map[string]any{"id": id}}
//}

//func Music(urlType string, id string) *MessageSegment {
//	return &MessageSegment{Type: "music", Data: map[string]any{"type": urlType, "id": id}}
//}

func Image(uri string) *MessageSegment {
	return &MessageSegment{Type: "image", Data: map[string]any{"uri": uri, "sub_type": "normal"}}
}

//func Unmarshal(message []byte) *[]MessageSegment {
//	var am []MessageSegment
//	err := json.Unmarshal(message, &am)
//
//	if err != nil {
//		return nil
//	}
//
//	return &am
//}

func FromStr(str string) *[]MessageSegment {
	var result []MessageSegment
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
		result = append(result, MessageSegment{Type: str[match[2]:match[3]], Data: data})
		begin = match[1]
	}
	result = append(result, *Text(str[begin:]))
	return &result
}
