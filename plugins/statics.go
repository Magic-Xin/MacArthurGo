package plugins

import (
	"MacArthurGo/base"
	"MacArthurGo/plugins/essentials"
	"MacArthurGo/structs"
	"MacArthurGo/structs/cqcode"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/psykhi/wordclouds"
	chart "github.com/wcharczuk/go-chart/v2"
	"github.com/yanyiwu/gojieba"
)

const (
	staticsDateLayout = "2006-01-02"
	defaultDataDir    = "data/statics"
)

var defaultStopWords = []string{
	"的", "了", "呢", "啊", "吗", "吧", "你", "我", "他", "她", "它", "他们", "我们", "你们",
	"然后", "以及", "就是", "聊天", "群聊", "消息", "http", "https", "www", "com",
	"哈哈", "哈哈哈", "emm", "这边", "那边", "所以", "因为", "但是", "如果", "不是",
}

type groupStats struct {
	Hourly [24]int64        `json:"hourly"`
	Words  map[string]int64 `json:"words"`
}

type staticsStore struct {
	root       string
	retention  int
	data       map[string]map[int64]*groupStats
	dirtyDates map[string]bool
	mu         sync.RWMutex
	flushOnce  sync.Once
}

type Statics struct {
	store       *staticsStore
	tokenizer   *gojieba.Jieba
	tokenizerMu sync.Mutex
	stopWords   map[string]struct{}
}

func init() {
	cfg := base.Config.Plugins.Statics
	store, err := newStaticsStore(cfg.DataDir, cfg.RetentionDays)
	if err != nil {
		log.Printf("statics store init error: %v", err)
	}

	statics := &Statics{
		store:     store,
		stopWords: buildStopWords(cfg.StopWords),
	}

	if cfg.Enable {
		dictPath := "./jieba_dict/jieba.dict.utf8"
		hmmPath := "./jieba_dict/hmm_model.utf8"
		userPath := "./jieba_dict/user.dict.utf8"
		idfPath := "./jieba_dict/idf.utf8"
		stopPath := "./jieba_dict/stop_words.utf8"

		statics.tokenizer = gojieba.NewJieba(dictPath, hmmPath, userPath, idfPath, stopPath)
	}

	plugin := &essentials.Plugin{
		Name:      "群聊统计",
		Enabled:   cfg.Enable,
		Interface: statics,
	}
	essentials.PluginArray = append(essentials.PluginArray, plugin)
}

func (s *Statics) ReceiveAll(chan<- *[]byte) {}

func (s *Statics) ReceiveMessage(messageStruct *structs.MessageStruct, send chan<- *[]byte) {
	if messageStruct == nil || messageStruct.MessageType != "group" || messageStruct.GroupId == 0 {
		return
	}
	if s.store == nil || s.tokenizer == nil {
		if messageStruct.Command != "" {
			send <- essentials.SendMsg(messageStruct, "群聊统计模块尚未完成初始化", nil, false, true, "")
		}
		return
	}

	s.recordMessage(messageStruct)

	if messageStruct.Command == "" {
		return
	}

	if label, ok := essentials.CheckArgumentMap(messageStruct.Command, &base.Config.Plugins.Statics.ChartArgsMap); ok {
		s.respondWithChart(messageStruct, label, send)
		return
	}

	if label, ok := essentials.CheckArgumentMap(messageStruct.Command, &base.Config.Plugins.Statics.WordCloudArgsMap); ok {
		s.respondWithWordCloud(messageStruct, label, send)
	}
}

func (s *Statics) ReceiveEcho(*structs.EchoMessageStruct, chan<- *[]byte) {}

func (s *Statics) recordMessage(messageStruct *structs.MessageStruct) {
	if s.store == nil {
		return
	}

	msgTime := time.Unix(messageStruct.Time, 0)
	dateKey := msgTime.Format(staticsDateLayout)
	hour := msgTime.Hour()
	s.store.incrementMessage(dateKey, messageStruct.GroupId, hour)

	plain := s.collectPlainText(messageStruct)
	wordFreq := s.segmentWords(plain)
	if len(wordFreq) > 0 {
		s.store.addWordCounts(dateKey, messageStruct.GroupId, wordFreq)
	}
}

