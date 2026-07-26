package test

import (
	"MacArthurGo/internal/jmcomic"
	"MacArthurGo/internal/napcatstream"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jmapi "github.com/laoin114514/jmapi"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestJMParseRequest(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		text     string
		wantKind jmcomic.RequestKind
		wantID   string
		wantSeq  int
		wantErr  string
	}{
		{name: "album clean message", command: "/jm", text: "123456", wantKind: jmcomic.RequestAlbum, wantID: "123456"},
		{name: "album raw message", command: "/jm", text: "/jm 123456", wantKind: jmcomic.RequestAlbum, wantID: "123456"},
		{name: "chapter", command: "/jmc", text: "123456 3", wantKind: jmcomic.RequestChapter, wantID: "123456", wantSeq: 3},
		{name: "info", command: "/jmi", text: "123456", wantKind: jmcomic.RequestInfo, wantID: "123456"},
		{name: "non numeric id", command: "/jm", text: "JM123", wantErr: "/jm <ID>"},
		{name: "zero chapter", command: "/jmc", text: "123456 0", wantErr: "从 1 开始"},
		{name: "missing chapter", command: "/jmc", text: "123456", wantErr: "/jmc"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := jmcomic.ParseRequest(test.command, []cqcode.ArrayMessage{*cqcode.Text(test.text)})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ParseRequest() error = %v, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRequest() error = %v", err)
			}
			if request.Kind != test.wantKind || request.AlbumID != test.wantID || request.Sequence != test.wantSeq {
				t.Fatalf("ParseRequest() = %#v", request)
			}
		})
	}
}

func TestJMPhotoIDsAndChapterSelection(t *testing.T) {
	album := &jmapi.AlbumDetail{
		ID: "100",
		EpisodeList: []jmapi.Episode{
			{PhotoID: "201", Index: 1, Title: "第一章"},
			{PhotoID: "202", Index: 2, Title: "第二章"},
			{PhotoID: "202", Index: 3, Title: "重复项"},
		},
		EpisodeIDs: []string{"ignored"},
	}
	ids := jmcomic.PhotoIDs(album)
	if got := strings.Join(ids, ","); got != "201,202" {
		t.Fatalf("PhotoIDs() = %q", got)
	}
	chapter, err := jmcomic.SelectChapter(album, 2)
	if err != nil || chapter != "202" {
		t.Fatalf("SelectChapter() = %q, %v", chapter, err)
	}
	if _, err := jmcomic.SelectChapter(album, 3); err == nil || !strings.Contains(err.Error(), "共 2 章") {
		t.Fatalf("SelectChapter() out-of-range error = %v", err)
	}

	single := &jmapi.AlbumDetail{ID: "300"}
	if got := jmcomic.PhotoIDs(single); len(got) != 1 || got[0] != "300" {
		t.Fatalf("single album PhotoIDs() = %#v", got)
	}
}

func TestJMFormatAlbumInfo(t *testing.T) {
	album := &jmapi.AlbumDetail{
		ID:          "123",
		Name:        "测试本子",
		Author:      []string{"作者甲", "作者乙"},
		Tags:        []string{"标签一", "标签二"},
		PageCount:   42,
		Views:       "1000",
		Likes:       "99",
		EpisodeList: []jmapi.Episode{{PhotoID: "1", Title: "开篇"}, {PhotoID: "2", Title: "终章"}},
	}
	info := jmcomic.FormatAlbumInfo(album)
	for _, want := range []string{"JM123", "标题：测试本子", "作者：作者甲、作者乙", "标签：标签一、标签二", "章节数：2", "页数：42", "1. 开篇", "2. 终章"} {
		if !strings.Contains(info, want) {
			t.Fatalf("FormatAlbumInfo() missing %q in %q", want, info)
		}
	}
}

