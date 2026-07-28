package jmcomic

import (
	"MacArthurGo/structs/cqcode"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	jmapi "github.com/laoin114514/jmapi"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	_ "golang.org/x/image/webp"
)

const (
	defaultDataDir        = "data/jm"
	defaultTimeout        = 25 * time.Second
	defaultRetryTimes     = 1
	defaultImageWorkers   = 8
	defaultChapterWorkers = 2
	maxInfoChapters       = 20
)

type RequestKind int

const (
	RequestAlbum RequestKind = iota + 1
	RequestChapter
	RequestInfo
)

type Request struct {
	Kind     RequestKind
	AlbumID  string
	Sequence int
}

type Options struct {
	DataDir        string
	Timeout        time.Duration
	RetryTimes     int
	ImageWorkers   int
	ChapterWorkers int
}

type DownloadResult struct {
	AlbumID    string
	Path       string
	UploadName string
	WorkDir    string
	PageCount  int
}

type Service struct {
	options Options
}

type photoSourceNormalizer struct {
	jmapi.PluginAdapter
}

var disablePDFCPUConfig sync.Once

func (photoSourceNormalizer) Key() string {
	return "macarthur-photo-source-normalizer"
}

func (photoSourceNormalizer) BeforePhoto(_ jmapi.PluginContext, photo *jmapi.PhotoDetail) error {
	if photo == nil {
		return nil
	}
	for _, value := range []*string{
		&photo.DataOriginalDomain,
		&photo.DataOriginal0,
		&photo.DataOriginalQuery,
	} {
		if strings.EqualFold(strings.TrimSpace(*value), "<nil>") {
			*value = ""
		}
	}
	return nil
}

func ParseRequest(command string, message []cqcode.ArrayMessage) (Request, error) {
	words := messageWords(message)
	if len(words) > 0 && words[0] == command {
		words = words[1:]
	}

	switch command {
	case "/jm":
		if len(words) != 1 || !validAlbumID(words[0]) {
			return Request{}, errors.New("用法：/jm <ID>")
		}
		return Request{Kind: RequestAlbum, AlbumID: words[0]}, nil
	case "/jmc":
		if len(words) != 2 || !validAlbumID(words[0]) {
			return Request{}, errors.New("用法：/jmc <本子ID> <章节序号>")
		}
		sequence, err := strconv.Atoi(words[1])
		if err != nil || sequence < 1 {
			return Request{}, errors.New("章节序号必须是从 1 开始的整数")
		}
		return Request{Kind: RequestChapter, AlbumID: words[0], Sequence: sequence}, nil
	case "/jmi":
		if len(words) != 1 || !validAlbumID(words[0]) {
			return Request{}, errors.New("用法：/jmi <ID>")
		}
		return Request{Kind: RequestInfo, AlbumID: words[0]}, nil
	default:
		return Request{}, fmt.Errorf("unsupported JM command %q", command)
	}
}

func ValidateDownloadRequest(request Request) error {
	if request.AlbumID == "350234" && (request.Kind == RequestAlbum || request.Kind == RequestChapter) {
		return errors.New("董卓滚啊")
	}
	return nil
}

func ProgressMessage(request Request) string {
	if request.Kind != RequestChapter {
		return ""
	}
	return fmt.Sprintf("正在下载 JM%s 并生成加密 PDF，请稍候…", request.AlbumID)
}

func NewService(options Options) (*Service, error) {
	if strings.TrimSpace(options.DataDir) == "" {
		options.DataDir = defaultDataDir
	}
	absDataDir, err := filepath.Abs(options.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve JM data directory: %w", err)
	}
	options.DataDir = absDataDir
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if options.RetryTimes < 0 {
		options.RetryTimes = defaultRetryTimes
	}
	if options.ImageWorkers <= 0 {
		options.ImageWorkers = defaultImageWorkers
	}
	if options.ChapterWorkers <= 0 {
		options.ChapterWorkers = defaultChapterWorkers
	}
	return &Service{options: options}, nil
}

