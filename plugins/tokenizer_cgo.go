//go:build cgo

package plugins

import (
	"fmt"

	"github.com/yanyiwu/gojieba"
)

func newWordTokenizer(paths ...string) (tokenizer wordTokenizer, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			tokenizer = nil
			err = fmt.Errorf("initialize gojieba: %v", recovered)
		}
	}()
	return gojieba.NewJieba(paths...), nil
}
