package base

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/tidwall/pretty"
)

var Config config

type config struct {
	Mutex      sync.RWMutex `json:"-"`
	ConfigPath string       `json:"-"`
	StartTime  int64        `json:"-"`

	Debug          bool    `json:"debug"`
	Address        string  `json:"address"`
	Port           int64   `json:"port"`
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
