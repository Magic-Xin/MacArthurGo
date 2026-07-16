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
