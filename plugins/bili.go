package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Bili struct {
	AiSummarize *AISummarize
}

type BiliLogin struct {
	Cookies      []*http.Cookie
	RefreshToken string
	TimeStamp    int64
}

type AISummarize struct {
	Enabled        bool
	GroupForward   bool
	mixinKeyEncTab []int
	cache          sync.Map
	lastUpdateTime time.Time
	LoginInfo      *BiliLogin
}

type VideoData struct {
	Title        string
	Author       string
	ThumbnailUrl string
	Aid          string
	Bvid         string
	Cid          string
	Mid          string
	Playtime     string
	Danmaku      string
	Url          string
	Summary      string
}

type LiveData struct {
	Title        string
	User         string
	ThumbnailUrl string
	RoomId       string
	AreaName     string
	Status       string
	Url          string
}

func init() {
	login := BiliLogin{}
	aiSummarize := AISummarize{
		Enabled:      base.Config.Plugins.Bili.AiSummarize.Enable,
		GroupForward: base.Config.Plugins.Bili.AiSummarize.GroupForward,
		mixinKeyEncTab: []int{
			46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49,
			33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40,
			61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11,
			36, 20, 34, 44, 52,
		},
		LoginInfo: &login,
	}
	bili := Bili{
		AiSummarize: &aiSummarize,
	}
	plugin := &essentials.Plugin{
		Name:      "B 站链接解析",
		Enabled:   base.Config.Plugins.Bili.Enable,
		Interface: &bili,
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (*Bili) ReceiveAll(chan<- *[]byte) {}

func (b *Bili) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	const biliShort = `((b23.tv|bili2233.cn)\\?/\w+)`
	const video = `[m|www].bilibili.com/video/(\w+)`
	const live = `live.bilibili.com/(\d+)`

	if essentials.CheckArgumentArray(messageStruct.Command, &[]string{"/bili_login"}) && messageStruct.UserId == base.Config.Admin {
		b.AiSummarize.Login(messageStruct, send)
		return
	}

	rawMsg := messageStruct.RawMessage
	if match := regexp.MustCompile(biliShort).FindAllStringSubmatch(rawMsg, -1); match != nil {
		replaceUrl := strings.Replace(match[0][1], "\\", "", -1)
		if orgUrl := essentials.GetOriginUrl("https://" + replaceUrl); orgUrl != nil {
			rawMsg = *orgUrl
		}
	}

	var (
		videoData *VideoData
		liveData  *LiveData
	)

	if match := regexp.MustCompile(video).FindAllStringSubmatch(rawMsg, -1); match != nil {
		videoData = b.getVideoData(match[0][1])
	} else if match = regexp.MustCompile(live).FindAllStringSubmatch(rawMsg, -1); match != nil {
		liveData = b.getLiveData(match[0][1])
	} else {
		return
	}

	if videoData != nil {
		e, r := b.AiSummarize.Summarize(videoData, true)
		if r != nil {
			videoData.Summary = "AI 视频总结：" + (*r)[0]
		} else {
			videoData.Summary = e
		}
		send <- essentials.SendMsg(messageStruct, "", videoData.ToArrayMessage(), false, true, "")
	} else if liveData != nil {
		send <- essentials.SendMsg(messageStruct, "", liveData.ToArrayMessage(), false, true, "")
	}
	return
}

func (b *Bili) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}

