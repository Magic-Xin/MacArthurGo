package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/ascii2d"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/plugins/googlelens"
	"MacArthurGo/plugins/soutubot"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PicSearch struct {
	groupForward      bool
	allowPrivate      bool
	handleBannedHosts bool
	sauceNAOToken     string
	ascii2dClient     *ascii2d.Client
	soutuBotClient    *soutubot.Client
	soutuBotThreshold float64
	googleLensClient  *googlelens.Client
}

type sauceNAOResponse struct {
	Results []struct {
		Header struct {
			Similarity string `json:"similarity"`
			Thumbnail  string `json:"thumbnail"`
		} `json:"header"`
		Data struct {
			MemberName string   `json:"member_name"`
			AuthorName string   `json:"author_name"`
			Title      string   `json:"title"`
			Source     string   `json:"source"`
			ExtURLs    []string `json:"ext_urls"`
		} `json:"data"`
	} `json:"results"`
}

func registerPicSearch() error {
	cfg := base.Config.Plugins.PicSearch
	asciiClient, err := ascii2d.NewClient(ascii2d.Config{
		CloudflareBypassURL: cfg.ASCII2D.CloudflareBypassURL,
		ProxyURL:            cfg.ASCII2D.ProxyURL,
		Timeout:             time.Duration(cfg.ASCII2D.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		log.Printf("Ascii2d client init error: %v", err)
	}
	soutuBotBypassURL := cfg.SoutuBot.CloudflareBypassURL
	if strings.TrimSpace(soutuBotBypassURL) == "" {
		soutuBotBypassURL = cfg.ASCII2D.CloudflareBypassURL
	}
	soutuBotProxyURL := cfg.SoutuBot.ProxyURL
	if strings.TrimSpace(soutuBotProxyURL) == "" {
		soutuBotProxyURL = cfg.ASCII2D.ProxyURL
	}
	soutuBotTimeout := cfg.SoutuBot.TimeoutSeconds
	if soutuBotTimeout == 0 {
		soutuBotTimeout = cfg.ASCII2D.TimeoutSeconds
	}
	soutuBotClient, err := soutubot.NewClient(soutubot.Config{
		CloudflareBypassURL: soutuBotBypassURL,
		ProxyURL:            soutuBotProxyURL,
		Timeout:             time.Duration(soutuBotTimeout) * time.Second,
	})
	if err != nil {
		log.Printf("SoutuBot client init error: %v", err)
	}
	var googleClient *googlelens.Client
	if strings.TrimSpace(cfg.GoogleLens.APIKey) != "" {
		googleClient, err = googlelens.NewClient(googlelens.Config{
			APIKey:  cfg.GoogleLens.APIKey,
			Timeout: time.Duration(cfg.GoogleLens.TimeoutSeconds) * time.Second,
		})
		if err != nil {
			log.Printf("Google Lens client init error: %v", err)
		}
	}
	picSearch := PicSearch{
		groupForward:      cfg.GroupForward,
		allowPrivate:      cfg.AllowPrivate,
		handleBannedHosts: cfg.HandleBannedHosts,
		sauceNAOToken:     cfg.SauceNAOToken,
		ascii2dClient:     asciiClient,
		soutuBotClient:    soutuBotClient,
		soutuBotThreshold: cfg.SoutuBot.SimilarityThreshold,
		googleLensClient:  googleClient,
	}
	plugin := &essentials.Plugin{
		Name:    "搜图",
		Enabled: cfg.Enable,
		Args:    cfg.Args,
		Handler: &picSearch,
	}
	if err := essentials.Register(plugin); err != nil {
		return err
	}
	if !cfg.Enable {
		return nil
	}

	key := []string{"uid", "res", "created"}
	value := []string{"TEXT PRIMARY KEY NOT NULL", "TEXT NOT NULL", "NUMERIC NOT NULL"}
	err = essentials.CreateDB("picSearch", key, value)

	if err != nil {
		return fmt.Errorf("create picSearch database: %w", err)
	}
	return nil
}

func (p *PicSearch) Start(ctx context.Context, _ chan<- []byte) {
	cfg := base.Config.Plugins.PicSearch
	go essentials.DeleteExpired(ctx, "picSearch", "created", cfg.ExpirationTime, cfg.IntervalTime)
}

func (p *PicSearch) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- []byte) {
	rawMsg := messageStruct.RawMessage

	if messageStruct.MessageType == "group" {
		if p.checkArgs(rawMsg, base.Config.Plugins.PicSearch.Args) {
			send <- p.picSearch(messageStruct, messageStruct.Message, send, false, true, p.checkArgs(rawMsg, []string{"purge"}))
		}
	} else if p.allowPrivate {
		if p.checkArgs(rawMsg, base.Config.Plugins.PicSearch.Args) {
			send <- p.picSearch(messageStruct, messageStruct.Message, send, false, false, p.checkArgs(rawMsg, []string{"purge"}))
		} else {
			words := essentials.SplitArgument(messageStruct.Message)
			if len(words) == 0 {
				send <- p.picSearch(messageStruct, messageStruct.Message, send, false, false, p.checkArgs(rawMsg, []string{"purge"}))
			} else if !strings.HasPrefix(words[0], "/") {
				send <- p.picSearch(messageStruct, messageStruct.Message, send, false, false, p.checkArgs(rawMsg, []string{"purge"}))
			}
		}

	}
}

func (p *PicSearch) ReceiveEcho(echoMessageStruct *structs.EchoMessageStruct, send chan<- []byte) {
	echo := echoMessageStruct.Echo
	split := strings.Split(echo, "|")
	if len(split) < 2 {
		return
	}

	if split[0] == "picSearch" && (echoMessageStruct.Data.Message != nil || echoMessageStruct.Status != "ok") {
		data := echoMessageStruct.Data
		msg := data.Message
		value, ok := essentials.GetCache(split[1])
		if !ok {
			log.Println("Pic search get cache error")
			return
		}
		originCtx := value.Value

		if echoMessageStruct.Status == "failed" {
			send <- essentials.SendMsg(&originCtx, "搜图失败", nil, false, false, "")
			return
		}

		if len(split) == 3 {
			send <- p.picSearch(&originCtx, msg, send, true, originCtx.MessageType == "group", split[2] == "purge")
		} else {
			send <- p.picSearch(&originCtx, msg, send, true, originCtx.MessageType == "group", false)
		}
	}
}

func (p *PicSearch) picSearch(messageStruct *structs.MessageStruct, msg []cqcode.ArrayMessage, send chan<- []byte,
	isEcho bool, isGroup bool, isPurge bool) []byte {
	if !isGroup && !p.allowPrivate {
		return nil
	}
	if msg == nil {
		return nil
	}

	var (
		result  [][]cqcode.ArrayMessage
		lastKey string
	)
	start := time.Now()
	for _, c := range msg {
		switch c.Type {
		case "image":
			send <- essentials.SendMsg(messageStruct, "正在搜索中，请稍等", nil, false, false, "")
			imgURL, ok := c.Data["url"].(string)
			if !ok || strings.TrimSpace(imgURL) == "" {
				result = append(result, []cqcode.ArrayMessage{*cqcode.Text("图片地址无效，无法搜图")})
				continue
			}

			key := essentials.GetImageKey(imgURL)
			lastKey = key
			cachedResult, cached := p.loadCachedResult(key)
			if cached && !isPurge {
				result = append(result, []cqcode.ArrayMessage{*cqcode.Text("本次搜图结果来自数据库缓存")})
				result = append(result, cachedResult...)
				if p.soutuBotClient != nil && !hasSoutuBotResult(cachedResult) {
					soutuBotResult := p.soutuBot(imgURL)
					result = append(result, soutuBotResult)
					if isCacheableSearchResult([][]cqcode.ArrayMessage{soutuBotResult}) {
						cachedResult = append(cachedResult, soutuBotResult)
						p.storeCachedResult(key, cachedResult, true)
					}
				}
				if p.googleLensClient != nil && !hasGoogleLensResult(cachedResult) {
					googleResult := p.googleLens(imgURL)
					result = append(result, googleResult)
					if isCacheableSearchResult([][]cqcode.ArrayMessage{googleResult}) {
						cachedResult = append(cachedResult, googleResult)
						p.storeCachedResult(key, cachedResult, true)
					}
				}
				continue
			}

			imageResult := p.searchImage(imgURL)
			result = append(result, imageResult...)
			if isCacheableSearchResult(imageResult) {
				p.storeCachedResult(key, imageResult, cached)
			}

		case "reply":
			if isEcho {
				continue
			}
			value := essentials.EchoCache{Value: *messageStruct, Time: time.Now().Unix()}
			essentials.SetCache(strconv.FormatInt(messageStruct.MessageId, 10), value)
			echo := fmt.Sprintf("picSearch|%d", messageStruct.MessageId)
			if isPurge {
				echo += "|purge"
			}
			idStr := c.Data["id"].(string)
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				log.Printf("Failed to convert id to int64: %v", err)
				continue
			}
			return essentials.SendAction("get_msg", structs.GetMsg{Id: id}, echo)
		}
	}
	if len(result) == 0 {
		return nil
	}

	result = append(result, []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("本次搜图总用时: %0.3fs", time.Since(start).Seconds()))})
	if p.groupForward {
		var data []structs.ForwardNode
		botID, botName := essentials.Info.Account()
		for _, r := range result {
			data = append(data, *essentials.ConstructForwardNode(botID, botName, r))
		}
		if isGroup {
			return essentials.SendGroupForward(messageStruct, data, *p.genEcho(messageStruct, lastKey, false))
		}
		return essentials.SendPrivateForward(messageStruct, data, *p.genEcho(messageStruct, lastKey, false))
	}

	var combined []cqcode.ArrayMessage
	for _, item := range result {
		combined = append(combined, item...)
	}
	return essentials.SendMsg(messageStruct, "", combined, false, false, "")
}

