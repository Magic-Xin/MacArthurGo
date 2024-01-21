package base

import (
	"encoding/json"
	"github.com/tidwall/pretty"
	"log"
	"os"
	"sync"
	"time"
)

var Config config

type config struct {
	Mutex      sync.RWMutex `json:"-"`
	ConfigPath string       `json:"-"`
	StartTime  int64        `json:"-"`

	Debug          bool    `json:"debug"`
	Address        string  `json:"address"`
	AuthToken      string  `json:"authToken"`
	RetryTimes     int64   `json:"retryTimes"`
	WaitingSeconds int64   `json:"waitingSeconds"`
	Admin          int64   `json:"admin"`
	UpdateUrl      string  `json:"updateUrl"`
	UpdateInterval int64   `json:"updateInterval"`
	BannedList     []int64 `json:"bannedList"`
	Plugins        struct {
		Corpus struct {
			Enable bool `json:"enable"`
		} `json:"corpus"`
		Repeat struct {
			Enable            bool    `json:"enable"`
			Times             int64   `json:"times"`
			Probability       float64 `json:"probability"`
			CommonProbability float64 `json:"commonProbability"`
		} `json:"repeat"`
		Bili struct {
			Enable      bool `json:"enable"`
			AiSummarize struct {
				Enable       bool     `json:"enable"`
				Args         []string `json:"args"`
				GroupForward bool     `json:"groupForward"`
			} `json:"ai_summarize"`
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
			SearchFeedback    string   `json:"searchFeedback"`
			SauceNAOToken     string   `json:"sauceNAOToken"`
		} `json:"picSearch"`
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
				Enable bool     `json:"enable"`
				Args   []string `json:"args"`
				APIKey string   `json:"apiKey"`
			} `json:"gemini"`
			NewBing struct {
				Enable bool     `json:"enable"`
				Args   []string `json:"args"`
				Model  string   `json:"model"`
				APIUrl string   `json:"apiUrl"`
				APIKey string   `json:"apiKey"`
			} `json:"newBing"`
			GroupForward bool `json:"groupForward"`
			Pangu        bool `json:"pangu"`
		} `json:"chatAI"`
	} `json:"plugins"`
}

func init() {
	var configPath string
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	} else {
		configPath = "config.json"
	}

	f, err := os.Open(configPath)
	if err != nil {
		log.Printf("Open config failed: %v", err)
		panic(nil)
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Printf("Close config failed: %v", err)
		}
	}(f)

	err = json.NewDecoder(f).Decode(&Config)
	if err != nil {
		log.Printf("Decode config failed: %v", err)
		panic(nil)
	}

	Config.ConfigPath = configPath
	Config.StartTime = time.Now().Unix()

	log.Printf("Config \"%s\" loaded! Initializing...", configPath)
}

func (c *config) UpdateConfig() {
	f, err := os.OpenFile(c.ConfigPath, os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("Open config failed: %v", err)
		return
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Printf("Close config failed: %v", err)
		}
	}(f)

	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	conf, err := json.Marshal(c)
	if err != nil {
		log.Printf("Marshal config error: %v", err)
		return
	}
	conf = pretty.Pretty(conf)
	_, err = f.Write(conf)
	if err != nil {
		log.Printf("Write config error: %v", err)
		return
	}
	log.Println("Config updated!")
}