func (b *Bili) getVideoData(vid string) *VideoData {
	const api = "https://api.bilibili.com/x/web-interface/view?"

	var reqUrl string
	if strings.HasPrefix(vid, "BV") {
		reqUrl = api + "bvid=" + vid
	} else {
		reqUrl = api + "aid=" + vid[2:]
	}
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		log.Printf("Video url parser request error: %v", err)
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Video url parser response error: %v", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Video url parser close error: %v", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Video url parser read body error: %v", err)
		return nil
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Video url parser unmarshal error: %v", err)
		return nil
	}

	ctx := i.(map[string]any)
	if ctx["code"].(float64) != 0 {
		return nil
	}
	data := &VideoData{
		Title:        ctx["data"].(map[string]any)["title"].(string),
		Author:       ctx["data"].(map[string]any)["owner"].(map[string]any)["name"].(string),
		ThumbnailUrl: ctx["data"].(map[string]any)["pic"].(string),
		Aid:          strconv.FormatInt(int64(ctx["data"].(map[string]any)["aid"].(float64)), 10),
		Bvid:         ctx["data"].(map[string]any)["bvid"].(string),
		Cid:          strconv.FormatInt(int64(ctx["data"].(map[string]any)["cid"].(float64)), 10),
		Mid:          strconv.FormatInt(int64(ctx["data"].(map[string]any)["owner"].(map[string]any)["mid"].(float64)), 10),
		Playtime:     b.iToS(int64(ctx["data"].(map[string]any)["stat"].(map[string]any)["view"].(float64))),
		Danmaku:      b.iToS(int64(ctx["data"].(map[string]any)["stat"].(map[string]any)["danmaku"].(float64))),
		Url:          "https://www.bilibili.com/video/" + vid,
	}
	return data
}

func (*Bili) iToS(i int64) string {
	if i >= 10000 {
		return fmt.Sprintf("%.1f万", float64(i)/10000)
	}
	return fmt.Sprintf("%d", i)
}

func (b *Bili) getLiveData(roomId string) *LiveData {
	const api = "https://api.live.bilibili.com/xlive/web-room/v1/index/getInfoByRoom?room_id="
	req, err := http.NewRequest("GET", api+roomId, nil)
	if err != nil {
		log.Printf("Live url parser request error: %v", err)
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Live url parser response error: %v", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Live url parser close error: %v", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Live url parser read body error: %v", err)
		return nil
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Live url parser unmarshal error: %v", err)
		return nil
	}
	ctx := i.(map[string]any)
	if ctx["code"].(float64) != 0 {
		return nil
	}

	data := &LiveData{
		Title:        ctx["data"].(map[string]any)["room_info"].(map[string]any)["title"].(string),
		User:         ctx["data"].(map[string]any)["anchor_info"].(map[string]any)["base_info"].(map[string]any)["uname"].(string),
		ThumbnailUrl: ctx["data"].(map[string]any)["room_info"].(map[string]any)["keyframe"].(string),
	}

	if shortId := ctx["data"].(map[string]any)["room_info"].(map[string]any)["short_id"].(float64); shortId != 0 {
		data.RoomId = fmt.Sprintf("短号: %d", int64(shortId))
		data.Url = "https://live.bilibili.com/" + fmt.Sprintf("%d", int64(shortId))
	} else {
		data.RoomId = fmt.Sprintf("房间号: %d", int64(ctx["data"].(map[string]any)["room_info"].(map[string]any)["room_id"].(float64)))
		data.Url = "https://live.bilibili.com/" + fmt.Sprintf("%d", int64(ctx["data"].(map[string]any)["room_info"].(map[string]any)["room_id"].(float64)))
	}

	areaName := ctx["data"].(map[string]any)["room_info"].(map[string]any)["area_name"].(string)
	parentAreaName := ctx["data"].(map[string]any)["room_info"].(map[string]any)["parent_area_name"].(string)
	if areaName != parentAreaName {
		data.AreaName = parentAreaName + "-" + areaName
	} else {
		data.AreaName = parentAreaName
	}

	if ctx["data"].(map[string]any)["room_info"].(map[string]any)["live_status"].(float64) == 1 {
		data.Status = "直播中	" + b.iToS(int64(ctx["data"].(map[string]any)["room_info"].(map[string]any)["online"].(float64))) + "人气"
	} else {
		data.Status = "未开播"
	}
	return data
}

