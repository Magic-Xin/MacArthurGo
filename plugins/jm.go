package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/internal/jmcomic"
	"MacArthurGo/internal/napcatstream"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	jmapi "github.com/laoin114514/jmapi"
)

const (
	jmStreamEchoPrefix = "jmStream:"
	jmUploadEchoPrefix = "jmUpload:"
)

type JM struct {
	service  *jmcomic.Service
	password string
	jobs     chan struct{}

	jobMu    sync.Mutex
	stopping bool
	wg       sync.WaitGroup

	contextMu sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	pendingMu sync.Mutex
	pending   map[string]*jmPendingUpload
}

type jmPendingUpload struct {
	mu sync.Mutex

	message      structs.MessageStruct
	workDir      string
	upload       *napcatstream.Upload
	nextChunk    int
	completeSent bool
	finalSent    bool
	timer        *time.Timer
}

func registerJM() error {
	config := base.Config.Plugins.JM
	service, err := jmcomic.NewService(jmcomic.Options{
		DataDir:        config.DataDir,
		Timeout:        time.Duration(config.TimeoutSeconds) * time.Second,
		RetryTimes:     config.RetryTimes,
		ImageWorkers:   config.ImageWorkers,
		ChapterWorkers: config.ChapterWorkers,
	})
	if err != nil {
		return err
	}
	maxJobs := config.MaxConcurrentJobs
	if maxJobs <= 0 {
		maxJobs = 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	handler := &JM{
		service:  service,
		password: config.PDFPassword,
		jobs:     make(chan struct{}, maxJobs),
		ctx:      ctx,
		cancel:   cancel,
		pending:  make(map[string]*jmPendingUpload),
	}
	return essentials.Register(&essentials.Plugin{
		Name:    "JM 本子下载",
		Enabled: config.Enable,
		Args:    []string{"/jm", "/jmc", "/jmi"},
		Handler: handler,
	})
}

func (j *JM) Start(ctx context.Context, _ chan<- []byte) {
	j.contextMu.Lock()
	if j.cancel != nil {
		j.cancel()
	}
	j.ctx, j.cancel = context.WithCancel(ctx)
	j.contextMu.Unlock()
}

func (j *JM) Stop() error {
	j.jobMu.Lock()
	j.stopping = true
	j.contextMu.RLock()
	cancel := j.cancel
	j.contextMu.RUnlock()
	if cancel != nil {
		cancel()
	}
	j.jobMu.Unlock()

	j.wg.Wait()
	j.cleanupPendingUploads()
	return nil
}

func (j *JM) ReceiveMessage(message *structs.MessageStruct, send chan<- []byte) {
	if message == nil || !isJMCommand(message.Command) {
		return
	}
	ctx := j.currentContext()
	segments := message.CleanMessage
	if segments == nil {
		segments = message.Message
	}
	request, err := jmcomic.ParseRequest(message.Command, segments)
	if err != nil {
		j.send(ctx, send, essentials.SendMsg(message, err.Error(), nil, false, true, ""))
		return
	}
	if err := jmcomic.ValidateDownloadRequest(request); err != nil {
		j.send(ctx, send, essentials.SendMsg(message, err.Error(), nil, false, true, ""))
		return
	}
	if !j.startJob() {
		j.send(ctx, send, essentials.SendMsg(message, "JM 任务已满，请稍后再试", nil, false, true, ""))
		return
	}

	if status := jmcomic.ProgressMessage(request); status != "" {
		j.send(ctx, send, essentials.SendMsg(message, status, nil, false, true, ""))
	}
	origin := *message
	go j.runJob(request, origin, send)
}

func (j *JM) ReceiveEcho(echo *structs.EchoMessageStruct, send chan<- []byte) {
	if echo == nil {
		return
	}
	if strings.HasPrefix(echo.Echo, jmStreamEchoPrefix) {
		j.receiveStreamEcho(echo, send)
		return
	}
	if !strings.HasPrefix(echo.Echo, jmUploadEchoPrefix) {
		return
	}
	streamID := strings.TrimPrefix(echo.Echo, jmUploadEchoPrefix)
	pending, ok := j.takePendingUpload(streamID)
	if !ok {
		return
	}
	if err := os.RemoveAll(pending.workDir); err != nil {
		log.Printf("JM remove uploaded work directory: %v", err)
	}
	if echo.Status != "ok" {
		j.send(j.currentContext(), send, essentials.SendMsg(&pending.message, "JM PDF 上传失败："+echoError(echo), nil, false, true, ""))
	}
}

func (j *JM) runJob(request jmcomic.Request, origin structs.MessageStruct, send chan<- []byte) {
	defer j.finishJob()
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("JM job panic: %v", recovered)
			j.send(j.currentContext(), send, essentials.SendMsg(&origin, "JM 处理失败，请稍后重试", nil, false, true, ""))
		}
	}()

	ctx := j.currentContext()
	if request.Kind == jmcomic.RequestInfo {
		album, err := j.service.GetAlbum(ctx, request.AlbumID)
		if err != nil {
			j.reportJobError(ctx, send, &origin, request, err)
			return
		}
		j.send(ctx, send, essentials.SendMsg(&origin, jmcomic.FormatAlbumInfo(album), nil, false, true, ""))
		return
	}

	var (
		result *jmcomic.DownloadResult
		err    error
	)
	if request.Kind == jmcomic.RequestAlbum {
		result, err = j.service.DownloadAlbumWithMetadata(ctx, request.AlbumID, j.password, func(album *jmapi.AlbumDetail) {
			j.send(ctx, send, essentials.SendMsg(&origin, jmcomic.FormatDownloadProgress(album), nil, false, true, ""))
		})
	} else {
		result, err = j.service.DownloadChapter(ctx, request.AlbumID, request.Sequence, j.password)
	}
	if err != nil {
		j.reportJobError(ctx, send, &origin, request, err)
		return
	}
	if ctx.Err() != nil {
		_ = os.RemoveAll(result.WorkDir)
		return
	}

	upload, err := napcatstream.NewUpload(result.Path, result.UploadName, 0)
	if err != nil {
		_ = os.RemoveAll(result.WorkDir)
		j.reportJobError(ctx, send, &origin, request, fmt.Errorf("prepare NapCat stream upload: %w", err))
		return
	}
	pending := j.addPendingUpload(origin, result.WorkDir, upload)
	message := fmt.Sprintf("PDF 已生成，共 %d 页，正在通过文件流上传。\n打开密码：%s", result.PageCount, j.password)
	j.send(ctx, send, essentials.SendMsg(&origin, message, nil, false, true, ""))
	if err := j.sendStreamChunk(ctx, send, pending, 0); err != nil {
		j.failPendingUpload(send, upload.ID, err)
	}
}