func (s *Statics) respondWithChart(messageStruct *structs.MessageStruct, label string, send chan<- *[]byte) {
	dateKey := resolveDateKey(label)
	counts := s.store.getHourly(dateKey, messageStruct.GroupId)
	var total int64
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		simple := fmt.Sprintf("%s暂无发言数据", describeDay(label))
		send <- essentials.SendMsg(messageStruct, simple, nil, false, true, "")
		return
	}

	img, err := s.renderLineChart(dateKey, counts)
	if err != nil {
		log.Printf("statics chart render error: %v", err)
		send <- essentials.SendMsg(messageStruct, "生成折线图失败，请稍后再试", nil, false, true, "")
		return
	}

	msg := fmt.Sprintf("%s发言折线图（%s），总计 %d 条", describeDay(label), dateKey, total)
	arr := buildImageArray(img)
	send <- essentials.SendMsg(messageStruct, msg, &arr, false, true, "")
}

func (s *Statics) respondWithWordCloud(messageStruct *structs.MessageStruct, label string, send chan<- *[]byte) {
	dateKey := resolveDateKey(label)
	freq := s.store.getWordFreq(dateKey, messageStruct.GroupId)
	if len(freq) == 0 {
		send <- essentials.SendMsg(messageStruct, fmt.Sprintf("%s暂无词频数据", describeDay(label)), nil, false, true, "")
		return
	}

	img, err := s.renderWordCloud(freq)
	if err != nil {
		log.Printf("statics wordcloud render error: %v", err)
		send <- essentials.SendMsg(messageStruct, err.Error(), nil, false, true, "")
		return
	}

	msg := fmt.Sprintf("%s词云（%s）", describeDay(label), dateKey)
	arr := buildImageArray(img)
	send <- essentials.SendMsg(messageStruct, msg, &arr, false, true, "")
}

func (s *Statics) collectPlainText(messageStruct *structs.MessageStruct) string {
	var segments []cqcode.ArrayMessage
	if messageStruct.CleanMessage != nil && len(*messageStruct.CleanMessage) > 0 {
		segments = *messageStruct.CleanMessage
	} else {
		segments = messageStruct.Message
	}
	if len(segments) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, seg := range segments {
		if seg.Type != "text" {
			continue
		}
		if text, ok := seg.Data["text"].(string); ok && text != "" {
			builder.WriteString(text)
			builder.WriteRune(' ')
		}
	}
	return builder.String()
}

func (s *Statics) segmentWords(plain string) map[string]int {
	plain = strings.TrimSpace(plain)
	if plain == "" || s.tokenizer == nil {
		return nil
	}
	s.tokenizerMu.Lock()
	words := s.tokenizer.CutForSearch(plain, true)
	s.tokenizerMu.Unlock()
	if len(words) == 0 {
		return nil
	}
	freq := make(map[string]int)
	for _, word := range words {
		normalized := normalizeWord(word)
		if normalized == "" {
			continue
		}
		if _, blocked := s.stopWords[normalized]; blocked {
			continue
		}
		freq[normalized]++
	}
	return freq
}

