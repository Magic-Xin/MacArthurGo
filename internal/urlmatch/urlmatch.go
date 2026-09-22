package urlmatch

import "regexp"

var (
	biliVideoPattern     = regexp.MustCompile(`(?:m|www)\.bilibili\.com/video/(\w+)`)
	biliLivePattern      = regexp.MustCompile(`live\.bilibili\.com/(\d+)`)
	neteaseMobilePattern = regexp.MustCompile(`(?:http|https)://y\.music\.163\.com/m/song/(\d+)`)
	musicQueryIDPattern  = regexp.MustCompile(`(?:^|[?&])id=(\d+)`)
)

func BiliVideoID(value string) (string, bool) {
	return firstCapture(biliVideoPattern, value)
}

func BiliLiveRoomID(value string) (string, bool) {
	return firstCapture(biliLivePattern, value)
}

func NeteaseMobileSongID(value string) (string, bool) {
	return firstCapture(neteaseMobilePattern, value)
}

func MusicQueryID(value string) (string, bool) {
	return firstCapture(musicQueryIDPattern, value)
}

func firstCapture(pattern *regexp.Regexp, value string) (string, bool) {
	match := pattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return "", false
	}
	return match[1], true
}