func (p *PicSearch) searchImage(imageURL string) [][]cqcode.ArrayMessage {
	response := make(chan []cqcode.ArrayMessage, 6)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		p.sauceNAO(imageURL, response)
	}()
	go func() {
		defer wg.Done()
		p.ascii2d(imageURL, response)
	}()
	go func() {
		defer wg.Done()
		response <- p.soutuBot(imageURL)
	}()
	if p.googleLensClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response <- p.googleLens(imageURL)
		}()
	}
	go func() {
		wg.Wait()
		close(response)
	}()

	var result [][]cqcode.ArrayMessage
	for item := range response {
		result = append(result, item)
	}
	return result
}

func (p *PicSearch) loadCachedResult(key string) ([][]cqcode.ArrayMessage, bool) {
	rows, err := essentials.SelectDB("picSearch", "res", "uid", key)
	if err != nil {
		log.Printf("Select picSearch cache error: %v", err)
		return nil, false
	}
	if len(rows) == 0 {
		return nil, false
	}
	encoded, ok := rows[0]["res"].(string)
	if !ok || encoded == "" {
		return nil, false
	}
	var result [][]cqcode.ArrayMessage
	if err = json.Unmarshal([]byte(encoded), &result); err != nil {
		log.Printf("Unmarshal cached message error: %v", err)
		return nil, false
	}
	result = removeLegacyGoogleSearchResults(result)
	result = removeLegacySoutuBotResults(result)
	if !isCacheableSearchResult(result) {
		return nil, false
	}
	return result, true
}

