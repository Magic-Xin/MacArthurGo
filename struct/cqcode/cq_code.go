package cqcode

import (
	"fmt"
	"regexp"
	"strings"
)

type CQCode struct {
	Type string
	Data map[string]any
}

func (cq *CQCode) toString() string {
	res := fmt.Sprintf("CQ:%s", cq.Type)
	for k, v := range cq.Data {
		res += fmt.Sprintf(",%s=%v", k, v)
	}
	return fmt.Sprintf("[%s]", res)
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
	s = regexp.MustCompile(`([\uD800-\uDBFF][\uDC00-\uDFFF])`).ReplaceAllString(s, " ")

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
