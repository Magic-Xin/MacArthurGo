package plugins

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

func Roll(words *[]string) string {
	if len(*words) == 1 {
		return getRoll(-1)
	}
	if len(*words) == 2 {
		n, err := strconv.Atoi((*words)[1])
		if err != nil {
			return getRoll(-1)
		}
		return getRoll(n)
	}
	return getRollContent((*words)[1:])
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