func removeLegacyGoogleSearchResults(results [][]cqcode.ArrayMessage) [][]cqcode.ArrayMessage {
	filtered := results[:0]
	for _, item := range results {
		legacy := false
		for _, segment := range item {
			if segment.Type != "text" {
				continue
			}
			text, _ := segment.Data["text"].(string)
			if strings.HasPrefix(text, "Google 搜图") {
				legacy = true
				break
			}
		}
		if !legacy {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func removeLegacySoutuBotResults(results [][]cqcode.ArrayMessage) [][]cqcode.ArrayMessage {
	filtered := results[:0]
	for _, item := range results {
		legacy := false
		for _, segment := range item {
			if segment.Type != "text" {
				continue
			}
			text, _ := segment.Data["text"].(string)
			if isLegacySoutuBotResult(text) {
				legacy = true
				break
			}
		}
		if !legacy {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func isLegacySoutuBotResult(text string) bool {
	if strings.HasPrefix(text, "SoutuBot\n") {
		return !soutubot.IsFormattedMatches(text)
	}
	englishLabels := strings.Contains(text, " | Similarity: ") &&
		strings.Contains(text, " | Language: ") &&
		strings.Contains(text, " | Source: ")
	chineseLabels := strings.Contains(text, " | 相似度: ") &&
		strings.Contains(text, " | 语言: ") &&
		strings.Contains(text, " | 来源: ")
	missingOnly := text == "未找到日文结果\n未找到中文结果" ||
		text == "未找到日文结果\n未找到中文结果\n未找到英文结果"
	return englishLabels || chineseLabels || missingOnly
}

func (p *PicSearch) storeCachedResult(key string, result [][]cqcode.ArrayMessage, exists bool) {
	encoded, err := json.Marshal(result)
	if err != nil {
		log.Printf("Marshal search result error: %v", err)
		return
	}
	now := time.Now().Unix()
	if exists {
		err = essentials.UpdateDB("picSearch", "uid", key, []string{"res", "created"}, []any{string(encoded), now})
	} else {
		err = essentials.InsertDB("picSearch", []string{"uid", "res", "created"}, []any{key, string(encoded), now})
		if err != nil {
			err = essentials.UpdateDB("picSearch", "uid", key, []string{"res", "created"}, []any{string(encoded), now})
		}
	}
	if err != nil {
		log.Printf("Store picSearch cache error: %v", err)
	}
}

func isCacheableSearchResult(result [][]cqcode.ArrayMessage) bool {
	if len(result) == 0 {
		return false
	}
	for _, item := range result {
		for _, segment := range item {
			if segment.Type != "text" {
				continue
			}
			text, _ := segment.Data["text"].(string)
			if strings.HasPrefix(text, "ascii2d：") || strings.HasPrefix(text, "SauceNAO：") || strings.HasPrefix(text, "SoutuBot：") || strings.HasPrefix(text, "Google Lens：") {
				return false
			}
		}
	}
	return true
}

func (p *PicSearch) sauceNAO(imageURL string, response chan<- []cqcode.ArrayMessage) {
	const api = "https://saucenao.com/search.php"
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	imgData, err := essentials.FetchImageData(ctx, imageURL)
	if err != nil {
		log.Printf("SauceNAO image fetch error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：下载待搜索图片失败")}
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image.jpg")
	if err != nil {
		log.Printf("SauceNAO create file field error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：创建请求失败")}
		return
	}
	if _, err = io.Copy(part, imgData); err != nil {
		log.Printf("SauceNAO write image data error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：创建请求失败")}
		return
	}
	fields := map[string]string{
		"db":          "999",
		"output_type": "2",
		"testmode":    "1",
		"numres":      "1",
		"api_key":     p.sauceNAOToken,
	}
	for name, value := range fields {
		if err = writer.WriteField(name, value); err != nil {
			log.Printf("SauceNAO write field error: %v", err)
			response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：创建请求失败")}
			return
		}
	}
	if err = writer.Close(); err != nil {
		log.Printf("SauceNAO close multipart error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：创建请求失败")}
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, body)
	if err != nil {
		log.Printf("SauceNAO request error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：创建请求失败")}
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("SauceNAO response error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：请求失败")}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("SauceNAO returned %s", resp.Status)
		response <- []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("SauceNAO：服务返回 %s", resp.Status))}
		return
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		log.Printf("SauceNAO read response error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：读取响应失败")}
		return
	}
	var payload sauceNAOResponse
	if err = json.Unmarshal(respBody, &payload); err != nil {
		log.Printf("SauceNAO decode response error: %v", err)
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：解析响应失败")}
		return
	}
	if len(payload.Results) == 0 {
		response <- []cqcode.ArrayMessage{*cqcode.Text("SauceNAO：未找到相似图片")}
		return
	}
	result := payload.Results[0]
	similarity, _ := strconv.ParseFloat(result.Header.Similarity, 64)
	author := result.Data.MemberName
	if author == "" {
		author = result.Data.AuthorName
	}
	sourceURL := result.Data.Source
	var extURL string
	if len(result.Data.ExtURLs) > 0 {
		extURL = result.Data.ExtURLs[0]
	}

	r := []cqcode.ArrayMessage{*cqcode.Text("SauceNAO\n")}
	if imageBase64 := p.ThumbnailToBase64(result.Header.Thumbnail); imageBase64 != nil {
		r = append(r, *cqcode.Image(*imageBase64))
	}

	msg := fmt.Sprintf("\n相似度: %.2f%%\n", similarity)
	if result.Data.Title != "" {
		msg += "「" + result.Data.Title + "」"
		if author != "" {
			msg += "/「" + author + "」"
		}
		msg += "\n"
	}
	if sourceURL != "" {
		if p.handleBannedHosts {
			p.HandleBannedHostsArray(&sourceURL)
		}
		msg += sourceURL + "\n"
	}
	if extURL != "" {
		if p.handleBannedHosts {
			p.HandleBannedHostsArray(&extURL)
		}
		msg += extURL
	}
	r = append(r, *cqcode.Text(msg))
	response <- r
}

