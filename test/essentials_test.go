package test

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type concurrencyHandler struct {
	current atomic.Int64
	maximum atomic.Int64
}

func (h *concurrencyHandler) ReceiveMessage(*structs.MessageStruct, chan<- []byte) {
	current := h.current.Add(1)
	for {
		maximum := h.maximum.Load()
		if current <= maximum || h.maximum.CompareAndSwap(maximum, current) {
			break
		}
	}
	time.Sleep(2 * time.Millisecond)
	h.current.Add(-1)
}

func (*concurrencyHandler) ReceiveEcho(*structs.EchoMessageStruct, chan<- []byte) {}

func TestDatabase_CRUD(t *testing.T) {
	if err := essentials.OpenDatabase(filepath.Join(t.TempDir(), "cache.db")); err != nil {
		t.Fatalf("OpenDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if err := essentials.CloseDatabase(); err != nil {
			t.Errorf("CloseDatabase() error = %v", err)
		}
	})

	if err := essentials.CreateDB("items", []string{"id", "value"}, []string{"TEXT PRIMARY KEY", "TEXT NOT NULL"}); err != nil {
		t.Fatalf("CreateDB() error = %v", err)
	}
	if err := essentials.InsertDB("items", []string{"id", "value"}, []any{"one", "first"}); err != nil {
		t.Fatalf("InsertDB() error = %v", err)
	}
	rows, err := essentials.SelectDB("items", "value", "id", "one")
	if err != nil {
		t.Fatalf("SelectDB() error = %v", err)
	}
	if len(rows) != 1 || rows[0]["value"] != "first" {
		t.Fatalf("SelectDB() rows = %#v", rows)
	}
	if err := essentials.UpdateDB("items", "id", "one", []string{"value"}, []any{"updated"}); err != nil {
		t.Fatalf("UpdateDB() error = %v", err)
	}
	rows, err = essentials.SelectDB("items", "value", "id", "one")
	if err != nil || len(rows) != 1 || rows[0]["value"] != "updated" {
		t.Fatalf("updated rows = %#v, error = %v", rows, err)
	}
}

func TestDatabase_RejectsInvalidIdentifier(t *testing.T) {
	if err := essentials.CreateDB("items; DROP TABLE items", []string{"id"}, []string{"TEXT"}); err == nil {
		t.Fatal("CreateDB() accepted an invalid identifier")
	}
}

func TestGetImageKey_DifferentImagesDoNotShareRKey(t *testing.T) {
	first := "https://multimedia.nt.qq.com.cn/download?appid=1407&fileid=image-a&spec=0&rkey=shared-token"
	second := "https://multimedia.nt.qq.com.cn/download?appid=1407&fileid=image-b&spec=0&rkey=shared-token"

	if firstKey, secondKey := essentials.GetImageKey(first), essentials.GetImageKey(second); firstKey == secondKey {
		t.Fatalf("GetImageKey() returned the same key %q for different images", firstKey)
	}
}

func TestGetImageKey_RKeyDoesNotChangeImageIdentity(t *testing.T) {
	first := "https://multimedia.nt.qq.com.cn/download?rkey=token-one&spec=0&fileid=image-a&appid=1407"
	second := "https://multimedia.nt.qq.com.cn/download?appid=1407&fileid=image-a&spec=0&rkey=token-two"

	if firstKey, secondKey := essentials.GetImageKey(first), essentials.GetImageKey(second); firstKey != secondKey {
		t.Fatalf("GetImageKey() keys differ after only rkey/query order changed: %q != %q", firstKey, secondKey)
	}
}

func TestLoginInfo_GroupListSignalsReadiness(t *testing.T) {
	info := &essentials.LoginInfo{}

	payload := []byte(`{"status":"ok","data":[{"group_id":790252716,"group_name":"test","member_count":93,"max_member_count":200}],"message":"","echo":"groupList"}`)
	var array structs.EchoMessageArrayStruct
	if err := json.Unmarshal(payload, &array); err != nil {
		t.Fatalf("decode group list: %v", err)
	}
	info.ReceiveEcho(&structs.EchoMessageStruct{
		DataArray: array.Data,
		Echo:      array.Echo,
		Status:    array.Status,
	}, make(chan []byte, 1))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if !info.WaitForGroups(ctx) {
		t.Fatal("group list did not signal readiness")
	}
	groups := info.Groups()
	if len(groups) != 1 || groups[0].GroupId != 790252716 {
		t.Fatalf("groups = %#v", groups)
	}
}

func TestPlugin_SerializesStatefulCallbacks(t *testing.T) {
	handler := &concurrencyHandler{}
	plugin := &essentials.Plugin{Name: "serialized", Enabled: true, Handler: handler}

	var group sync.WaitGroup
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			plugin.HandleMessage(&structs.MessageStruct{}, make(chan []byte, 1))
		}()
	}
	group.Wait()
	if got := handler.maximum.Load(); got != 1 {
		t.Fatalf("maximum concurrent callbacks = %d, want 1", got)
	}
}

func TestRegister_RejectsDuplicateName(t *testing.T) {
	handler := &concurrencyHandler{}
	name := fmt.Sprintf("test-duplicate-%p", handler)
	first := &essentials.Plugin{Name: name, Handler: handler}
	second := &essentials.Plugin{Name: name, Handler: &concurrencyHandler{}}
	if err := essentials.Register(first); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := essentials.Register(second); err == nil {
		t.Fatal("second Register() unexpectedly succeeded")
	}
}

func TestUpdate_IgnoresNonCommandMessages(t *testing.T) {
	oldConfig, oldBranch := base.Config, base.Branch
	base.Config = &base.Configuration{UpdateUrl: "https://example.com/"}
	base.Branch = "Release"
	t.Cleanup(func() {
		base.Config = oldConfig
		base.Branch = oldBranch
	})

	send := make(chan []byte, 1)
	(&essentials.Update{}).ReceiveMessage(&structs.MessageStruct{Command: ""}, send)
	select {
	case message := <-send:
		t.Fatalf("non-command message produced response %q", message)
	default:
	}
}

func TestRegisterAll_ExplicitStartup(t *testing.T) {
	const helperEnv = "MACARTHURGO_REGISTER_ALL_HELPER"
	if os.Getenv(helperEnv) == "1" {
		base.Config = &base.Configuration{}
		if err := plugins.RegisterAll(); err != nil {
			t.Fatalf("RegisterAll() error = %v", err)
		}
		if got := essentials.PluginCount(); got != 14 {
			t.Fatalf("PluginCount() = %d, want 14", got)
		}
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestRegisterAll_ExplicitStartup$")
	command.Env = append(os.Environ(), helperEnv+"=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated registration test failed: %v\n%s", err, output)
	}
}
