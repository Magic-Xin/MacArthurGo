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
	"github.com/google/go-cmp/cmp"
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

type AISummarize struct {
	Enabled        bool
	Args           []string
	GroupForward   bool
	mixinKeyEncTab []int
	cache          sync.Map
	lastUpdateTime time.Time
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
	aiSummarize := AISummarize{
		Enabled:      base.Config.Plugins.Bili.AiSummarize.Enable,
		Args:         base.Config.Plugins.Bili.AiSummarize.Args,
		GroupForward: base.Config.Plugins.Bili.AiSummarize.GroupForward,
		mixinKeyEncTab: []int{
			46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49,
			33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40,
			61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11,
			36, 20, 34, 44, 52,
		},
	}
	bili := Bili{
		AiSummarize: &aiSummarize,
	}
	plugin := &essentials.Plugin{
		Name:      "B 站链接解析",
		Enabled:   base.Config.Plugins.Bili.Enable,
		Args:      aiSummarize.Args,
		Interface: &bili,
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (b *Bili) ReceiveAll() *[]byte {
	return nil
}

func (b *Bili) ReceiveMessage(messageStruct *structs.MessageStruct) *[]byte {
	const biliShort = `(b23.tv\\?/\w+)`
	const video = `www.bilibili.com/video/(\w+)`
	const live = `live.bilibili.com/(\d+)`

	rawMsg := messageStruct.RawMessage
	if match := regexp.MustCompile(biliShort).FindAllStringSubmatch(rawMsg, -1); match != nil {
		replaceUrl := strings.Replace(match[0][1], "\\", "", -1)
		if orgUrl := essentials.GetOriginUrl("https://" + replaceUrl); orgUrl != nil {
			rawMsg = *orgUrl
		}
	}

	if essentials.CheckArgumentArray(&messageStruct.Message, &b.AiSummarize.Args) {
		message := messageStruct.Message
		for _, msg := range message {
			if msg.Type == "reply" {
				echo := "BiliAI"
				if messageStruct.MessageType == "group" {
					echo += fmt.Sprintf("|%d", messageStruct.MessageId)
					value := essentials.Value{
						Value: *messageStruct,
						Time:  time.Now().Unix(),
					}
					essentials.SetCache(strconv.FormatInt(messageStruct.MessageId, 10), value)
				}
				return essentials.SendAction("get_msg", structs.GetMsg{Id: msg.Data["id"].(string)}, "BiliAI")
			}
		}

		if match := regexp.MustCompile(video).FindAllStringSubmatch(rawMsg, -1); match != nil {
			videoData := b.getVideoData(match[0][1])
			e, aiSum := b.AiSummarize.Summarize(videoData, false)
			if aiSum != nil {
				if messageStruct.MessageType == "group" && b.AiSummarize.GroupForward && len(*aiSum) > 1 {
					var data []structs.ForwardNode
					data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, videoData.ToArrayMessage()))
					for _, msg := range *aiSum {
						data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(msg)}))
					}
					return essentials.SendGroupForward(messageStruct, &data, "")
				} else {
					for _, msg := range *aiSum {
						return essentials.SendMsg(messageStruct, msg, nil, false, false)
					}
				}
			} else {
				return essentials.SendMsg(messageStruct, e, nil, false, false)
			}
		}
		return nil
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
		return nil
	}

	if videoData != nil {
		e, r := b.AiSummarize.Summarize(videoData, true)
		if r != nil {
			videoData.Summary = "AI 视频总结：" + (*r)[0]
		} else {
			videoData.Summary = e
		}
		return essentials.SendMsg(messageStruct, "", videoData.ToArrayMessage(), false, true)
	} else if liveData != nil {
		return essentials.SendMsg(messageStruct, "", liveData.ToArrayMessage(), false, true)
	}
	return nil
}