func (a *AISummarize) Login(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	const genQr = "https://passport.bilibili.com/x/passport-login/web/qrcode/generate"
	const login = "https://passport.bilibili.com/x/passport-login/web/qrcode/poll"

	req, err := http.NewRequest("GET", genQr, nil)
	if err != nil {
		log.Printf("Bili Login Error: %s", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Bili Login Error: %s", err)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Bili Login Error: %s", err)
		return
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Bili Login Error: %s", err)
		return
	}

	ctx := i.(map[string]any)
	if ctx["code"].(float64) != 0 {
		log.Printf("Bili Login Error: %s", ctx["message"].(string))
		return
	}

	qrUrl := ctx["data"].(map[string]any)["url"].(string)
	qrKey := ctx["data"].(map[string]any)["qrcode_key"].(string)
	send <- essentials.SendMsg(messageStruct, qrUrl, nil, false, false, "")

	for x := 0; x < 18; x++ { // timeout 180s
		time.Sleep(time.Second * 10)
		req, err := http.NewRequest("GET", login+"?qrcode_key="+qrKey, nil)
		if err != nil {
			log.Printf("Bili Login Error: %s", err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("Bili Login Error: %s", err)
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Bili Login Error: %s", err)
			return
		}

		var i any
		err = json.Unmarshal(body, &i)
		if err != nil {
			log.Printf("Bili Login Error: %s", err)
			return
		}

		ctx := i.(map[string]any)
		if ctx["code"].(float64) == 0 {
			if ctx["data"].(map[string]any)["code"].(float64) == 0 {
				a.LoginInfo.Cookies = resp.Cookies()
				a.LoginInfo.RefreshToken = ctx["data"].(map[string]any)["refresh_token"].(string)
				a.LoginInfo.TimeStamp = int64(ctx["data"].(map[string]any)["timestamp"].(float64))
				send <- essentials.SendMsg(messageStruct, "登录成功", nil, false, false, "")
				return
			} else if ctx["code"].(float64) == 86038 {
				send <- essentials.SendMsg(messageStruct, "二维码已失效, 请重新获取", nil, false, false, "")
				return
			}
		}
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Bili Login Error: %s", err)
		}
	}(resp.Body)
}

func (a *AISummarize) Summarize(videoData *VideoData, sumOnly bool) (string, *[]string) {
	if !a.Enabled {
		return "", nil
	}

	if videoData == nil {
		return "获取视频信息失败", nil
	}

	const api = "https://api.bilibili.com/x/web-interface/view/conclusion/get?"
	params := url.Values{
		"aid":    {videoData.Aid},
		"bvid":   {videoData.Bvid},
		"cid":    {videoData.Cid},
		"up_mid": {videoData.Mid},
	}
	ctx, err := a.requireSummarize(api + params.Encode())
	if err != nil {
		return err.Error(), nil
	}

	if (*ctx)["code"].(float64) != 0 {
		return "获取视频 AI 总结失败" + (*ctx)["message"].(string), nil
	}

	dataCode := int64((*ctx)["data"].(map[string]any)["code"].(float64))
	if dataCode == -1 {
		return "该视频可能内含敏感内容或其他异常，不支持 AI 总结", nil
	}
	if dataCode == 1 {
		if (*ctx)["data"].(map[string]any)["stid"].(string) == "" {
			return "该视频未识别到语音，暂不支持 AI 总结", nil
		} else if (*ctx)["data"].(map[string]any)["stid"].(string) == "0" {
			return "该视频正在 AI 总结等待队列，请稍后再试", nil
		} else {
			return "由于未知问题，无法获得该视频的 AI 总结", nil
		}
	}
	if (*ctx)["data"].(map[string]any)["model_result"] == nil {
		return "该视频的 AI 总结摘要为空", nil
	}

	var res struct {
		ResultType int64  `json:"result_type"`
		Summary    string `json:"summary"`
		Outline    []struct {
			Title       string `json:"title"`
			PartOutline []struct {
				TimeStamp int64  `json:"timestamp"`
				Content   string `json:"content"`
			} `json:"part_outline"`
			TimeStamp int64 `json:"timestamp"`
		} `json:"outline"`
	}
	jsonRes, err := json.Marshal((*ctx)["data"].(map[string]any)["model_result"])
	if err != nil {
		log.Printf("Model result marshal error: %v", err)
		return "AI 总结摘要解析失败，详细信息见日志", nil
	}
	err = json.Unmarshal(jsonRes, &res)
	if err != nil {
		log.Printf("Model result unmarshal error: %v", err)
		return "AI 总结摘要解析失败，详细信息见日志", nil
	}

	if sumOnly {
		return "", &[]string{res.Summary}
	}

	var sum []string
	if res.Summary != "" {
		sum = append(sum, fmt.Sprintf("摘要: %s\n", res.Summary))
	}
	if len(res.Outline) == 0 {
		return "", &sum
	}

	for i, o := range res.Outline {
		contents := fmt.Sprintf("%d. %s\n\n", i+1, o.Title)
		for _, p := range o.PartOutline {
			contents += fmt.Sprintf("(%s) %s\n", a.timestampToString(p.TimeStamp), p.Content)
		}
		sum = append(sum, contents)
	}
	return "", &sum
}

