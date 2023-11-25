package plugins

import (
	"fmt"
	"github.com/gookit/config/v2"
	"math/rand"
	"strconv"
	"time"
)

func Roll(ctx *map[string]any, words *[]string, send *chan []byte) {
	if (*words)[0] != config.String("plugins.roll.args") || !config.Bool("plugins.roll.enable") {
		return
	}

	var result string
	if len(*words) == 1 {
		result = getRoll(-1)
	} else if len(*words) == 2 {
		n, err := strconv.Atoi((*words)[1])
		if err != nil {
			result = getRoll(-1)
		} else {
			result = getRoll(n)
		}
	} else {
		result = getRollContent((*words)[1:])
	}

	msg := SendMsg(ctx, result, false, true)
	*send <- *msg
}

func getRoll(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if n < 1 {
		return fmt.Sprintf("生成 [0-9] 随机值：%d", r.Intn(10))
	}
	return fmt.Sprintf("生成 [0-%d] 随机值：%d", n, r.Intn(n))
}

func getRollContent(content []string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("随机结果为：%s", content[r.Intn(len(content))])
}
