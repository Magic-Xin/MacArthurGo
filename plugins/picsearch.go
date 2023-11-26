package plugins

import (
	_struct "MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/floatbox/web"
	xpath "github.com/antchfx/htmlquery"
	"github.com/gookit/config/v2"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func PicSearch(ctx *map[string]any, send *chan []byte) {
	if !config.Bool("plugins.picSearch.enable") {
		return
	}
	var (
		msg     string
		isEcho  bool
		isGroup bool
		isStart bool
	)
	if (*ctx)["echo"] != nil {
		if (*ctx)["data"].(map[string]any)["message"] != nil && (*ctx)["echo"].(string) == "picSearch" {
			isEcho = true
			*ctx = (*ctx)["data"].(map[string]any)
			msg = (*ctx)["message"].(string)
			isGroup = (*ctx)["group"].(bool)
		}
	} else {
		msg = (*ctx)["raw_message"].(string)
		if (*ctx)["message_type"].(string) == "group" {
			isGroup = true
		}
	}

	if !isGroup && !config.Bool("plugins.picSearch.allowPrivate") {
		return
	}
	if !isEcho && !strings.Contains(msg, config.String("plugins.picSearch.args")) {
		return
	}

	var result []string
	cc := cqcode.FromStr(msg)
	start := time.Now()
	for _, c := range *cc {
		if c.Type == "image" {
			if !isStart {
				*send <- *SendMsg(ctx, config.String("plugins.picSearch.searchFeedback"), false, false)
				isStart = true
			}
			fileUrl := c.Data["url"].(string)
			fileUrl = getUniversalImgURL(fileUrl)

			wg := &sync.WaitGroup{}
			wgResponse := &sync.WaitGroup{}
			limiter := make(chan bool, 10)
			response := make(chan string, 200)

			go func() {
				wgResponse.Add(1)
				for rc := range response {
					result = append(result, rc)
				}
				wgResponse.Done()
			}()

			for i := 0; i < 2; i++ {
				wg.Add(1)
				limiter <- true
				switch i {
				case 0:
					go sauceNAO(fileUrl, response, limiter, wg)
				case 1:
					go ascii2d(fileUrl, response, limiter, wg)
				}
			}
			wg.Wait()
			close(response)
			wgResponse.Wait()
		}
		if c.Type == "reply" {
			cqMid := c.Data["id"].(string)
			mid, err := strconv.Atoi(cqMid)
			if err != nil {
				continue
			}
			jsonMsg, _ := json.Marshal(_struct.EchoAction{Action: _struct.Action{
				Action: "get_msg",
				Params: _struct.GetMsg{
					Id: mid,
				},
			}, Echo: "picSearch"})
			*send <- jsonMsg
			return
		}
	}
	end := time.Since(start)
	if result != nil {
		result = append(result, fmt.Sprintf("本次搜图总用时: %0.3fs", end.Seconds()))
		if isGroup && config.Bool("plugins.picSearch.groupForward") {
			var data []_struct.ForwardNode
			for _, r := range result {
				data = append(data, *ConstructForwardNode(&r, info.NickName, info.UserId))
			}
			msg := *SendGroupForward(ctx, &data)
			log.Println(string(msg))
			*send <- msg
		} else {
			for _, r := range result {
				*send <- *SendMsg(ctx, r, false, false)
			}
		}
	}
}

func getUniversalImgURL(url string) string {
	pattern := regexp.MustCompile(`^https?://(c2cpicdw|gchat)\.qpic\.cn/(offpic|gchatpic)_new/`)
	if pattern.MatchString(url) {
		url = strings.Replace(url, "/c2cpicdw.qpic.cn/offpic_new/", "/gchat.qpic.cn/gchatpic_new/", 1)
		url = regexp.MustCompile(`/\d+/+\d+-\d+-`).ReplaceAllString(url, "/0/0-0-")
		url = strings.TrimSuffix(url, "?.*$")
	}
	return url
}

func sauceNAO(img string, response chan string, limiter chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	const api = "https://saucenao.com/search.php?db=999&output_type=2&testmode=1&numres=1"
	token := config.String("plugins.picSearch.sauceNAOToken")

	reqUrl := api + "&api_key=" + token + "&url=" + img
	client := web.NewTLS12Client()
	req, _ := http.NewRequest("GET", reqUrl, nil)
	resp, _ := client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("SauceNAO response close error: %v", err)
		}
	}(resp.Body)

	var i any
	err := json.Unmarshal(body, &i)
	if err != nil {
		response <- fmt.Sprintf("%v", err)
		return
	}

	ctx := i.(map[string]any)
	var (
		similarity float64
		thumbNail  string
		author     string
		title      string
		sourceUrl  string
		extUrl     string
	)
	if ctx["results"] != nil {
		results := ctx["results"].([]any)[0]
		header := results.(map[string]any)["header"].(map[string]any)
		data := results.(map[string]any)["data"].(map[string]any)
		similarity, _ = strconv.ParseFloat(header["similarity"].(string), 64)
		thumbNail = header["thumbnail"].(string)
		if data["member_name"] != nil {
			author = data["member_name"].(string)
		} else if data["author_name"] != nil {
			author = data["author_name"].(string)
		}
		if data["title"] != nil {
			title = data["title"].(string)
		}
		if data["source"] != nil {
			sourceUrl = data["source"].(string)
		}
		if data["ext_urls"] != nil {
			extUrl = data["ext_urls"].([]any)[0].(string)
		}
	}

	r := fmt.Sprintf("SauceNAO\n%s\n相似度: %.2f\n", cqcode.Image(thumbNail), similarity)
	if title != "" {
		r += "「" + author + "」"
		if author != "" {
			r += "/「" + author + "」"
		}
		r += "\n"
	}
	if sourceUrl != "" {
		r += sourceUrl + "\n"
	}
	if extUrl != "" {
		r += extUrl
	}
	response <- r
	<-limiter
}