func (p *PicSearch) ascii2d(imageURL string, response chan<- []cqcode.ArrayMessage) {
	if p.ascii2dClient == nil {
		response <- []cqcode.ArrayMessage{*cqcode.Text("ascii2d：客户端配置无效，请检查绕过服务地址")}
		return
	}

	results, err := p.ascii2dClient.Search(context.Background(), imageURL)
	if err != nil {
		log.Printf("Ascii2d search warning: %v", err)
	}
	if len(results) == 0 {
		message := "ascii2d：搜索失败，详细原因请查看程序日志"
		if err != nil {
			switch {
			case strings.Contains(err.Error(), "CloudflareBypassForScraping"):
				message = "ascii2d：CloudflareBypassForScraping 请求失败，请检查服务、网络或代理"
			case strings.Contains(err.Error(), "could not recover the ascii2d result URL"):
				message = "ascii2d：无法识别搜索结果页，请查看程序日志"
			}
		}
		response <- []cqcode.ArrayMessage{*cqcode.Text(message)}
		return
	}

	modeLabels := map[string]string{"color": "色合搜索", "bovw": "特征搜索"}
	for _, result := range results {
		label := modeLabels[result.Mode]
		if label == "" {
			label = result.Mode
		}
		message := []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("ascii2d %s\n", label))}
		if thumbnail, downloadErr := p.ascii2dClient.DownloadThumbnail(context.Background(), result); downloadErr == nil {
			encoded := "base64://" + base64.StdEncoding.EncodeToString(thumbnail)
			message = append(message, *cqcode.Image(encoded))
		} else if result.Thumbnail != "" {
			log.Printf("Ascii2d thumbnail download error: %v", downloadErr)
		}

		link := result.URL
		authorURL := result.AuthorURL
		if p.handleBannedHosts {
			p.HandleBannedHostsArray(&link)
			p.HandleBannedHostsArray(&authorURL)
		}
		var details []string
		if result.Info != "" || result.SourceType != "" {
			details = append(details, strings.TrimSpace(result.Info+" "+result.SourceType))
		}
		title := "「" + result.Title + "」"
		if result.Author != "" {
			title += "/「" + result.Author + "」"
		}
		details = append(details, title, link)
		if authorURL != "" {
			details = append(details, "作者: "+authorURL)
		}
		message = append(message, *cqcode.Text("\n" + strings.Join(details, "\n")))
		response <- message
	}
}

