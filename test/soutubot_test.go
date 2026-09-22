package test

import (
	"MacArthurGo/plugins/soutubot"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	)
	imageData := []byte("\x89PNG\r\n\x1a\nimage-data")
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
			_, _ = writer.Write([]byte(`<html>current site has no GLOBAL.m</html>`))
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
			if key := request.Header.Get("X-Api-Key"); key != "" {
				t.Errorf("obsolete X-Api-Key = %q", key)
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
			if got := request.MultipartForm.File["file"][0].Filename; got != "image.png" {
				t.Errorf("image filename = %q, want image.png", got)
			}
			if got := request.MultipartForm.File["file"][0].Header.Get("Content-Type"); got != "image/png" {
				t.Errorf("image content type = %q, want image/png", got)
			}
			gotImage, _ := io.ReadAll(file)
			if string(gotImage) != string(imageData) {
				t.Errorf("uploaded image = %q, want %q", gotImage, imageData)
			}
			if got := request.FormValue("factor"); got != "1.2" {
				t.Errorf("factor = %q, want 1.2", got)
			}
			if got := request.FormValue("metadata_mode"); got != "display" {
				t.Errorf("metadata_mode = %q, want display", got)
			}

			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"result_id": "result-id",
				"results": []map[string]any{
					{"score": 80.0, "path_segments": []map[string]any{{"language": "ja", "source_key": "nhentai", "source_url": "https://nhentai.net/g/100", "metadata": map[string]any{"title": map[string]any{"primary": "Japanese lower"}}}}},
					{"score": 88.0, "path_segments": []map[string]any{{"language": "zh", "source_key": "ehentai", "source_url": "https://e-hentai.org/g/200/token", "metadata": map[string]any{"title": map[string]any{"primary": "Chinese highest"}}}}},
					{"score": 90.0, "path_segments": []map[string]any{{"language": "ja", "source_key": "panda", "source_url": "https://panda.chaika.moe/g/300", "metadata": map[string]any{"title": map[string]any{"primary": "Japanese\n highest"}}}}},
					{"score": 99.0, "path_segments": []map[string]any{{"language": "en", "source_key": "ehentai", "source_url": "https://e-hentai.org/g/400/token", "metadata": map[string]any{"title": map[string]any{"primary": "English overall"}}}}},
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
			_, _ = writer.Write([]byte(`<html>no global m</html>`))
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
			_, _ = writer.Write([]byte(`{"results":[]}`))
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
