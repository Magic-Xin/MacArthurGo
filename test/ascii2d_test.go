package test

import (
	"MacArthurGo/plugins/ascii2d"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientSearch_UsesCloudflareBypassForScraping(t *testing.T) {
	t.Parallel()

	const (
		proxyURL = "http://proxy.example:8080"
		imageURL = "https://example.com/image.png?size=large"
	)
	var (
		htmlCalls   atomic.Int32
		imageCalls  atomic.Int32
		colorTarget string
		targetMu    sync.Mutex
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/html":
			htmlCalls.Add(1)
			if got := request.URL.Query().Get("proxy"); got != proxyURL {
				t.Errorf("HTML proxy query = %q, want %q", got, proxyURL)
			}
			if got := request.Header.Get("x-proxy"); got != "" {
				t.Errorf("HTML request unexpectedly used x-proxy = %q", got)
			}
			target := request.URL.Query().Get("url")
			mode := "color"
			thumbnail := "https://cdn.ascii2d.net/thumb.jpg?size=small"
			if strings.Contains(target, "/search/bovw/") {
				mode = "bovw"
				thumbnail = "https://cdn.ascii2d.net/bovw.jpg"
			} else {
				targetMu.Lock()
				colorTarget = target
				targetMu.Unlock()
			}
			writer.Header().Set("x-cf-bypasser-final-url", "https://ascii2d.net/search/"+mode+"/result-token")
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte(ascii2dResultHTML(strings.ToUpper(mode), thumbnail)))
		case "/thumb.jpg":
			imageCalls.Add(1)
			if got := request.Header.Get("x-hostname"); got != "cdn.ascii2d.net" {
				t.Errorf("image x-hostname = %q", got)
			}
			if got := request.Header.Get("x-proxy"); got != proxyURL {
				t.Errorf("image x-proxy = %q, want %q", got, proxyURL)
			}
			if got := request.URL.Query().Get("size"); got != "small" {
				t.Errorf("image query size = %q", got)
			}
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("mirrored-image"))
		default:
			http.Error(writer, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{
		CloudflareBypassURL: server.URL,
		ProxyURL:            proxyURL,
		Timeout:             time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), imageURL)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 || results[0].Title != "COLOR" || results[1].Title != "BOVW" {
		t.Fatalf("Search() results = %#v", results)
	}
	wantTarget := "https://ascii2d.net/search/url/" + url.QueryEscape(imageURL) + "?type=color"
	targetMu.Lock()
	gotTarget := colorTarget
	targetMu.Unlock()
	if gotTarget != wantTarget {
		t.Fatalf("color search target = %q, want %q", gotTarget, wantTarget)
	}
	data, err := client.DownloadThumbnail(context.Background(), results[0])
	if err != nil {
		t.Fatalf("DownloadThumbnail: %v", err)
	}
	if string(data) != "mirrored-image" {
		t.Fatalf("thumbnail data = %q", data)
	}
	if htmlCalls.Load() != 2 || imageCalls.Load() != 1 {
		t.Fatalf("calls: html=%d image=%d", htmlCalls.Load(), imageCalls.Load())
	}
}

func TestClientSearch_ReportsChromiumNetworkErrorWithoutLeakingImageURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("x-cf-bypasser-final-url", request.URL.Query().Get("url"))
		_, _ = writer.Write([]byte(`<html><script>window.loadTimeDataRaw = {"errorCode":"ERR_TIMED_OUT"};</script></html>`))
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Search(context.Background(), "https://example.com/image.png?rkey=secret")
	if err == nil || !strings.Contains(err.Error(), "ERR_TIMED_OUT") {
		t.Fatalf("Search error = %v, want ERR_TIMED_OUT", err)
	}
	if strings.Contains(err.Error(), "rkey") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("Search error leaked the source image URL: %v", err)
	}
}