func (s *Service) GetAlbum(ctx context.Context, albumID string) (*jmapi.AlbumDetail, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validAlbumID(albumID) {
		return nil, errors.New("invalid album ID")
	}
	album, err := s.newClient(true).GetAlbumDetail(albumID)
	if err != nil {
		return nil, fmt.Errorf("get album %s: %w", albumID, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if album.ID == "" {
		album.ID = albumID
	}
	return album, nil
}

func (s *Service) DownloadAlbum(ctx context.Context, albumID, password string) (*DownloadResult, error) {
	return s.downloadAlbum(ctx, albumID, password, nil)
}

func (s *Service) DownloadAlbumWithMetadata(ctx context.Context, albumID, password string, onMetadata func(*jmapi.AlbumDetail)) (*DownloadResult, error) {
	return s.downloadAlbum(ctx, albumID, password, onMetadata)
}

func (s *Service) downloadAlbum(ctx context.Context, albumID, password string, onMetadata func(*jmapi.AlbumDetail)) (*DownloadResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validAlbumID(albumID) {
		return nil, errors.New("invalid album ID")
	}
	client := s.newClient(true)
	album, err := client.GetAlbumDetail(albumID)
	if err != nil {
		return nil, fmt.Errorf("get album %s: %w", albumID, err)
	}
	if album.ID == "" {
		album.ID = albumID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if onMetadata != nil {
		onMetadata(album)
	}
	photoIDs := PhotoIDs(album)
	if len(photoIDs) == 0 {
		return nil, fmt.Errorf("album %s has no downloadable chapters", albumID)
	}
	return s.download(ctx, album, photoIDs, 0, password, client.Domains())
}

func (s *Service) DownloadChapter(ctx context.Context, albumID string, sequence int, password string) (*DownloadResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validAlbumID(albumID) {
		return nil, errors.New("invalid album ID")
	}
	client := s.newClient(true)
	album, err := client.GetAlbumDetail(albumID)
	if err != nil {
		return nil, fmt.Errorf("get album %s: %w", albumID, err)
	}
	if album.ID == "" {
		album.ID = albumID
	}
	photoID, err := SelectChapter(album, sequence)
	if err != nil {
		return nil, err
	}
	return s.download(ctx, album, []string{photoID}, sequence, password, client.Domains())
}

func PhotoIDs(album *jmapi.AlbumDetail) []string {
	if album == nil {
		return nil
	}
	ids := make([]string, 0, len(album.EpisodeList)+len(album.EpisodeIDs)+1)
	for _, episode := range album.EpisodeList {
		ids = appendUniqueID(ids, episode.PhotoID)
	}
	if len(ids) == 0 {
		for _, id := range album.EpisodeIDs {
			ids = appendUniqueID(ids, id)
		}
	}
	if len(ids) == 0 {
		ids = appendUniqueID(ids, album.ID)
	}
	return ids
}

func SelectChapter(album *jmapi.AlbumDetail, sequence int) (string, error) {
	if sequence < 1 {
		return "", errors.New("chapter sequence must start at 1")
	}
	ids := PhotoIDs(album)
	if sequence > len(ids) {
		return "", fmt.Errorf("章节序号超出范围，本子共 %d 章", len(ids))
	}
	return ids[sequence-1], nil
}

func FormatAlbumInfo(album *jmapi.AlbumDetail) string {
	if album == nil {
		return "未获取到本子详情"
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "JM%s\n标题：%s", fallback(album.ID, "未知"), fallback(album.Name, "未知"))
	fmt.Fprintf(&builder, "\n作者：%s", joinOrUnknown(album.Author))
	fmt.Fprintf(&builder, "\n标签：%s", joinOrUnknown(album.Tags))
	fmt.Fprintf(&builder, "\n章节数：%d", len(PhotoIDs(album)))
	if album.PageCount > 0 {
		fmt.Fprintf(&builder, "\n页数：%d", album.PageCount)
	}
	if album.Views != "" || album.Likes != "" {
		fmt.Fprintf(&builder, "\n浏览/喜欢：%s / %s", fallback(album.Views, "未知"), fallback(album.Likes, "未知"))
	}
	if album.UpdateDate != "" {
		fmt.Fprintf(&builder, "\n更新：%s", album.UpdateDate)
	}
	if description := truncateRunes(strings.TrimSpace(album.Description), 300); description != "" {
		fmt.Fprintf(&builder, "\n简介：%s", description)
	}
	if len(album.EpisodeList) > 0 {
		builder.WriteString("\n章节：")
		limit := min(len(album.EpisodeList), maxInfoChapters)
		for i := 0; i < limit; i++ {
			title := fallback(strings.TrimSpace(album.EpisodeList[i].Title), "未命名")
			fmt.Fprintf(&builder, "\n%d. %s", i+1, title)
		}
		if remaining := len(album.EpisodeList) - limit; remaining > 0 {
			fmt.Fprintf(&builder, "\n…另有 %d 章", remaining)
		}
	}
	return builder.String()
}

func FormatDownloadProgress(album *jmapi.AlbumDetail) string {
	if album == nil {
		return "正在下载 JM未知 并生成加密 PDF，请稍候…\n标题：未知\n作者：未知\n标签：未知"
	}
	return fmt.Sprintf(
		"正在下载 JM%s 并生成加密 PDF，请稍候…\n标题：%s\n作者：%s\n标签：%s",
		fallback(album.ID, "未知"),
		fallback(album.Name, "未知"),
		joinOrUnknown(album.Author),
		joinOrUnknown(album.Tags),
	)
}

func BuildEncryptedPDF(imagePaths []string, outputPath, password string) error {
	if len(imagePaths) == 0 {
		return errors.New("cannot build PDF without images")
	}
	if strings.TrimSpace(password) == "" {
		return errors.New("PDF password is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create PDF directory: %w", err)
	}

	disablePDFCPUConfig.Do(api.DisableConfigDir)
	plainPath := outputPath + ".plain.pdf"
	for _, path := range []string{plainPath, outputPath} {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refuse to overwrite existing PDF path %q", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect PDF path %q: %w", path, err)
		}
	}
	if err := api.ImportImagesFile(imagePaths, plainPath, nil, nil); err != nil {
		_ = os.Remove(plainPath)
		return fmt.Errorf("create PDF from images: %w", err)
	}

	ownerPassword, err := randomOwnerPassword()
	if err != nil {
		_ = os.Remove(plainPath)
		return fmt.Errorf("generate PDF owner password: %w", err)
	}
	conf := model.NewAESConfiguration(password, ownerPassword, 256)
	conf.Permissions = model.PermissionsAll
	if err := api.EncryptFile(plainPath, outputPath, conf); err != nil {
		_ = os.Remove(plainPath)
		_ = os.Remove(outputPath)
		return fmt.Errorf("encrypt PDF: %w", err)
	}
	if err := os.Remove(plainPath); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("remove unencrypted PDF: %w", err)
	}

	validation := model.NewDefaultConfiguration()
	validation.UserPW = password
	if err := api.ValidateFile(outputPath, validation); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("validate encrypted PDF: %w", err)
	}
	if err := os.Chmod(outputPath, 0o600); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("secure PDF permissions: %w", err)
	}
	return nil
}

