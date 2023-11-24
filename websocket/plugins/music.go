package plugins

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

func Music(str string) (string, int64, bool) {
	urlType := ""
	if strings.Contains(str, "music.163.com") {
		urlType = "163"
	} else if strings.Contains(str, "i.y.qq.com") {
		urlType = "qq"
	}

	if urlType != "" {
		re := regexp.MustCompile(`id=(\d+)&`)
		match := re.FindAllStringSubmatch(str, -1)
		log.Println(match[0][1])
		if len(match) > 0 {
			id, err := strconv.ParseInt(match[0][1], 10, 64)
			if err == nil {
				return urlType, id, true
			}
		}
	}

	return "", -1, false
}