func TestClientSearch_UsesRenderedHTMLWhenFinalURLIsStale(t *testing.T) {
	t.Parallel()

	const (
		staleURL = "https://ascii2d.net/search/url/encoded-image?type=color"
		colorURL = "https://ascii2d.net/search/color/result-token"
		bovwURL  = "https://ascii2d.net/search/bovw/result-token"
	)
	var (
		requestedBovwURL string
		requestedMu      sync.Mutex
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		target := request.URL.Query().Get("url")
		switch {
		case strings.Contains(target, "/search/url/"):
			writer.Header().Set("x-cf-bypasser-final-url", staleURL)
			_, _ = writer.Write([]byte(`<html><head><link rel="canonical" href="` + colorURL + `"></head><body>` + ascii2dResultHTML("Color title", "/color.jpg") + `</body></html>`))
		case target == bovwURL:
			requestedMu.Lock()
			requestedBovwURL = target
			requestedMu.Unlock()
			writer.Header().Set("x-cf-bypasser-final-url", bovwURL)
			_, _ = writer.Write([]byte(ascii2dResultHTML("Bovw title", "/bovw.jpg")))
		default:
			http.Error(writer, "unexpected target", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].ResultURL != colorURL {
		t.Fatalf("color result URL = %q, want %q", results[0].ResultURL, colorURL)
	}
	requestedMu.Lock()
	gotBovwURL := requestedBovwURL
	requestedMu.Unlock()
	if gotBovwURL != bovwURL {
		t.Fatalf("requested bovw URL = %q, want %q", gotBovwURL, bovwURL)
	}
}

func TestClientSearch_RejectsResultPageOnAnotherHost(t *testing.T) {
	t.Parallel()

	var (
		requestedTargets []string
		requestedMu      sync.Mutex
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		target := request.URL.Query().Get("url")
		requestedMu.Lock()
		requestedTargets = append(requestedTargets, target)
		requestedMu.Unlock()
		writer.Header().Set("x-cf-bypasser-final-url", "https://ascii2d.net/search/url/image?type=color")
		_, _ = writer.Write([]byte(`<link rel="canonical" href="https://example.com/search/color/not-ascii2d">` + ascii2dResultHTML("Color title", "/color.jpg")))
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, searchErr := client.Search(context.Background(), "https://example.com/image.png")
	if len(results) != 1 || searchErr == nil {
		t.Fatalf("Search() results = %#v, error = %v; want one color result and a bovw URL error", results, searchErr)
	}
	requestedMu.Lock()
	targets := append([]string(nil), requestedTargets...)
	requestedMu.Unlock()
	for _, target := range targets {
		if strings.Contains(target, "example.com/search/") {
			t.Fatalf("Search requested an untrusted result URL: %s", target)
		}
	}
}

func TestClientSearch_RejectsInvalidImageURL(t *testing.T) {
	t.Parallel()

	client, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: "http://127.0.0.1:8000"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err = client.Search(context.Background(), "file:///tmp/image.png"); err == nil {
		t.Fatal("Search accepted a non-HTTP image URL")
	}
}

func TestClientSearch_IgnoresIncompleteResultBoxes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		target := request.URL.Query().Get("url")
		mode := "color"
		if strings.Contains(target, "/bovw/") {
			mode = "bovw"
		}
		writer.Header().Set("x-cf-bypasser-final-url", "https://ascii2d.net/search/"+mode+"/id")
		_, _ = writer.Write([]byte(`<div class="row item-box"><div class="detail-box">no links</div></div>` + ascii2dResultHTML("Title", "//cdn.ascii2d.net/thumb.jpg")))
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png")
	if err != nil || len(results) != 2 {
		t.Fatalf("Search() results = %#v, error = %v", results, err)
	}
	if results[0].Title != "Title" || results[0].Author != "Author" || results[0].SourceType != "pixiv" {
		t.Fatalf("unexpected result: %#v", results[0])
	}
	if results[0].Thumbnail != "https://cdn.ascii2d.net/thumb.jpg" {
		t.Fatalf("unexpected thumbnail: %s", results[0].Thumbnail)
	}
}

func TestClientSearch_CloudflareBypassRetriesFirstByteTimeout(t *testing.T) {
	t.Parallel()

	var colorCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		target := request.URL.Query().Get("url")
		mode := "bovw"
		if strings.Contains(target, "/search/url/") {
			mode = "color"
			if colorCalls.Add(1) == 1 {
				http.Error(writer, "first byte timeout", http.StatusGatewayTimeout)
				return
			}
		}
		writer.Header().Set("x-cf-bypasser-final-url", "https://ascii2d.net/search/"+mode+"/token")
		_, _ = writer.Write([]byte(ascii2dResultHTML(mode, "/"+mode+".jpg")))
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png")
	if err != nil || len(results) != 2 {
		t.Fatalf("Search() results = %#v, error = %v", results, err)
	}
	if colorCalls.Load() != 2 {
		t.Fatalf("color calls = %d, want 2", colorCalls.Load())
	}
}

func TestClientSearch_SerializesCloudflareBypassRequests(t *testing.T) {
	t.Parallel()

	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			maximum := maxInFlight.Load()
			if current <= maximum || maxInFlight.CompareAndSwap(maximum, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		target := request.URL.Query().Get("url")
		mode := "color"
		if strings.Contains(target, "/search/bovw/") {
			mode = "bovw"
		}
		writer.Header().Set("x-cf-bypasser-final-url", "https://ascii2d.net/search/"+mode+"/token")
		_, _ = writer.Write([]byte(ascii2dResultHTML(mode, "/"+mode+".jpg")))
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var wait sync.WaitGroup
	errors := make(chan error, 4)
	for index := 0; index < 4; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, searchErr := client.Search(context.Background(), "https://example.com/image.png")
			errors <- searchErr
		}()
	}
	wait.Wait()
	close(errors)
	for searchErr := range errors {
		if searchErr != nil {
			t.Fatalf("Search() error = %v", searchErr)
		}
	}
	if maxInFlight.Load() != 1 {
		t.Fatalf("maximum concurrent bypass requests = %d, want 1", maxInFlight.Load())
	}
}

func TestNewClient_RejectsInvalidCloudflareBypassURL(t *testing.T) {
	t.Parallel()

	for _, bypassURL := range []string{"", "://invalid"} {
		if _, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: bypassURL}); err == nil {
			t.Errorf("NewClient accepted invalid CloudflareBypassForScraping URL %q", bypassURL)
		}
	}
}

func ascii2dResultHTML(title, thumbnail string) string {
	return fmt.Sprintf(`
<div class="row item-box">
  <div class="image-box"><img src="%s"></div>
  <div class="detail-box">
    <small>1200x800 JPEG</small>
    <h6><a href="https://example.com/work"> %s </a><a href="https://example.com/author">Author</a><small>pixiv</small></h6>
  </div>
</div>`, thumbnail, title)
}
