package test

import (
	"MacArthurGo/plugins/ascii2d"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type flareTestRequest struct {
	Command       string `json:"cmd"`
	URL           string `json:"url,omitempty"`
	WaitInSeconds int    `json:"waitInSeconds,omitempty"`
	DisableMedia  bool   `json:"disableMedia,omitempty"`
}

type flareTestResponse struct {
	Status   string            `json:"status"`
	Solution flareTestSolution `json:"solution"`
}

type flareTestSolution struct {
	URL       string            `json:"url"`
	Status    int               `json:"status"`
	Response  string            `json:"response"`
	Cookies   []flareTestCookie `json:"cookies"`
	UserAgent string            `json:"userAgent"`
}

type flareTestCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func TestClientSearch(t *testing.T) {
	t.Parallel()

	var (
		commands          []string
		searchURL         string
		searchWaitSeconds int
		mediaDisabled     []bool
		mu                sync.Mutex
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload flareTestRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		mu.Lock()
		commands = append(commands, payload.Command)
		mu.Unlock()

		response := flareTestResponse{Status: "ok"}
		switch {
		case payload.Command == "request.get" && strings.Contains(payload.URL, "/search/url/"):
			mu.Lock()
			searchURL = payload.URL
			searchWaitSeconds = payload.WaitInSeconds
			mediaDisabled = append(mediaDisabled, payload.DisableMedia)
			mu.Unlock()
			response.Solution = flareTestSolution{
				URL:       "https://ascii2d.net/search/color/token",
				Status:    http.StatusOK,
				Response:  ascii2dResultHTML("Color title", "/color.jpg"),
				UserAgent: "test-agent",
				Cookies:   []flareTestCookie{{Name: "cf_clearance", Value: "token"}},
			}
		case payload.Command == "request.get" && strings.Contains(payload.URL, "/bovw/"):
			mu.Lock()
			mediaDisabled = append(mediaDisabled, payload.DisableMedia)
			mu.Unlock()
			response.Solution = flareTestSolution{
				URL:       "https://ascii2d.net/search/bovw/token",
				Status:    http.StatusOK,
				Response:  ascii2dResultHTML("Bovw title", "/bovw.jpg"),
				UserAgent: "test-agent",
				Cookies:   []flareTestCookie{{Name: "cf_clearance", Value: "token"}},
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{APIURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png?size=large")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Mode != "color" || results[0].Title != "Color title" {
		t.Fatalf("unexpected color result: %#v", results[0])
	}
	if results[1].Mode != "bovw" || results[1].Title != "Bovw title" {
		t.Fatalf("unexpected bovw result: %#v", results[1])
	}
	if results[0].Thumbnail != "https://ascii2d.net/color.jpg" {
		t.Fatalf("unexpected thumbnail URL: %s", results[0].Thumbnail)
	}

	wantCommands := []string{"sessions.create", "request.get", "request.get", "sessions.destroy"}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(searchURL, "https%3A%2F%2Fexample.com%2Fimage.png%3Fsize%3Dlarge") {
		t.Fatalf("image URL was not safely encoded: %s", searchURL)
	}
	if !strings.HasSuffix(searchURL, "?type=color") {
		t.Fatalf("color search type is missing: %s", searchURL)
	}
	if searchWaitSeconds != 5 {
		t.Fatalf("color search wait = %d, want 5", searchWaitSeconds)
	}
	if !reflect.DeepEqual(mediaDisabled, []bool{true, true}) {
		t.Fatalf("request.get disableMedia values = %v, want [true true]", mediaDisabled)
	}
	if !reflect.DeepEqual(commands, wantCommands) {
		t.Fatalf("commands = %v, want %v", commands, wantCommands)
	}
}

func TestClientSearch_ReportsChromiumNetworkErrorWithoutLeakingImageURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload flareTestRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}

		response := flareTestResponse{Status: "ok"}
		if payload.Command == "request.get" {
			response.Solution = flareTestSolution{
				URL:    payload.URL,
				Status: http.StatusOK,
				Response: `<html><script>window.loadTimeDataRaw = {` +
					`"errorCode":"ERR_TIMED_OUT"};</script></html>`,
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{APIURL: server.URL, Timeout: time.Second})
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

func TestClientSearch_UsesPostWaitHTMLWhenFlareURLIsStale(t *testing.T) {
	t.Parallel()

	const (
		staleURL = "https://ascii2d.net/search/url/encoded-image?type=color"
		colorURL = "https://ascii2d.net/search/color/result-token"
		bovwURL  = "https://ascii2d.net/search/bovw/result-token"
	)
	var requestedBovwURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload flareTestRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}

		response := flareTestResponse{Status: "ok"}
		switch {
		case payload.Command == "request.get" && strings.Contains(payload.URL, "/search/url/"):
			response.Solution = flareTestSolution{
				URL:      staleURL,
				Status:   http.StatusOK,
				Response: `<html><head><link rel="canonical" href="` + colorURL + `"></head><body>` + ascii2dResultHTML("Color title", "/color.jpg") + `</body></html>`,
			}
		case payload.Command == "request.get" && payload.URL == bovwURL:
			requestedBovwURL = payload.URL
			response.Solution = flareTestSolution{
				URL:      bovwURL,
				Status:   http.StatusOK,
				Response: ascii2dResultHTML("Bovw title", "/bovw.jpg"),
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{APIURL: server.URL, Timeout: time.Second})
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
	if requestedBovwURL != bovwURL {
		t.Fatalf("requested bovw URL = %q, want %q", requestedBovwURL, bovwURL)
	}
}

func TestClientSearch_RejectsResultPageOnAnotherHost(t *testing.T) {
	t.Parallel()

	var requestedTargets []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload flareTestRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		if payload.Command == "request.get" {
			requestedTargets = append(requestedTargets, payload.URL)
		}

		response := flareTestResponse{Status: "ok"}
		if payload.Command == "request.get" {
			response.Solution = flareTestSolution{
				URL:      "https://ascii2d.net/search/url/image?type=color",
				Status:   http.StatusOK,
				Response: `<link rel="canonical" href="https://example.com/search/color/not-ascii2d">` + ascii2dResultHTML("Color title", "/color.jpg"),
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{APIURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, searchErr := client.Search(context.Background(), "https://example.com/image.png")
	if len(results) != 1 || searchErr == nil {
		t.Fatalf("Search() results = %#v, error = %v; want one color result and a bovw URL error", results, searchErr)
	}
	for _, target := range requestedTargets {
		if strings.Contains(target, "example.com/search/") {
			t.Fatalf("Search requested an untrusted result URL: %s", target)
		}
	}
}

func TestClientSearchRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	client, err := ascii2d.NewClient(ascii2d.Config{APIURL: "http://127.0.0.1:8191/v1"})
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
		var payload flareTestRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		response := flareTestResponse{Status: "ok"}
		if payload.Command == "request.get" {
			mode := "color"
			if strings.Contains(payload.URL, "/bovw/") {
				mode = "bovw"
			}
			response.Solution = flareTestSolution{
				URL:      "https://ascii2d.net/search/" + mode + "/id",
				Status:   http.StatusOK,
				Response: `<div class="row item-box"><div class="detail-box">no links</div></div>` + ascii2dResultHTML("Title", "//cdn.ascii2d.net/thumb.jpg"),
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer server.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{APIURL: server.URL, Timeout: time.Second})
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

func TestDownloadThumbnailUsesClearanceCredentials(t *testing.T) {
	t.Parallel()

	thumbnailServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie("cf_clearance")
		if err != nil || cookie.Value != "token" {
			http.Error(writer, "missing cookie", http.StatusForbidden)
			return
		}
		if request.UserAgent() != "test-agent" {
			http.Error(writer, "wrong user agent", http.StatusForbidden)
			return
		}
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("image-data"))
	}))
	defer thumbnailServer.Close()

	flareServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload flareTestRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		response := flareTestResponse{Status: "ok"}
		if payload.Command == "request.get" {
			mode := "color"
			if strings.Contains(payload.URL, "/bovw/") {
				mode = "bovw"
			}
			response.Solution = flareTestSolution{
				URL:       "https://ascii2d.net/search/" + mode + "/id",
				Status:    http.StatusOK,
				Response:  ascii2dResultHTML("Title", thumbnailServer.URL+"/thumb.jpg"),
				UserAgent: "test-agent",
				Cookies:   []flareTestCookie{{Name: "cf_clearance", Value: "token"}},
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer flareServer.Close()

	client, err := ascii2d.NewClient(ascii2d.Config{APIURL: flareServer.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png")
	if err != nil || len(results) == 0 {
		t.Fatalf("Search() results = %#v, error = %v", results, err)
	}
	data, err := client.DownloadThumbnail(context.Background(), results[0])
	if err != nil {
		t.Fatalf("DownloadThumbnail: %v", err)
	}
	if string(data) != "image-data" {
		t.Fatalf("data = %q", data)
	}
}

func TestClientSearch_UsesCloudflareBypassForScraping(t *testing.T) {
	t.Parallel()

	const proxyURL = "http://proxy.example:8080"
	var htmlCalls atomic.Int32
	var imageCalls atomic.Int32
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
			}
			writer.Header().Set("x-cf-bypasser-final-url", "https://ascii2d.net/search/"+mode+"/result-token")
			writer.Header().Set("x-cf-bypasser-user-agent", "bypass-agent")
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
		APIURL:              "://unused-invalid-flaresolverr",
		CloudflareBypassURL: server.URL,
		ProxyURL:            proxyURL,
		Timeout:             time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	results, err := client.Search(context.Background(), "https://example.com/image.png")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 || results[0].Title != "COLOR" || results[1].Title != "BOVW" {
		t.Fatalf("Search() results = %#v", results)
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

func TestNewClientRejectsInvalidCloudflareBypassURL(t *testing.T) {
	t.Parallel()

	if _, err := ascii2d.NewClient(ascii2d.Config{CloudflareBypassURL: "://invalid"}); err == nil {
		t.Fatal("NewClient accepted an invalid CloudflareBypassForScraping URL")
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