func (s *Service) download(ctx context.Context, album *jmapi.AlbumDetail, photoIDs []string, sequence int, password string, domains []string) (_ *DownloadResult, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(password) == "" {
		return nil, errors.New("PDF password is required")
	}
	if err := os.MkdirAll(s.options.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create JM data directory: %w", err)
	}
	workDir, err := os.MkdirTemp(s.options.DataDir, "jm-"+album.ID+"-")
	if err != nil {
		return nil, fmt.Errorf("create JM work directory: %w", err)
	}
	keepWorkDir := false
	defer func() {
		if !keepWorkDir {
			_ = os.RemoveAll(workDir)
		}
	}()

	imageRoot := filepath.Join(workDir, "images")
	imagePaths, err := s.downloadPhotos(ctx, photoIDs, imageRoot, domains)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fileStem := "JM" + album.ID
	if sequence > 0 {
		fileStem += fmt.Sprintf("-chapter-%d", sequence)
	}
	pdfPath := filepath.Join(workDir, fileStem+".pdf")
	if err := BuildEncryptedPDF(imagePaths, pdfPath, password); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(imageRoot); err != nil {
		return nil, fmt.Errorf("remove downloaded images: %w", err)
	}

	uploadName := safeUploadName(fileStem + "-" + fallback(album.Name, "untitled") + ".pdf")
	keepWorkDir = true
	return &DownloadResult{
		AlbumID:    album.ID,
		Path:       pdfPath,
		UploadName: uploadName,
		WorkDir:    workDir,
		PageCount:  len(imagePaths),
	}, nil
}

