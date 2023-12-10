package cqcode

import (
	"regexp"
	"strings"
)

//TODO CQ码上报Array化

func CQAt(qq int64) string {
	data := map[string]any{
		"qq": qq,
	}
	cq := CQCode{Type: "at", Data: data}
	return cq.toString()
}

func CQPoke(qq int64) string {
	data := map[string]any{
		"qq": qq,
	}
	cq := CQCode{Type: "poke", Data: data}
	return cq.toString()
}

func CQMusic(urlType string, id int64) string {
	data := map[string]any{
		"type": urlType,
		"id":   id,
	}
	cq := CQCode{Type: "music", Data: data}
	return cq.toString()
}

func CQImage(file string) string {
	data := map[string]any{
		"file": file,
	}
	cq := CQCode{Type: "image", Data: data}
	return cq.toString()
}

func FromStr(str string) *[]CQCode {
	var result []CQCode
	cqCodeRegex := regexp.MustCompile(`\[CQ:([^,[\]]+)((?:,[^,=[\]]+=[^,[\]]*)*)]`)
	splitFn := func(c rune) bool {
		return c == ','
	}
	for _, match := range cqCodeRegex.FindAllStringSubmatch(str, -1) {
		data := make(map[string]any)
		for _, kv := range strings.FieldsFunc(match[2], splitFn) {
			parts := strings.SplitN(kv, "=", 2)
			key := Unescape(parts[0])
			value := Unescape(parts[1])
			data[key] = value
		}
		result = append(result, CQCode{Type: match[1], Data: data})
	}
	return &result
}

func Escape(str string, insideCQ bool) string {
	s := str
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "[", "&#91;")
	s = strings.ReplaceAll(s, "]", "&#93;")

	if !insideCQ {
		return s
	}

	s = strings.ReplaceAll(s, ",", "&#44;")
	s = regexp.MustCompile(`(\x{d83c}[\x{df00}-\x{dfff}])|(\x{d83d}[\x{dc00}-\x{de4f}\x{de80}-\x{deff}])|[\x{2600}-\x{2B55}]`).ReplaceAllString(s, " ")

	return s
}

func Unescape(str string) string {
	s := str
	s = strings.ReplaceAll(s, "&#44;", ",")
	s = strings.ReplaceAll(s, "&#91;", "[")
	s = strings.ReplaceAll(s, "&#93;", "]")
	s = strings.ReplaceAll(s, "&amp;", "&")

	return s
}

func EscapeInsideCQ(str string) string {
	return Escape(str, true)
}