func (b *Bili) ReceiveEcho(EchoMessageStruct *structs.EchoMessageStruct) *[]byte {
	split := strings.Split(EchoMessageStruct.Echo, "|")

	if split[0] == "BiliAI" {
		if !cmp.Equal(EchoMessageStruct.Data, struct{}{}) {
			ctxData := EchoMessageStruct.Data
			message := ctxData.Message
			var text string
			for _, msg := range message {
				if msg.Type == "text" {
					text += msg.Data["text"].(string) + " "
				}
			}

			const biliShort = `(b23.tv/\w+)`
			const video = `www.bilibili.com/video/(\w+)`

			if match := regexp.MustCompile(biliShort).FindAllStringSubmatch(text, -1); match != nil {
				if orgUrl := essentials.GetOriginUrl("https://" + match[0][1]); orgUrl != nil {
					text = *orgUrl
				}
			}

			if match := regexp.MustCompile(video).FindAllStringSubmatch(text, -1); match != nil {
				videoData := b.getVideoData(match[0][1])
				e, aiSum := b.AiSummarize.Summarize(videoData, false)
				if aiSum != nil {
					if ctxData.MessageType == "group" && b.AiSummarize.GroupForward && len(*aiSum) > 1 {
						value, ok := essentials.GetCache(split[1])
						if !ok {
							log.Println("BiliAI cache not found")
							return nil
						}
						orgStruct := value.(essentials.Value).Value
						var data []structs.ForwardNode
						data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, videoData.ToArrayMessage()))
						for _, msg := range *aiSum {
							data = append(data, *essentials.ConstructForwardNode(essentials.Info.UserId, essentials.Info.NickName, &[]cqcode.ArrayMessage{*cqcode.Text(msg)}))
						}
						return essentials.SendGroupForward(&orgStruct, &data, "")
					} else {
						for _, msg := range *aiSum {
							sendStruct := structs.MessageStruct{
								MessageType: ctxData.MessageType,
								UserId:      ctxData.UserId,
							}
							return essentials.SendMsg(&sendStruct, msg, nil, false, false)
						}
					}
				} else {
					if ctxData.MessageType == "group" {
						value, ok := essentials.GetCache(split[1])
						if !ok {
							log.Println("BiliAI cache not found")
							return nil
						}
						orgStruct := value.(essentials.Value).Value
						return essentials.SendMsg(&orgStruct, e, nil, false, false)
					} else {
						sendStruct := structs.MessageStruct{
							MessageType: ctxData.MessageType,
							UserId:      ctxData.UserId,
						}
						return essentials.SendMsg(&sendStruct, e, nil, false, false)
					}
				}
			}
		}
	}
	return nil
}

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

func (a *AISummarize) Summarize(videoData *VideoData, sumOnly bool) (string, *[]string) {
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
	newUrlStr, err := a.signAndGenerateURL(url)
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

func (a *AISummarize) signAndGenerateURL(urlStr string) (string, error) {
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	imgKey, subKey := a.getWbiKeysCached()
	query := urlObj.Query()
	params := map[string]string{}
	for k, v := range query {
		params[k] = v[0]
	}
	newParams := a.encWbi(params, imgKey, subKey)
	for k, v := range newParams {
		query.Set(k, v)
	}
	urlObj.RawQuery = query.Encode()
	newUrlStr := urlObj.String()
	return newUrlStr, nil
}

func (a *AISummarize) encWbi(params map[string]string, imgKey, subKey string) map[string]string {
	mixinKey := a.getMixinKey(imgKey + subKey)
	currTime := strconv.FormatInt(time.Now().Unix(), 10)
	params["wts"] = currTime

	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Remove unwanted characters
	for k, v := range params {
		v = a.sanitizeString(v)
		params[k] = v
	}

	// Build URL parameters
	query := url.Values{}
	for _, k := range keys {
		query.Set(k, params[k])
	}
	queryStr := query.Encode()

	// Calculate w_rid
	hash := md5.Sum([]byte(queryStr + mixinKey))
	params["w_rid"] = hex.EncodeToString(hash[:])
	return params
}

func (a *AISummarize) getMixinKey(orig string) string {
	var str strings.Builder
	for _, v := range a.mixinKeyEncTab {
		if v < len(orig) {
			str.WriteByte(orig[v])
		}
	}
	return str.String()[:32]
}

func (a *AISummarize) sanitizeString(s string) string {
	unwantedChars := []string{"!", "'", "(", ")", "*"}
	for _, char := range unwantedChars {
		s = strings.ReplaceAll(s, char, "")
	}
	return s
}

func (a *AISummarize) updateCache() {
	if time.Since(a.lastUpdateTime).Minutes() < 10 {
		return
	}
	imgKey, subKey := a.getWbiKeys()
	a.cache.Store("imgKey", imgKey)
	a.cache.Store("subKey", subKey)
	a.lastUpdateTime = time.Now()
}

func (a *AISummarize) getWbiKeysCached() (string, string) {
	a.updateCache()
	imgKeyI, _ := a.cache.Load("imgKey")
	subKeyI, _ := a.cache.Load("subKey")
	return imgKeyI.(string), subKeyI.(string)
}

func (a *AISummarize) getWbiKeys() (string, string) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	if err != nil {
		fmt.Printf("Error creating request: %s", err)
		return "", ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %s", err)
		return "", ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response: %s", err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %s", err)
		return "", ""
	}
	jsonBody := string(body)
	imgURL := gjson.Get(jsonBody, "data.wbi_img.img_url").String()
	subURL := gjson.Get(jsonBody, "data.wbi_img.sub_url").String()
	imgKey := strings.Split(strings.Split(imgURL, "/")[len(strings.Split(imgURL, "/"))-1], ".")[0]
	subKey := strings.Split(strings.Split(subURL, "/")[len(strings.Split(subURL, "/"))-1], ".")[0]
	return imgKey, subKey
}

func (a *AISummarize) timestampToString(timestamp int64) string {
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
	} else {
		messageArray = append(messageArray, *cqcode.Text("该视频暂无 AI 总结"))
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