func (j *JM) receiveStreamEcho(echo *structs.EchoMessageStruct, send chan<- []byte) {
	streamID, step, ok := strings.Cut(strings.TrimPrefix(echo.Echo, jmStreamEchoPrefix), ":")
	if !ok || streamID == "" || step == "" {
		return
	}
	pending, ok := j.pendingUpload(streamID)
	if !ok {
		return
	}
	if echo.Status != "ok" {
		j.failPendingUpload(send, streamID, fmt.Errorf("NapCat stream action failed: %s", echoError(echo)))
		return
	}
	if echo.Data.StreamID != "" && echo.Data.StreamID != streamID {
		j.failPendingUpload(send, streamID, fmt.Errorf("NapCat returned stream ID %q, want %q", echo.Data.StreamID, streamID))
		return
	}
	if step == "complete" {
		j.receiveStreamComplete(echo, send, pending)
		return
	}
	chunkIndex, err := strconv.Atoi(step)
	if err != nil {
		return
	}

	pending.mu.Lock()
	if pending.completeSent || pending.finalSent || chunkIndex != pending.nextChunk {
		pending.mu.Unlock()
		return
	}
	pending.nextChunk++
	nextChunk := pending.nextChunk
	sendComplete := nextChunk == pending.upload.TotalChunks
	if sendComplete {
		pending.completeSent = true
	}
	pending.mu.Unlock()

	ctx := j.currentContext()
	if sendComplete {
		echoID := jmStreamEchoPrefix + streamID + ":complete"
		j.send(ctx, send, essentials.SendAction("upload_file_stream", pending.upload.Complete(), echoID))
		return
	}
	if err := j.sendStreamChunk(ctx, send, pending, nextChunk); err != nil {
		j.failPendingUpload(send, streamID, err)
	}
}

func (j *JM) receiveStreamComplete(echo *structs.EchoMessageStruct, send chan<- []byte, pending *jmPendingUpload) {
	if echo.Data.Status != "file_complete" || strings.TrimSpace(echo.Data.FilePath) == "" {
		j.failPendingUpload(send, pending.upload.ID, fmt.Errorf("NapCat did not return a completed stream file: %s", echoError(echo)))
		return
	}
	if echo.Data.FileSize != 0 && echo.Data.FileSize != pending.upload.Size {
		j.failPendingUpload(send, pending.upload.ID, fmt.Errorf("NapCat stream size is %d, want %d", echo.Data.FileSize, pending.upload.Size))
		return
	}
	if echo.Data.SHA256 != "" && !strings.EqualFold(echo.Data.SHA256, pending.upload.SHA256) {
		j.failPendingUpload(send, pending.upload.ID, errors.New("NapCat stream SHA-256 mismatch"))
		return
	}

	pending.mu.Lock()
	if !pending.completeSent || pending.finalSent {
		pending.mu.Unlock()
		return
	}
	pending.finalSent = true
	pending.mu.Unlock()

	echoID := jmUploadEchoPrefix + pending.upload.ID
	j.send(j.currentContext(), send, essentials.SendFileWithEcho(&pending.message, echo.Data.FilePath, pending.upload.Filename, echoID))
}

