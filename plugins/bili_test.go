package plugins

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBiliLiveData(t *testing.T) {
	tests := []struct {
		name       string
		roomReply  string
		masterCode int
		wantRoom   string
		wantURL    string
		wantStatus string
		wantName   string
		wantImage  bool
	}{
		{
			name:       "live short room",
			roomReply:  `{"code":0,"data":{"uid":1472906636,"room_id":22727121,"short_id":6655,"title":"直播标题","keyframe":"https://example.com/cover.jpg","area_name":"游戏","parent_area_name":"娱乐","live_status":1,"online":12601}}`,
			wantRoom:   "短号: 6655",
			wantURL:    "https://live.bilibili.com/6655",
			wantStatus: "直播中\t1.3万人气",
			wantName:   "ywwuyi",
			wantImage:  true,
		},
		{
			name:       "offline room with unavailable master",
			roomReply:  `{"code":0,"data":{"uid":1472906636,"room_id":22727121,"title":"直播标题","area_name":"游戏","parent_area_name":"游戏","live_status":0}}`,
			masterCode: http.StatusServiceUnavailable,
			wantRoom:   "房间号: 22727121",
			wantURL:    "https://live.bilibili.com/22727121",
			wantStatus: "未开播",
		},
		{
			name:      "room API rejects request",
			roomReply: `{"code":-352,"message":"-352"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") != "MacArthurGo/1.0" {
					t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
				}
				switch r.URL.Path {
				case "/room/v1/Room/get_info":
					if got := r.URL.Query().Get("room_id"); got != "6655" {
						t.Errorf("room_id = %q", got)
					}
					fmt.Fprint(w, tt.roomReply)
				case "/live_user/v1/Master/info":
					if got := r.URL.Query().Get("uid"); got != "1472906636" {
						t.Errorf("uid = %q", got)
					}
					if tt.masterCode != 0 {
						w.WriteHeader(tt.masterCode)
						return
					}
					fmt.Fprint(w, `{"code":0,"data":{"info":{"uname":"ywwuyi"}}}`)
				default:
					t.Errorf("unexpected path %q", r.URL.Path)
				}
			}))
			defer server.Close()

			got := (&Bili{}).getLiveDataFromAPI("6655", server.URL)
			if tt.wantRoom == "" {
				if got != nil {
					t.Fatalf("getLiveDataFromAPI() = %#v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("getLiveDataFromAPI() = nil")
			}
			if got.RoomId != tt.wantRoom || got.Url != tt.wantURL || got.Status != tt.wantStatus || got.User != tt.wantName {
				t.Errorf("live data = %#v", got)
			}
			messages := got.ToArrayMessage()
			imageCount := 0
			for _, message := range messages {
				if message.Type == "image" {
					imageCount++
				}
			}
			if (imageCount == 1) != tt.wantImage {
				t.Errorf("image count = %d, want image = %t", imageCount, tt.wantImage)
			}
		})
	}
}

func TestSaveBiliLoginInfoRestrictsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bili_info.dat")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveBiliLoginInfo(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("file permissions = %#o, want 0600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("file contents = %q", data)
	}
}