func (p *PicSearch) soutuBot(imageURL string) []cqcode.ArrayMessage {
	if p.soutuBotClient == nil {
		return []cqcode.ArrayMessage{*cqcode.Text("SoutuBot：客户端配置无效，请检查 CloudflareBypassForScraping 地址")}
	}

	imageData, err := essentials.FetchImageData(context.Background(), imageURL)
	if err != nil {
		log.Printf("SoutuBot image fetch error: %v", err)
		return []cqcode.ArrayMessage{*cqcode.Text("SoutuBot：下载待搜索图片失败")}
	}
	result, err := p.soutuBotClient.Search(context.Background(), imageData.Bytes())
	if err != nil {
		log.Printf("SoutuBot search error: %v", err)
		message := "SoutuBot：搜索失败，详细原因请查看程序日志"
		if strings.Contains(err.Error(), "CloudflareBypassForScraping") {
			message = "SoutuBot：CloudflareBypassForScraping 请求失败，请检查服务、网络或代理"
		}
		return []cqcode.ArrayMessage{*cqcode.Text(message)}
	}
	if len(result.Data) == 0 {
		return []cqcode.ArrayMessage{*cqcode.Text("SoutuBot：未找到相似结果")}
	}

	matches, maxSimilarity := soutubot.SelectBestMatches(result.Data, p.soutuBotThreshold)
	if maxSimilarity < p.soutuBotThreshold {
		return []cqcode.ArrayMessage{*cqcode.Text(fmt.Sprintf("SoutuBot：置信度过低（最高 Similarity %.2f%%，阈值 %.2f%%）", maxSimilarity, p.soutuBotThreshold))}
	}
	return []cqcode.ArrayMessage{*cqcode.Text(soutubot.FormatMatches(matches))}
}