func (a *AISummarize) requireSummarize(url string) (*map[string]any, error) {
	newUrlStr, err := signAndGenerateURL(url)
	if err != nil {
		log.Printf("Error: %s", err)
		return nil, err
	}
	req, err := http.NewRequest("GET", newUrlStr, nil)
	if err != nil {
		log.Printf("Error: %s", err)
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")

	for _, c := range a.LoginInfo.Cookies {
		if c.Name == "SESSDATA" {
			req.AddCookie(c)
		}
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Request failed: %s", err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			fmt.Printf("Failed to close response body: %s", err)
		}
	}(response.Body)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Failed to read response: %s", err)
		return nil, err
	}

	var i any
	err = json.Unmarshal(body, &i)
	if err != nil {
		log.Printf("Failed to unmarshal response: %s", err)
		return nil, err
	}
	ctx := i.(map[string]any)
	return &ctx, nil
}

func (*AISummarize) timestampToString(timestamp int64) string {
	hour := timestamp / 3600
	minute := timestamp % 3600 / 60
	second := timestamp % 60
	if hour == 0 {
		return fmt.Sprintf("%02d:%02d", minute, second)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}

func (v *VideoData) ToArrayMessage() *[]cqcode.ArrayMessage {
	var messageArray []cqcode.ArrayMessage
	messageArray = append(messageArray, *cqcode.Image(v.ThumbnailUrl + "\n"))
	messageArray = append(messageArray, *cqcode.Text("av" + v.Aid + "\n"))
	messageArray = append(messageArray, *cqcode.Text(v.Title + "\n"))
	messageArray = append(messageArray, *cqcode.Text("UP: " + v.Author + "\n"))
	messageArray = append(messageArray, *cqcode.Text("播放: " + v.Playtime + "	弹幕: " + v.Danmaku + "\n"))
	messageArray = append(messageArray, *cqcode.Text(v.Url + "\n\n"))
	if v.Summary != "" {
		messageArray = append(messageArray, *cqcode.Text(v.Summary))
	}
	return &messageArray
}

func (l *LiveData) ToArrayMessage() *[]cqcode.ArrayMessage {
	var messageArray []cqcode.ArrayMessage
	messageArray = append(messageArray, *cqcode.Image(l.ThumbnailUrl + "\n"))
	messageArray = append(messageArray, *cqcode.Text(l.Title + "\n"))
	messageArray = append(messageArray, *cqcode.Text("主播: " + l.User + "\n"))
	messageArray = append(messageArray, *cqcode.Text(l.RoomId + "\n"))
	messageArray = append(messageArray, *cqcode.Text("分区: " + l.AreaName + "\n"))
	messageArray = append(messageArray, *cqcode.Text(l.Status + "\n"))
	messageArray = append(messageArray, *cqcode.Text(l.Url))
	return &messageArray
}

// Wbi
func signAndGenerateURL(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	err = Sign(u)
	if err != nil {
		return "", fmt.Errorf("sign error: %w", err)
	}
	return u.String(), nil
}

// Sign 为链接签名
func Sign(u *url.URL) error {
	return wbiKeys.Sign(u)
}

// Update 无视过期时间更新
func Update() error {
	return wbiKeys.Update()
}

func Get() (wk WbiKeys, err error) {
	if err = wk.update(false); err != nil {
		return WbiKeys{}, err
	}
	return wbiKeys, nil
}

var wbiKeys WbiKeys

type WbiKeys struct {
	Img            string
	Sub            string
	Mixin          string
	lastUpdateTime time.Time
}

// Sign 为链接签名
func (wk *WbiKeys) Sign(u *url.URL) (err error) {
	if err = wk.update(false); err != nil {
		return err
	}

	values := u.Query()

	values = removeUnwantedChars(values, '!', '\'', '(', ')', '*') // 必要性存疑?

	values.Set("wts", strconv.FormatInt(time.Now().Unix(), 10))

	// [url.Values.Encode] 内会对参数排序,
	// 且遍历 map 时本身就是无序的
	hash := md5.Sum([]byte(values.Encode() + wk.Mixin)) // Calculate w_rid
	values.Set("w_rid", hex.EncodeToString(hash[:]))
	u.RawQuery = values.Encode()
	return nil
}

// Update 无视过期时间更新
func (wk *WbiKeys) Update() (err error) {
	return wk.update(true)
}

// update 按需更新
func (wk *WbiKeys) update(purge bool) error {
	if !purge && time.Since(wk.lastUpdateTime) < time.Hour {
		return nil
	}

	// 测试下来不用修改 header 也能过
	resp, err := http.Get("https://api.bilibili.com/x/web-interface/nav")
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %s", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	nav := Nav{}
	err = json.Unmarshal(body, &nav)
	if err != nil {
		return err
	}

	if nav.Code != 0 && nav.Code != -101 { // -101 未登录时也会返回两个 key
		return fmt.Errorf("unexpected code: %d, message: %s", nav.Code, nav.Message)
	}
	img := nav.Data.WbiImg.ImgUrl
	sub := nav.Data.WbiImg.SubUrl
	if img == "" || sub == "" {
		return fmt.Errorf("empty image or sub url: %s", body)
	}

	// https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png
	imgParts := strings.Split(img, "/")
	subParts := strings.Split(sub, "/")

	// 7cd084941338484aae1ad9425b84077c.png
	imgPng := imgParts[len(imgParts)-1]
	subPng := subParts[len(subParts)-1]

	// 7cd084941338484aae1ad9425b84077c
	wbiKeys.Img = strings.TrimSuffix(imgPng, ".png")
	wbiKeys.Sub = strings.TrimSuffix(subPng, ".png")

	wbiKeys.mixin()
	wbiKeys.lastUpdateTime = time.Now()
	return nil
}

func (wk *WbiKeys) mixin() {
	var mixin [32]byte
	wbi := wk.Img + wk.Sub
	for i := range mixin { // for i := 0; i < len(mixin); i++ {
		mixin[i] = wbi[mixinKeyEncTab[i]]
	}
	wk.Mixin = string(mixin[:])
}

var mixinKeyEncTab = [...]int{
	46, 47, 18, 2, 53, 8, 23, 32,
	15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19,
	29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61,
	26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63,
	57, 62, 11, 36, 20, 34, 44, 52,
}

func removeUnwantedChars(v url.Values, chars ...byte) url.Values {
	b := []byte(v.Encode())
	for _, c := range chars {
		b = bytes.ReplaceAll(b, []byte{c}, nil)
	}
	s, err := url.ParseQuery(string(b))
	if err != nil {
		panic(err)
	}
	return s
}

type Nav struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Ttl     int    `json:"ttl"`
	Data    struct {
		WbiImg struct {
			ImgUrl string `json:"img_url"`
			SubUrl string `json:"sub_url"`
		} `json:"wbi_img"`

		// ......
	} `json:"data"`
}