func (j *JM) sendStreamChunk(ctx context.Context, send chan<- []byte, pending *jmPendingUpload, chunkIndex int) error {
	params, err := pending.upload.Chunk(chunkIndex)
	if err != nil {
		return err
	}
	echo := jmStreamEchoPrefix + pending.upload.ID + ":" + strconv.Itoa(chunkIndex)
	j.send(ctx, send, essentials.SendAction("upload_file_stream", params, echo))
	return nil
}

func (j *JM) failPendingUpload(send chan<- []byte, streamID string, err error) {
	pending, ok := j.takePendingUpload(streamID)
	if !ok {
		return
	}
	if removeErr := os.RemoveAll(pending.workDir); removeErr != nil {
		log.Printf("JM remove failed stream work directory: %v", removeErr)
	}
	log.Printf("JM stream upload %s failed: %v", streamID, err)
	message := []rune(err.Error())
	if len(message) > 200 {
		message = append(message[:200], '…')
	}
	j.send(j.currentContext(), send, essentials.SendMsg(&pending.message, "JM PDF 上传失败："+string(message), nil, false, true, ""))
}

func echoError(echo *structs.EchoMessageStruct) string {
	if echo == nil {
		return "未知错误"
	}
	for _, value := range []string{echo.Wording, echo.Message, echo.Data.Status} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	if echo.Retcode != 0 {
		return fmt.Sprintf("retcode %d", echo.Retcode)
	}
	return "未知错误"
}

func (j *JM) reportJobError(ctx context.Context, send chan<- []byte, origin *structs.MessageStruct, request jmcomic.Request, err error) {
	if errorsIsContext(err) || ctx.Err() != nil {
		return
	}
	log.Printf("JM command %d for album %s failed: %v", request.Kind, request.AlbumID, err)
	errorText := []rune(err.Error())
	if len(errorText) > 200 {
		errorText = append(errorText[:200], '…')
	}
	j.send(ctx, send, essentials.SendMsg(origin, "JM 处理失败："+string(errorText), nil, false, true, ""))
}

func (j *JM) startJob() bool {
	j.jobMu.Lock()
	defer j.jobMu.Unlock()
	if j.stopping {
		return false
	}
	select {
	case j.jobs <- struct{}{}:
		j.wg.Add(1)
		return true
	default:
		return false
	}
}

func (j *JM) finishJob() {
	<-j.jobs
	j.wg.Done()
}

func (j *JM) currentContext() context.Context {
	j.contextMu.RLock()
	defer j.contextMu.RUnlock()
	if j.ctx == nil {
		return context.Background()
	}
	return j.ctx
}

func (j *JM) send(ctx context.Context, send chan<- []byte, payload []byte) {
	if len(payload) == 0 {
		return
	}
	select {
	case send <- payload:
	case <-ctx.Done():
	}
}

func (j *JM) addPendingUpload(message structs.MessageStruct, workDir string, upload *napcatstream.Upload) *jmPendingUpload {
	pending := &jmPendingUpload{message: message, workDir: workDir, upload: upload}
	j.pendingMu.Lock()
	j.pending[upload.ID] = pending
	pending.timer = time.AfterFunc(time.Hour, func() {
		expired, ok := j.takePendingUpload(upload.ID)
		if !ok {
			return
		}
		if err := os.RemoveAll(expired.workDir); err != nil {
			log.Printf("JM remove expired work directory: %v", err)
		}
	})
	j.pendingMu.Unlock()
	return pending
}

func (j *JM) pendingUpload(streamID string) (*jmPendingUpload, bool) {
	j.pendingMu.Lock()
	defer j.pendingMu.Unlock()
	pending, ok := j.pending[streamID]
	return pending, ok
}

func (j *JM) takePendingUpload(streamID string) (*jmPendingUpload, bool) {
	j.pendingMu.Lock()
	defer j.pendingMu.Unlock()
	pending, ok := j.pending[streamID]
	if !ok {
		return nil, false
	}
	delete(j.pending, streamID)
	if pending.timer != nil {
		pending.timer.Stop()
	}
	return pending, true
}

func (j *JM) cleanupPendingUploads() {
	j.pendingMu.Lock()
	pending := j.pending
	j.pending = make(map[string]*jmPendingUpload)
	j.pendingMu.Unlock()
	for _, upload := range pending {
		if upload.timer != nil {
			upload.timer.Stop()
		}
		if err := os.RemoveAll(upload.workDir); err != nil {
			log.Printf("JM remove pending work directory: %v", err)
		}
	}
}

func isJMCommand(command string) bool {
	return command == "/jm" || command == "/jmc" || command == "/jmi"
}

func errorsIsContext(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