func (s *Statics) renderLineChart(dateKey string, counts [24]int64) ([]byte, error) {
	cfg := base.Config.Plugins.Statics.Chart
	width := cfg.Width
	if width <= 0 {
		width = 900
	}
	height := cfg.Height
	if height <= 0 {
		height = 520
	}

	xValues := make([]float64, 24)
	yValues := make([]float64, 24)
	var maxValue float64
	for i := 0; i < 24; i++ {
		xValues[i] = float64(i)
		yValues[i] = float64(counts[i])
		if yValues[i] > maxValue {
			maxValue = yValues[i]
		}
	}

	graph := chart.Chart{
		Width:  width,
		Height: height,
		Background: chart.Style{
			Padding: chart.Box{Top: 50, Left: 40, Right: 20, Bottom: 40},
		},
		XAxis: chart.XAxis{
			Name: "小时",
			ValueFormatter: func(v interface{}) string {
				value, ok := v.(float64)
				if !ok {
					return ""
				}
				h := int(math.Round(value))
				if h < 0 {
					h = 0
				}
				if h > 23 {
					h = 23
				}
				return fmt.Sprintf("%02d:00", h)
			},
		},
		YAxis: chart.YAxis{
			Name: "消息条数",
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    dateKey,
				XValues: xValues,
				YValues: yValues,
				Style: chart.Style{
					StrokeColor: chart.ColorBlue,
					StrokeWidth: 3,
					FillColor:   chart.ColorBlue.WithAlpha(50),
				},
			},
		},
	}

	if maxValue == 0 {
		maxValue = 1
	}
	graph.YAxis.Range = &chart.ContinuousRange{Min: 0, Max: math.Ceil(maxValue*1.15 + 1)}
	graph.Elements = []chart.Renderable{chart.Legend(&graph)}

	var buf bytes.Buffer
	if err := graph.Render(chart.PNG, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Statics) renderWordCloud(freq map[string]int64) ([]byte, error) {
	cfg := base.Config.Plugins.Statics.WordCloud
	fontPath := strings.TrimSpace(cfg.FontFile)
	if fontPath == "" {
		return nil, errors.New("请先在配置文件中设置词云字体文件路径")
	}
	if _, err := os.Stat(fontPath); err != nil {
		return nil, fmt.Errorf("无法读取字体文件: %w", err)
	}
	width := cfg.Width
	if width <= 0 {
		width = 900
	}
	height := cfg.Height
	if height <= 0 {
		height = 520
	}
	maxWords := cfg.MaxWords
	if maxWords <= 0 {
		maxWords = 80
	}

	type pair struct {
		word  string
		count int64
	}
	items := make([]pair, 0, len(freq))
	for word, count := range freq {
		if count <= 0 {
			continue
		}
		items = append(items, pair{word: word, count: count})
	}
	if len(items) == 0 {
		return nil, errors.New("暂无可用词频数据")
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})
	if len(items) > maxWords {
		items = items[:maxWords]
	}

	wordMap := make(map[string]int, len(items))
	for _, it := range items {
		wordMap[it.word] = int(it.count)
	}

	palette := []color.Color{
		color.RGBA{R: 59, G: 130, B: 246, A: 255},
		color.RGBA{R: 249, G: 115, B: 22, A: 255},
		color.RGBA{R: 16, G: 185, B: 129, A: 255},
		color.RGBA{R: 236, G: 72, B: 153, A: 255},
		color.RGBA{R: 139, G: 92, B: 246, A: 255},
	}

	wc := wordclouds.NewWordcloud(
		wordMap,
		wordclouds.FontFile(fontPath),
		wordclouds.FontMaxSize(int(float64(height)*0.18)),
		wordclouds.FontMinSize(16),
		wordclouds.Width(width),
		wordclouds.Height(height),
		wordclouds.Colors(palette),
		wordclouds.BackgroundColor(color.RGBA{R: 247, G: 249, B: 252, A: 255}),
		wordclouds.WordSizeFunction(wordclouds.SizeFunctionLinear),
	)
	img := wc.Draw()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *staticsStore) incrementMessage(dateKey string, groupId int64, hour int) {
	if hour < 0 || hour > 23 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.ensureGroupStats(dateKey, groupId)
	stats.Hourly[hour]++
	s.dirtyDates[dateKey] = true
}

func (s *staticsStore) addWordCounts(dateKey string, groupId int64, freq map[string]int) {
	if len(freq) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.ensureGroupStats(dateKey, groupId)
	if stats.Words == nil {
		stats.Words = make(map[string]int64)
	}
	for word, count := range freq {
		stats.Words[word] += int64(count)
	}
	s.dirtyDates[dateKey] = true
}

func (s *staticsStore) getHourly(dateKey string, groupId int64) [24]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if groups, ok := s.data[dateKey]; ok {
		if stats, exists := groups[groupId]; exists {
			return stats.Hourly
		}
	}
	return [24]int64{}
}

func (s *staticsStore) getWordFreq(dateKey string, groupId int64) map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]int64)
	if groups, ok := s.data[dateKey]; ok {
		if stats, exists := groups[groupId]; exists && stats.Words != nil {
			for word, count := range stats.Words {
				result[word] = count
			}
		}
	}
	return result
}

func (s *staticsStore) ensureGroupStats(dateKey string, groupId int64) *groupStats {
	groups, ok := s.data[dateKey]
	if !ok {
		groups = make(map[int64]*groupStats)
		s.data[dateKey] = groups
	}
	stats, ok := groups[groupId]
	if !ok {
		stats = &groupStats{Words: make(map[string]int64)}
		groups[groupId] = stats
	}
	return stats
}

func newStaticsStore(dir string, retention int) (*staticsStore, error) {
	if dir == "" {
		dir = defaultDataDir
	}
	if retention < 2 {
		retention = 2
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	store := &staticsStore{
		root:       dir,
		retention:  retention,
		data:       make(map[string]map[int64]*groupStats),
		dirtyDates: make(map[string]bool),
	}
	if err := store.loadRecent(); err != nil {
		return nil, err
	}
	store.startAutoFlush()
	return store, nil
}

func (s *staticsStore) loadRecent() error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -s.retention)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		dateKey := strings.TrimSuffix(name, ".json")
		parsed, err := time.Parse(staticsDateLayout, dateKey)
		if err != nil || parsed.Before(cutoff) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, name))
		if err != nil {
			log.Printf("statics load read error: %v", err)
			continue
		}
		var payload map[string]*groupStats
		if err := json.Unmarshal(data, &payload); err != nil {
			log.Printf("statics load unmarshal %s error: %v", name, err)
			continue
		}
		groups := make(map[int64]*groupStats)
		for gidStr, stats := range payload {
			gid, err := strconv.ParseInt(gidStr, 10, 64)
			if err != nil {
				continue
			}
			if stats != nil && stats.Words == nil {
				stats.Words = make(map[string]int64)
			}
			groups[gid] = stats
		}
		s.data[dateKey] = groups
	}
	return nil
}

