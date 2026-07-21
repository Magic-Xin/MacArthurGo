package test

import (
	"MacArthurGo/plugins/soutubot"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSoutuBotClientSearchUsesCloudflareBypassForScraping(t *testing.T) {
	t.Parallel()

	const (
		proxyURL  = "http://proxy.example:8080"
		userAgent = "test-browser-agent"
		globalM   = int64(24680)
	)
	imageData := []byte("image-data")
	var homepageCalls atomic.Int32
	var searchCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/html":
			homepageCalls.Add(1)
			if got := request.URL.Query().Get("url"); got != "https://soutubot.moe" {
				t.Errorf("homepage target = %q", got)
			}
			if got := request.URL.Query().Get("proxy"); got != proxyURL {
				t.Errorf("homepage proxy = %q, want %q", got, proxyURL)
			}
			writer.Header().Set("x-cf-bypasser-user-agent", userAgent)
			writer.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(writer, `<script>window.GLOBAL = {m: %d, version: 1}</script>`, globalM)
		case "/api/search":
			searchCalls.Add(1)
			if request.Method != http.MethodPost {
				t.Errorf("search method = %s", request.Method)
			}
			if got := request.Header.Get("x-hostname"); got != "soutubot.moe" {
				t.Errorf("search x-hostname = %q", got)
			}
			if got := request.Header.Get("x-proxy"); got != proxyURL {
				t.Errorf("search x-proxy = %q, want %q", got, proxyURL)
			}
			if got := request.UserAgent(); got != userAgent {
				t.Errorf("search user agent = %q, want %q", got, userAgent)
			}
			if got := request.Header.Get("Origin"); got != "https://soutubot.moe" {
				t.Errorf("search origin = %q", got)
			}
			if !strings.Contains(request.Header.Get("Content-Type"), "boundary=----WebKitFormBoundary") {
				t.Errorf("search content type = %q", request.Header.Get("Content-Type"))
			}
			if !validSoutuBotAPIKey(request.Header.Get("X-Api-Key"), len(userAgent), globalM, time.Now().Unix()) {
				t.Errorf("invalid X-Api-Key %q", request.Header.Get("X-Api-Key"))
			}

			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart form: %v", err)
				http.Error(writer, "bad multipart", http.StatusBadRequest)
				return
			}
			file, _, err := request.FormFile("file")
			if err != nil {
				t.Errorf("read image form file: %v", err)
				http.Error(writer, "missing image", http.StatusBadRequest)
				return
			}
			defer file.Close()
			gotImage, _ := io.ReadAll(file)
			if string(gotImage) != string(imageData) {
				t.Errorf("uploaded image = %q, want %q", gotImage, imageData)
			}
			if got := request.FormValue("factor"); got != "1.2" {
				t.Errorf("factor = %q, want 1.2", got)
			}

			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": "result-id",
				"data": []map[string]any{
					{"title": "Japanese lower", "similarity": 80.0, "language": "jp", "source": "nhentai", "subjectPath": "/g/100"},
					{"title": "Chinese highest", "similarity": 88.0, "language": "cn", "source": "ehentai", "subjectPath": "/g/200/token"},
					{"title": "Japanese\n highest", "similarity": 90.0, "language": "jp", "source": "panda", "subjectPath": "/g/300"},
					{"title": "English overall", "similarity": 99.0, "language": "gb", "source": "ehentai", "subjectPath": "/g/400/token"},
				},
			})
		default:
			http.Error(writer, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := soutubot.NewClient(soutubot.Config{
		CloudflareBypassURL: server.URL,
		ProxyURL:            proxyURL,
		Timeout:             time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	response, err := client.Search(context.Background(), imageData)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if response.ID != "result-id" {
		t.Fatalf("response ID = %q", response.ID)
	}
	if got := response.Data[0].SourceURL(); got != "https://nhentai.net/g/100" {
		t.Fatalf("nhentai source URL = %q", got)
	}
	if homepageCalls.Load() != 1 || searchCalls.Load() != 1 {
		t.Fatalf("calls: homepage=%d search=%d", homepageCalls.Load(), searchCalls.Load())
	}

	matches, maxSimilarity := soutubot.SelectBestMatches(response.Data, soutubot.DefaultSimilarityThreshold)
	if maxSimilarity != 99 || len(matches) != 3 {
		t.Fatalf("matches = %#v, max similarity = %.2f", matches, maxSimilarity)
	}
	if matches[0].Title != "Japanese\n highest" || matches[1].Title != "Chinese highest" || matches[2].Title != "English overall" {
		t.Fatalf("selected matches = %#v", matches)
	}
	want := "SoutuBot\n\n" +
		"标题: Japanese highest | 相似度: 90.00% | 语言: 🇯🇵 | 来源: https://panda.chaika.moe/g/300\n\n" +
		"标题: Chinese highest | 相似度: 88.00% | 语言: 🇨🇳 | 来源: https://e-hentai.org/g/200/token\n\n" +
		"标题: English overall | 相似度: 99.00% | 语言: 🇬🇧 | 来源: https://e-hentai.org/g/400/token"
	if got := soutubot.FormatMatches(matches); got != want {
		t.Fatalf("formatted matches = %q, want %q", got, want)
	}
	if !soutubot.IsFormattedMatches(want) {
		t.Fatal("formatted matches were not recognized")
	}
	if soutubot.IsFormattedMatches(strings.ReplaceAll(want, "\n\n", "\n")) {
		t.Fatal("legacy single-spaced matches were recognized as current")
	}
	missingChinese := "SoutuBot\n\n标题: Japanese highest | 相似度: 90.00% | 语言: 🇯🇵 | 来源: https://panda.chaika.moe/g/300\n\n未找到中文结果\n\n" +
		"标题: English overall | 相似度: 99.00% | 语言: 🇬🇧 | 来源: https://e-hentai.org/g/400/token"
	if got := soutubot.FormatMatches([]soutubot.Item{matches[0], matches[2]}); got != missingChinese {
		t.Fatalf("formatted match without Chinese result = %q, want %q", got, missingChinese)
	}
	if !soutubot.IsFormattedMatches(missingChinese) {
		t.Fatal("result without a Chinese match was not recognized")
	}
	missingJapanese := "SoutuBot\n\n未找到日文结果\n\n标题: Chinese highest | 相似度: 88.00% | 语言: 🇨🇳 | 来源: https://e-hentai.org/g/200/token\n\n" +
		"标题: English overall | 相似度: 99.00% | 语言: 🇬🇧 | 来源: https://e-hentai.org/g/400/token"
	if got := soutubot.FormatMatches(matches[1:]); got != missingJapanese {
		t.Fatalf("formatted match without Japanese result = %q, want %q", got, missingJapanese)
	}
	if !soutubot.IsFormattedMatches(missingJapanese) {
		t.Fatal("result without a Japanese match was not recognized")
	}
	missingEnglish := "SoutuBot\n\n标题: Japanese highest | 相似度: 90.00% | 语言: 🇯🇵 | 来源: https://panda.chaika.moe/g/300\n\n" +
		"标题: Chinese highest | 相似度: 88.00% | 语言: 🇨🇳 | 来源: https://e-hentai.org/g/200/token\n\n未找到英文结果"
	if got := soutubot.FormatMatches(matches[:2]); got != missingEnglish {
		t.Fatalf("formatted match without English result = %q, want %q", got, missingEnglish)
	}
	if !soutubot.IsFormattedMatches(missingEnglish) {
		t.Fatal("result without an English match was not recognized")
	}
	if lowMatches, maximum := soutubot.SelectBestMatches(response.Data, 99.01); len(lowMatches) != 0 || maximum != 99 {
		t.Fatalf("low-confidence selection = %#v, max = %.2f", lowMatches, maximum)
	}
}

func TestSoutuBotClientSearchRefreshesCredentialsAfterForbidden(t *testing.T) {
	t.Parallel()

	var homepageCalls atomic.Int32
	var searchCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/html":
			call := homepageCalls.Add(1)
			writer.Header().Set("x-cf-bypasser-user-agent", fmt.Sprintf("agent-%d", call))
			_, _ = fmt.Fprintf(writer, `<script>const GLOBAL = {m: %d, value: 1}</script>`, 100+call)
		case "/api/search":
			call := searchCalls.Add(1)
			if call == 1 {
				http.Error(writer, "forbidden", http.StatusForbidden)
				return
			}
			if got := request.Header.Get("x-bypass-cache"); got != "true" {
				t.Errorf("retry x-bypass-cache = %q", got)
			}
			if got := request.UserAgent(); got != "agent-2" {
				t.Errorf("retry user agent = %q", got)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"data":[]}`))
		default:
			http.Error(writer, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := soutubot.NewClient(soutubot.Config{CloudflareBypassURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err = client.Search(context.Background(), []byte("image")); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if homepageCalls.Load() != 2 || searchCalls.Load() != 2 {
		t.Fatalf("calls: homepage=%d search=%d", homepageCalls.Load(), searchCalls.Load())
	}
}

func TestSoutuBotClientRejectsInvalidConfigurationAndImage(t *testing.T) {
	t.Parallel()

	if _, err := soutubot.NewClient(soutubot.Config{CloudflareBypassURL: "://invalid"}); err == nil {
		t.Fatal("NewClient accepted an invalid bypass URL")
	}
	client, err := soutubot.NewClient(soutubot.Config{CloudflareBypassURL: "http://127.0.0.1:8000"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err = client.Search(context.Background(), nil); err == nil {
		t.Fatal("Search accepted empty image data")
	}
}

func validSoutuBotAPIKey(key string, userAgentLength int, globalM int64, now int64) bool {
	encoded := []byte(key)
	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}
	for len(encoded)%4 != 0 {
		encoded = append(encoded, '=')
	}
	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return false
	}
	value, err := strconv.ParseFloat(string(decoded), 64)
	if err != nil {
		return false
	}
	for timestamp := now - 2; timestamp <= now+2; timestamp++ {
		expected := math.Pow(float64(timestamp), 2) + math.Pow(float64(userAgentLength), 2) + float64(globalM)
		if math.Abs(value-expected) <= 4096 {
			return true
		}
	}
	return false
}
