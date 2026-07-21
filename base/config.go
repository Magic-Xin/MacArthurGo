package base

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/pretty"
)

var Config = &Configuration{}

const defaultSoutuBotSimilarityThreshold = 45.0

// Configuration contains all runtime settings loaded from config.json.
// Runtime-only fields are excluded from JSON serialization.
type Configuration struct {
	Mutex      sync.RWMutex `json:"-"`
	ConfigPath string       `json:"-"`
	StartTime  int64        `json:"-"`

	Debug          bool    `json:"debug"`
	Address        string  `json:"address"`
	AuthToken      string  `json:"authToken"`
	Admin          int64   `json:"admin"`
	UpdateUrl      string  `json:"updateUrl"`
	UpdateInterval int64   `json:"updateInterval"`
	BannedList     []int64 `json:"bannedList"`
	Plugins        struct {
		Corpus struct {
			Enable bool `json:"enable"`
			Rules  []struct {
				Regexp  string  `json:"regexp"`
				Reply   string  `json:"reply"`
				IsReply bool    `json:"isReply"`
				IsAt    bool    `json:"isAt"`
				Scene   string  `json:"scene"`
				Users   []int64 `json:"users"`
				Groups  []int64 `json:"groups"`
			} `json:"rules"`
		} `json:"corpus"`
		OriginPic struct {
			Enable bool     `json:"enable"`
			Args   []string `json:"args"`
		} `json:"originPic"`
		Repeat struct {
			Enable            bool    `json:"enable"`
			Times             int64   `json:"times"`
			Probability       float64 `json:"probability"`
			CommonProbability float64 `json:"commonProbability"`
		} `json:"repeat"`
		Bili struct {
			Enable      bool `json:"enable"`
			AiSummarize struct {
				Enable       bool `json:"enable"`
				GroupForward bool `json:"groupForward"`
			} `json:"aiSummarize"`
		} `json:"bili"`
		Poke struct {
			Enable bool     `json:"enable"`
			Args   []string `json:"args"`
		} `json:"poke"`
		Roll struct {
			Enable bool     `json:"enable"`
			Args   []string `json:"args"`
		} `json:"roll"`
		Music struct {
			Enable bool `json:"enable"`
		} `json:"music"`
		PicSearch struct {
			Enable            bool     `json:"enable"`
			Args              []string `json:"args"`
			GroupForward      bool     `json:"groupForward"`
			AllowPrivate      bool     `json:"allowPrivate"`
			HandleBannedHosts bool     `json:"handleBannedHosts"`
			ExpirationTime    int64    `json:"expirationTime"`
			IntervalTime      int64    `json:"intervalTime"`
			SauceNAOToken     string   `json:"sauceNAOToken"`
			ASCII2D           struct {
				CloudflareBypassURL string `json:"cloudflareBypassUrl"`
				FlareSolverrURL     string `json:"flareSolverrUrl"`
				ProxyURL            string `json:"proxyUrl"`
				TimeoutSeconds      int    `json:"timeoutSeconds"`
			} `json:"ascii2d"`
			SoutuBot struct {
				CloudflareBypassURL string  `json:"cloudflareBypassUrl"`
				ProxyURL            string  `json:"proxyUrl"`
				TimeoutSeconds      int     `json:"timeoutSeconds"`
				SimilarityThreshold float64 `json:"similarityThreshold"`
			} `json:"soutuBot"`
			GoogleLens struct {
				APIKey         string `json:"apiKey"`
				TimeoutSeconds int    `json:"timeoutSeconds"`
			} `json:"googleLens"`
		} `json:"picSearch"`
		Statics struct {
			Enable           bool              `json:"enable"`
			ChartArgsMap     map[string]string `json:"chartArgsMap"`
			WordCloudArgsMap map[string]string `json:"wordCloudArgsMap"`
			StopWords        []string          `json:"stopWords"`
			RetentionDays    int               `json:"retentionDays"`
			DataDir          string            `json:"dataDir"`
			Chart            struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"chart"`
			WordCloud struct {
				Width    int    `json:"width"`
				Height   int    `json:"height"`
				MaxWords int    `json:"maxWords"`
				FontFile string `json:"fontFile"`
			} `json:"wordCloud"`
		} `json:"statics"`
		ChatAI struct {
			Enable  bool `json:"enable"`
			ChatGPT struct {
				Enable bool     `json:"enable"`
				Args   []string `json:"args"`
				Model  string   `json:"model"`
				APIKey string   `json:"apiKey"`
			} `json:"chatGPT"`
			QWen struct {
				Enable bool     `json:"enable"`
				Args   []string `json:"args"`
				Model  string   `json:"model"`
				APIKey string   `json:"apiKey"`
			} `json:"qWen"`
			Gemini struct {
				Enable  bool              `json:"enable"`
				ArgsMap map[string]string `json:"argsMap"`
				APIKey  string            `json:"apiKey"`
			} `json:"gemini"`
			Github struct {
				Enable  bool              `json:"enable"`
				ArgsMap map[string]string `json:"argsMap"`
				Token   string            `json:"token"`
			} `json:"github"`
			GroupForward bool `json:"groupForward"`
			PanGu        bool `json:"panGu"`
		} `json:"chatAI"`
		Waifu struct {
			Enable bool     `json:"enable"`
			Args   []string `json:"args"`
		} `json:"waifu"`
	} `json:"plugins"`
}

// ConfigPath returns the configured path from command-line arguments.
func ConfigPath(args []string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return args[0]
	}
	return "config.json"
}

// LoadConfig explicitly loads and validates configuration before plugins are
// registered. Keeping this out of init makes startup order deterministic and
// allows packages to be tested without a local config.json.
func LoadConfig(configPath string) error {
	f, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("open config %q: %w", configPath, err)
	}
	defer f.Close()

	var loaded Configuration
	loaded.Plugins.PicSearch.SoutuBot.SimilarityThreshold = defaultSoutuBotSimilarityThreshold
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&loaded); err != nil {
		return fmt.Errorf("decode config %q: %w", configPath, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode config %q: %w", configPath, err)
	}
	if err := loaded.Validate(); err != nil {
		return fmt.Errorf("validate config %q: %w", configPath, err)
	}

	loaded.ConfigPath = configPath
	loaded.StartTime = time.Now().Unix()
	Config = &loaded

	log.Printf("Config %q loaded", configPath)
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values are not allowed")
}

// Validate catches invalid settings before background workers start.
func (c *Configuration) Validate() error {
	address := strings.TrimSpace(c.Address)
	if address == "" {
		return errors.New("address is required")
	}
	parsed, err := url.Parse(address)
	if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.Host == "" {
		return fmt.Errorf("address must be a valid ws:// or wss:// URL")
	}
	if c.UpdateInterval < 0 {
		return errors.New("updateInterval cannot be negative")
	}
	if p := c.Plugins.Repeat.Probability; p < 0 || p > 1 {
		return errors.New("plugins.repeat.probability must be between 0 and 1")
	}
	if p := c.Plugins.Repeat.CommonProbability; p < 0 || p > 1 {
		return errors.New("plugins.repeat.commonProbability must be between 0 and 1")
	}
	if c.Plugins.PicSearch.ASCII2D.TimeoutSeconds < 0 {
		return errors.New("plugins.picSearch.ascii2d.timeoutSeconds cannot be negative")
	}
	if c.Plugins.PicSearch.SoutuBot.TimeoutSeconds < 0 {
		return errors.New("plugins.picSearch.soutuBot.timeoutSeconds cannot be negative")
	}
	if threshold := c.Plugins.PicSearch.SoutuBot.SimilarityThreshold; threshold < 0 || threshold > 100 {
		return errors.New("plugins.picSearch.soutuBot.similarityThreshold must be between 0 and 100")
	}
	if c.Plugins.PicSearch.GoogleLens.TimeoutSeconds < 0 {
		return errors.New("plugins.picSearch.googleLens.timeoutSeconds cannot be negative")
	}
	return nil
}

// UpdateConfig persists the current configuration after serialization has
// succeeded, avoiding truncating a valid file when marshaling fails.
func (c *Configuration) UpdateConfig() error {
	c.Mutex.RLock()
	conf, err := json.Marshal(c)
	c.Mutex.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	conf = pretty.Pretty(conf)
	conf = append(conf, '\n')

	if err := os.WriteFile(c.ConfigPath, conf, 0600); err != nil {
		return fmt.Errorf("write config %q: %w", c.ConfigPath, err)
	}
	log.Println("Config updated!")
	return nil
}