func (s *staticsStore) startAutoFlush() {
	s.flushOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				s.flushDirtyDates()
			}
		}()
	})
}

func (s *staticsStore) flushDirtyDates() {
	dates := s.snapshotDirtyDates()
	for _, date := range dates {
		if err := s.flushDate(date); err != nil {
			log.Printf("statics flush %s error: %v", date, err)
		}
	}
	s.cleanupExpired()
}

func (s *staticsStore) snapshotDirtyDates() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.dirtyDates) == 0 {
		return nil
	}
	res := make([]string, 0, len(s.dirtyDates))
	for date := range s.dirtyDates {
		res = append(res, date)
	}
	return res
}

func (s *staticsStore) flushDate(date string) error {
	s.mu.RLock()
	groups := s.data[date]
	if len(groups) == 0 {
		s.mu.RUnlock()
		s.mu.Lock()
		delete(s.dirtyDates, date)
		s.mu.Unlock()
		return nil
	}
	snapshot := make(map[int64]*groupStats, len(groups))
	for gid, stats := range groups {
		if stats == nil {
			continue
		}
		snapshot[gid] = stats.clone()
	}
	s.mu.RUnlock()
	if len(snapshot) == 0 {
		s.mu.Lock()
		delete(s.dirtyDates, date)
		s.mu.Unlock()
		return nil
	}
	serializable := make(map[string]*groupStats, len(snapshot))
	for gid, stats := range snapshot {
		serializable[strconv.FormatInt(gid, 10)] = stats
	}
	payload, err := json.MarshalIndent(serializable, "", "  ")
	if err != nil {
		return err
	}
	finalPath := filepath.Join(s.root, fmt.Sprintf("%s.json", date))
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.dirtyDates, date)
	s.mu.Unlock()
	return nil
}

func (s *staticsStore) cleanupExpired() {
	cutoff := time.Now().AddDate(0, 0, -s.retention)
	s.mu.Lock()
	for date := range s.data {
		parsed, err := time.Parse(staticsDateLayout, date)
		if err != nil || parsed.Before(cutoff) {
			delete(s.data, date)
			delete(s.dirtyDates, date)
			_ = os.Remove(filepath.Join(s.root, fmt.Sprintf("%s.json", date)))
		}
	}
	s.mu.Unlock()
}

func (g *groupStats) clone() *groupStats {
	if g == nil {
		return &groupStats{Words: make(map[string]int64)}
	}
	clone := &groupStats{
		Hourly: g.Hourly,
		Words:  make(map[string]int64, len(g.Words)),
	}
	for word, count := range g.Words {
		clone.Words[word] = count
	}
	return clone
}

func resolveDateKey(label string) string {
	now := time.Now()
	if isYesterdayLabel(label) {
		return now.AddDate(0, 0, -1).Format(staticsDateLayout)
	}
	return now.Format(staticsDateLayout)
}

func describeDay(label string) string {
	if isYesterdayLabel(label) {
		return "昨日"
	}
	return "今日"
}

func isYesterdayLabel(label string) bool {
	lower := strings.ToLower(strings.TrimSpace(label))
	if lower == "yesterday" || lower == "y" || lower == "-1" || strings.Contains(lower, "yesterday") {
		return true
	}
	return strings.Contains(label, "昨")
}

func normalizeWord(word string) string {
	word = strings.TrimSpace(strings.ToLower(word))
	if word == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range word {
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) {
			builder.WriteRune(r)
		}
	}
	res := builder.String()
	if res == "" {
		return ""
	}
	runes := []rune(res)
	if len(runes) == 1 && !unicode.Is(unicode.Han, runes[0]) {
		return ""
	}
	if len(runes) > 12 {
		res = string(runes[:12])
	}
	return res
}

func buildStopWords(custom []string) map[string]struct{} {
	combined := append([]string{}, defaultStopWords...)
	combined = append(combined, custom...)
	set := make(map[string]struct{}, len(combined))
	for _, word := range combined {
		if norm := normalizeWord(word); norm != "" {
			set[norm] = struct{}{}
		}
	}
	return set
}

func buildImageArray(img []byte) []cqcode.ArrayMessage {
	encoded := "base64://" + base64.StdEncoding.EncodeToString(img)
	return []cqcode.ArrayMessage{*cqcode.Image(encoded)}
}