func ascii2d(img string, response chan string, limiter chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	const api = "https://ascii2d.net/search/uri"
	client := web.NewTLS12Client()
	data := url.Values{}
	data.Set("uri", img) // 图片链接
	fromData := strings.NewReader(data.Encode())

	reqC, _ := http.NewRequest("POST", api, fromData)
	reqC.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqC.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0")
	respC, err := client.Do(reqC)
	if err != nil {
		response <- fmt.Sprintf("%v", err)
		return
	}

	urlB := strings.ReplaceAll(respC.Request.URL.String(), "color", "bovw")
	reqB, _ := http.NewRequest("GET", urlB, nil)
	reqB.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0")
	respB, err := client.Do(reqB)
	if err != nil {
		response <- fmt.Sprintf("%v", err)
		return
	}

	defer func() {
		err := respB.Body.Close()
		if err != nil {
			log.Printf("Picsearch response close error: %v", err)
			return
		}
		err = respC.Body.Close()
		if err != nil {
			log.Printf("Picsearch response close error: %v", err)
			return
		}
	}()

	checkType := []string{"色合検索", "特徴検索"}

	for i, resp := range []*http.Response{respC, respB} {
		doc, err := xpath.Parse(resp.Body)
		if err != nil {
			return
		}
		// 取出每个返回的结果
		list := xpath.Find(doc, `//div[@class="row item-box"]`)
		if len(list) == 0 {
			err := errors.New("ascii2d not found")
			response <- fmt.Sprintf("%v", err)
			return
		}
		for _, n := range list {
			linkPath := xpath.FindOne(n, `//div[2]/div[3]/h6/a[1]`)
			authPath := xpath.FindOne(n, `//div[2]/div[3]/h6/a[2]`)
			picPath := xpath.FindOne(n, `//div[1]/img`)
			typePath := xpath.FindOne(n, `//div[2]/div[3]/h6/small`)

			if linkPath != nil && authPath != nil && picPath != nil && typePath != nil {
				Info := xpath.InnerText(xpath.FindOne(list[0], `//div[2]/small`))
				Link := xpath.SelectAttr(linkPath, "href")
				Name := xpath.InnerText(linkPath)
				Author := xpath.SelectAttr(authPath, "href")
				AuthNm := xpath.InnerText(authPath)
				Thumb := "https://ascii2d.net" + xpath.SelectAttr(picPath, "src")
				Type := strings.Trim(xpath.InnerText(typePath), "\n")

				response <- fmt.Sprintf("ascii2d %s\n%s\n%s %s\n「%s」/「%s」\n%s\nArthor:%s",
					checkType[i], cqcode.Image(Thumb), Info, Type, Name, AuthNm, Link, Author)
				break
			}
		}
	}
	<-limiter
}
