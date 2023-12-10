package cqcode

import (
	"fmt"
)

// CQCode Deprecated
type CQCode struct {
	Type string
	Data map[string]any
}

type ArrayMessage struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

func (cq CQCode) toString() string {
	res := fmt.Sprintf("CQ:%s", cq.Type)
	for k, v := range cq.Data {
		switch v.(type) {
		case string:
			v = EscapeInsideCQ(v.(string))
		}
		res += fmt.Sprintf(",%s=%v", k, v)
	}
	return fmt.Sprintf("[%s]", res)
}