func TestJMBuildEncryptedPDF(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "00001.png")
	second := filepath.Join(dir, "00002.jpg")
	writeJMTestImage(t, first, 80, 120, color.RGBA{R: 220, G: 30, B: 60, A: 255})
	writeJMTestJPEG(t, second, 120, 80, color.RGBA{R: 30, G: 100, B: 220, A: 255})

	output := filepath.Join(dir, "JM123.pdf")
	const password = "test-password"
	if err := jmcomic.BuildEncryptedPDF([]string{first, second}, output, password); err != nil {
		t.Fatalf("BuildEncryptedPDF() error = %v", err)
	}
	if _, err := os.Stat(output + ".plain.pdf"); !os.IsNotExist(err) {
		t.Fatalf("unencrypted intermediate still exists: %v", err)
	}
	if err := api.ValidateFile(output, model.NewDefaultConfiguration()); err == nil {
		t.Fatal("encrypted PDF validated without a password")
	}

	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	pageCount, err := api.PageCount(file, conf)
	if err != nil {
		t.Fatalf("read encrypted PDF with password: %v", err)
	}
	if pageCount != 2 {
		t.Fatalf("encrypted PDF page count = %d, want 2", pageCount)
	}

	if qaDir := os.Getenv("MACARTHURGO_PDF_QA_DIR"); qaDir != "" {
		if err := os.MkdirAll(qaDir, 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(qaDir, "jm-encrypted-sample.pdf"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSendFileWithEcho_UploadTargets(t *testing.T) {
	tests := []struct {
		name       string
		message    *structs.MessageStruct
		wantAction string
		wantID     int64
	}{
		{name: "group", message: &structs.MessageStruct{MessageType: "group", GroupId: 12345}, wantAction: "upload_group_file", wantID: 12345},
		{name: "private", message: &structs.MessageStruct{MessageType: "private", UserId: 67890}, wantAction: "upload_private_file", wantID: 67890},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := essentials.SendFileWithEcho(test.message, `C:\data\JM123.pdf`, "JM123.pdf", "jmUpload:1")
			var action struct {
				Action string `json:"action"`
				Echo   string `json:"echo"`
				Params struct {
					GroupID int64  `json:"group_id"`
					UserID  int64  `json:"user_id"`
					File    string `json:"file"`
					Name    string `json:"name"`
				} `json:"params"`
			}
			if err := json.Unmarshal(payload, &action); err != nil {
				t.Fatal(err)
			}
			gotID := action.Params.GroupID
			if gotID == 0 {
				gotID = action.Params.UserID
			}
			if action.Action != test.wantAction || action.Echo != "jmUpload:1" || gotID != test.wantID || action.Params.Name != "JM123.pdf" {
				t.Fatalf("upload action = %#v", action)
			}
		})
	}
}

func TestNapCatStreamUploadChunks(t *testing.T) {
	data := bytes.Repeat([]byte("encrypted-pdf-data"), 37)
	path := filepath.Join(t.TempDir(), "JM123.pdf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	upload, err := napcatstream.NewUpload(path, "JM123.pdf", 64)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(data)
	if upload.SHA256 != hex.EncodeToString(wantHash[:]) || upload.TotalChunks < 2 {
		t.Fatalf("stream metadata = %#v", upload)
	}

	var reconstructed []byte
	for index := 0; index < upload.TotalChunks; index++ {
		params, err := upload.Chunk(index)
		if err != nil {
			t.Fatal(err)
		}
		if params.StreamID != upload.ID || params.ChunkIndex != index || params.TotalChunks != upload.TotalChunks || params.FileSize != int64(len(data)) || params.ExpectedSHA256 != upload.SHA256 || params.FileRetention <= 0 {
			t.Fatalf("chunk %d metadata = %#v", index, params)
		}
		chunk, err := base64.StdEncoding.DecodeString(params.ChunkData)
		if err != nil {
			t.Fatal(err)
		}
		reconstructed = append(reconstructed, chunk...)
	}
	if !bytes.Equal(reconstructed, data) {
		t.Fatal("stream chunks did not reconstruct the source file")
	}
	complete := upload.Complete()
	if complete.StreamID != upload.ID || !complete.IsComplete || complete.FileRetention <= 0 {
		t.Fatalf("complete params = %#v", complete)
	}
}

func TestNapCatStreamEchoFields(t *testing.T) {
	var echo structs.EchoMessageStruct
	payload := []byte(`{"status":"ok","retcode":0,"data":{"type":"response","stream_id":"stream-1","status":"file_complete","received_chunks":3,"total_chunks":3,"file_path":"/tmp/JM123.pdf","file_size":42,"sha256":"abc"},"message":"","wording":"","echo":"jmStream:stream-1:complete","stream":"stream-action"}`)
	if err := json.Unmarshal(payload, &echo); err != nil {
		t.Fatal(err)
	}
	if echo.Data.Type != "response" || echo.Data.StreamID != "stream-1" || echo.Data.Status != "file_complete" || echo.Data.ReceivedChunks != 3 || echo.Data.TotalChunks != 3 || echo.Data.FilePath != "/tmp/JM123.pdf" || echo.Data.FileSize != 42 || echo.Data.SHA256 != "abc" || echo.Stream != "stream-action" {
		t.Fatalf("stream echo = %#v", echo)
	}
}

func writeJMTestImage(t *testing.T, path string, width, height int, fill color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func writeJMTestJPEG(t *testing.T, path string, width, height int, fill color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
}