func (s *Service) downloadPhotos(ctx context.Context, photoIDs []string, imageRoot string, domains []string) ([]string, error) {
	type job struct {
		index   int
		photoID string
	}
	type result struct {
		index int
		paths []string
		err   error
	}

	jobs := make(chan job, len(photoIDs))
	results := make(chan result, len(photoIDs))
	for index, photoID := range photoIDs {
		jobs <- job{index: index, photoID: photoID}
	}
	close(jobs)

	workerCount := min(s.options.ChapterWorkers, len(photoIDs))
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				if err := ctx.Err(); err != nil {
					results <- result{index: item.index, err: err}
					continue
				}
				downloader := jmapi.NewDownloader(s.downloadOption(imageRoot, domains))
				downloader.RegisterPlugin(photoSourceNormalizer{})
				photo, err := downloader.DownloadPhoto(item.photoID)
				if err == nil {
					err = downloader.RaiseIfHasFailures()
				}
				var paths []string
				if err == nil && photo != nil {
					paths = append(paths, downloader.SuccessImages[photo.ID]...)
					if len(paths) == 0 {
						paths = append(paths, downloader.SuccessImages[item.photoID]...)
					}
					sort.Slice(paths, func(i, j int) bool {
						return filepath.Base(paths[i]) < filepath.Base(paths[j])
					})
					if len(paths) == 0 {
						err = errors.New("chapter contains no downloaded images")
					}
				}
				if err != nil {
					err = fmt.Errorf("download chapter %s: %w", item.photoID, err)
				}
				results <- result{index: item.index, paths: paths, err: err}
			}
		}()
	}
	go func() {
		workers.Wait()
		close(results)
	}()

	ordered := make([][]string, len(photoIDs))
	var firstErr error
	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
		}
		ordered[result.index] = result.paths
	}
	if firstErr != nil {
		return nil, firstErr
	}
	var imagePaths []string
	for _, chapterPaths := range ordered {
		imagePaths = append(imagePaths, chapterPaths...)
	}
	return imagePaths, nil
}

func (s *Service) newClient(updateHost bool) *jmapi.Client {
	config := s.clientConfig(nil)
	config.AutoUpdateHost = updateHost
	config.AutoEnsureCookies = updateHost
	return jmapi.NewClient(config)
}

func (s *Service) downloadOption(imageRoot string, domains []string) jmapi.Option {
	option := jmapi.DefaultOption()
	option.Log = false
	option.ClientConfig = s.clientConfig(domains)
	option.DirRule.BaseDir = imageRoot
	option.DirRule.Rule = "Bd_Pid"
	option.Download.Image.Suffix = ".jpg"
	option.Download.Threading.Image = s.options.ImageWorkers
	option.Download.Threading.Photo = 1
	return option
}

func (s *Service) clientConfig(domains []string) jmapi.Config {
	return jmapi.Config{
		ClientType:        jmapi.ClientTypeAPI,
		Domains:           append([]string(nil), domains...),
		Timeout:           s.options.Timeout,
		RetryTimes:        s.options.RetryTimes,
		UseFixedTimestamp: true,
	}
}

func messageWords(message []cqcode.ArrayMessage) []string {
	var words []string
	for _, segment := range message {
		if segment.Type != "text" {
			continue
		}
		text, ok := segment.Data["text"].(string)
		if !ok {
			continue
		}
		words = append(words, strings.Fields(text)...)
	}
	return words
}

func validAlbumID(id string) bool {
	if id == "" {
		return false
	}
	for _, char := range id {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func appendUniqueID(ids []string, id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func joinOrUnknown(items []string) string {
	clean := make([]string, 0, len(items))
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			clean = append(clean, item)
		}
	}
	if len(clean) == 0 {
		return "未知"
	}
	return strings.Join(clean, "、")
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return strings.TrimSpace(value)
}

func truncateRunes(value string, maxRunes int) string {
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes]) + "…"
}

func safeUploadName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer(
		"\\", "_", "/", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	).Replace(name)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		return "JM.pdf"
	}
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	if extension == "" {
		extension = ".pdf"
	}
	return truncateRunes(base, 115) + extension
}

func randomOwnerPassword() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