func (p *PicSearch) googleLens(imageURL string) []cqcode.ArrayMessage {
	result, err := p.googleLensClient.Search(context.Background(), imageURL)
	if err != nil {
		log.Printf("Google Lens search error: %v", err)
		return []cqcode.ArrayMessage{*cqcode.Text("Google Lens：搜索失败，详细原因请查看程序日志")}
	}

	message := []cqcode.ArrayMessage{*cqcode.Text("Google Lens\n")}
	if thumbnail := p.ThumbnailToBase64(result.Thumbnail); thumbnail != nil {
		message = append(message, *cqcode.Image(*thumbnail))
	} else {
		message = append(message, *cqcode.Image(result.Thumbnail))
	}
	link := result.Link
	if p.handleBannedHosts {
		p.HandleBannedHostsArray(&link)
	}
	var details []string
	if result.Title != "" {
		details = append(details, "「"+result.Title+"」")
	}
	details = append(details, link)
	message = append(message, *cqcode.Text("\n" + strings.Join(details, "\n")))
	return message
}

func hasSoutuBotResult(results [][]cqcode.ArrayMessage) bool {
	for _, item := range results {
		for _, segment := range item {
			if segment.Type != "text" {
				continue
			}
			text, _ := segment.Data["text"].(string)
			if soutubot.IsFormattedMatches(text) {
				return true
			}
		}
	}
	return false
}

func hasGoogleLensResult(results [][]cqcode.ArrayMessage) bool {
	for _, item := range results {
		for _, segment := range item {
			if segment.Type != "text" {
				continue
			}
			text, _ := segment.Data["text"].(string)
			if strings.HasPrefix(text, "Google Lens\n") {
				return true
			}
		}
	}
	return false
}

func (p *PicSearch) checkArgs(rawMsg string, args []string) bool {
	for _, arg := range args {
		quoted := regexp.QuoteMeta(arg)
		if match := regexp.MustCompile(`(` + quoted + `$|` + quoted + `\W)`).FindStringIndex(rawMsg); match != nil {
			return true
		}
	}
	return false
}

func (p *PicSearch) genEcho(messageStruct *structs.MessageStruct, key string, retry bool) *string {
	var res string

	if retry {
		res = "picFailed|" + key
	} else {
		res = "picForward|" + key
	}

	if messageStruct.MessageType == "private" {
		res += "|private|" + strconv.FormatInt(messageStruct.UserId, 10)
	} else {
		res += "|group|" + strconv.FormatInt(messageStruct.GroupId, 10)
	}

	return &res
}

func (p *PicSearch) HandleBannedHostsArray(str *string) {
	bannedHosts := []string{"danbooru.donmai.us", "konachan.com"}
	*str = strings.Replace(*str, "//", "//\u200B", -1)
	for _, host := range bannedHosts {
		*str = strings.Replace(*str, host, strings.Replace(host, ".", ".\u200B", -1), -1)
	}
}

func (p *PicSearch) ThumbnailToBase64(url string) *string {
	if strings.TrimSpace(url) == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Thumbnail request error: %v", err)
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/135.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Thumbnail response error: %v", err)
		return nil
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("Thumbnail response close error: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		log.Printf("Thumbnail response status: %s", resp.Status)
		return nil
	}

	imageData, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20+1))
	if err != nil {
		log.Printf("Thumbnail read error: %v", err)
		return nil
	}
	if len(imageData) > 10<<20 {
		log.Printf("Thumbnail exceeds 10 MiB")
		return nil
	}

	imageBase64 := "base64://" + base64.StdEncoding.EncodeToString(imageData)
	return &imageBase64
}
