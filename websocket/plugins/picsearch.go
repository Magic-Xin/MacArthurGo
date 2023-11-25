package plugins

import (
	_struct "MacArthurGo/struct"
	"MacArthurGo/struct/cqcode"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/floatbox/web"
	xpath "github.com/antchfx/htmlquery"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Result struct {
	Info   string
	Link   string
	Name   string
	Author string
	AuthNm string
	Thumb  string
	Type   string
}

func PicSearch(msg string, isEcho bool) (s *[]string, b *[]byte) {
	if !(isEcho || strings.Contains(msg, "/search")) {
		return
	}

	var res []string
	cc := cqcode.FromStr(msg)
	for _, c := range *cc {
		if c.Type == "image" {
			fileUrl := c.Data["url"].(string)
			fileUrl = getUniversalImgURL(fileUrl)
			r, err := ascii2d(fileUrl)
			if err != nil {
				res = append(res, fmt.Sprintf("%v", err))
			} else {
				res = append(res, fmt.Sprintf("ascii2d 色合検索\n%s\n%s %s\n「%s」/「%s」\n%s\nArthor:%s",
					cqcode.Image(r[0].Thumb), r[0].Info, r[0].Type, r[0].Name, r[0].AuthNm, r[0].Link, r[0].Author))
				res = append(res, fmt.Sprintf("ascii2d 特徴検索\n%s\n%s %s\n「%s」/「%s」\n%s\nArthor:%s",
					cqcode.Image(r[1].Thumb), r[1].Info, r[1].Type, r[1].Name, r[1].AuthNm, r[1].Link, r[1].Author))
			}
		}
		if c.Type == "reply" {
			cqMid := c.Data["id"].(string)
			mid, err := strconv.Atoi(cqMid)
			if err != nil {
				continue
			}
			params := _struct.GetMsg{Id: mid}
			act := _struct.EchoAction{Action: "get_msg", Params: params, Echo: "picSearch"}
			jsonMsg, _ := json.Marshal(act)
			b = &jsonMsg
		}
	}
	return &res, b
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

func ascii2d(img string) (r []*Result, err error) {
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
		return nil, err
	}

	urlB := strings.ReplaceAll(respC.Request.URL.String(), "color", "bovw")
	reqB, _ := http.NewRequest("GET", urlB, nil)
	reqB.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:6.0) Gecko/20100101 Firefox/6.0")
	respB, err := client.Do(reqB)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := respB.Body.Close()
		if err != nil {
			log.Printf("Picsearch respawn close error: %v", err)
			return
		}
		err = respC.Body.Close()
		if err != nil {
			log.Printf("Picsearch respawn close error: %v", err)
			return
		}
	}()

	r = make([]*Result, 0, 2)

	for _, resp := range []*http.Response{respC, respB} {
		doc, err := xpath.Parse(resp.Body)
		if err != nil {
			return nil, err
		}
		// 取出每个返回的结果
		list := xpath.Find(doc, `//div[@class="row item-box"]`)
		if len(list) == 0 {
			return nil, errors.New("ascii2d not found")
		}
		for _, n := range list {
			linkPath := xpath.FindOne(n, `//div[2]/div[3]/h6/a[1]`)
			authPath := xpath.FindOne(n, `//div[2]/div[3]/h6/a[2]`)
			picPath := xpath.FindOne(n, `//div[1]/img`)
			typePath := xpath.FindOne(n, `//div[2]/div[3]/h6/small`)
			if linkPath != nil && authPath != nil && picPath != nil && typePath != nil {
				r = append(r, &Result{
					Info:   xpath.InnerText(xpath.FindOne(list[0], `//div[2]/small`)),
					Link:   xpath.SelectAttr(linkPath, "href"),
					Name:   xpath.InnerText(linkPath),
					Author: xpath.SelectAttr(authPath, "href"),
					AuthNm: xpath.InnerText(authPath),
					Thumb:  "https://ascii2d.net" + xpath.SelectAttr(picPath, "src"),
					Type:   strings.Trim(xpath.InnerText(typePath), "\n"),
				})
				break
			}
		}
	}
	return
}
