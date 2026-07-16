package plugins

import (
	"fmt"

	"github.com/yanyiwu/gojieba"
)

type wordTokenizer interface {
	CutForSearch(string, bool) []string
	Free()
}

func newWordTokenizer(paths ...string) (tokenizer wordTokenizer, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			tokenizer = nil
			err = fmt.Errorf("initialize gojieba: %v", recovered)
		}
	}()
	return gojieba.NewJieba(paths...), nil
}
