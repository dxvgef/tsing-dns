package global

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
	"github.com/rs/zerolog/log"
)

// 启动参数
var LaunchFlag struct {
	ConfigSource string // 配置来源(file或者服务中心地址'127.0.0.1:10000')
	Env          string // 环境变量
}

// 运行时配置
var Config struct {
	Debug bool `toml:"-"`

	Service struct {
		Upstream struct {
			Count     int      `toml:"-"`
			Addrs     []string `toml:"addrs"`
			HTTPProxy string   `toml:"httpProxy"`
		} `toml:"upstream"`
		HTTPS struct {
			Port     uint16 `toml:"port"`
			CertFile string `toml:"certFile"`
			KeyFile  string `toml:"keyFile"`
		} `toml:"https"`
		TLS struct {
			Port     uint16 `toml:"port"`
			CertFile string `toml:"certFile"`
			KeyFile  string `toml:"keyFile"`
		} `toml:"tls"`
		InternalSuffix  []string `toml:"internalSuffix"`
		IP              string   `toml:"ip"`
		QuitWaitTimeout uint     `toml:"quitWaitTimeout"`
		HTTP            struct {
			Port uint16 `toml:"port"`
		} `toml:"http"`
		UDP struct {
			Port uint16 `toml:"port"`
		} `toml:"udp"`
		TCP struct {
			Port uint16 `toml:"port"`
		} `toml:"tcp"`
	} `toml:"service"`
	Storage struct {
		Type   string `toml:"type"`
		Config string `toml:"config"`
	} `toml:"storage"`
	Logger struct {
		Level      string      `toml:"level"`
		Output     string      `toml:"output"`
		Encode     string      `toml:"encode"`
		TimeFormat string      `toml:"timeFormat"`
		FileMode   os.FileMode `toml:"fileMode"`
		NoColor    bool        `toml:"noColor"`
	} `toml:"logger"`
}

// 设置本地默认配置
func defaultConfig() {
	// 环境变量
	LaunchFlag.ConfigSource = "file"

	Config.Debug = true
	Config.Service.QuitWaitTimeout = 5

	Config.Service.UDP.Port = 53
	Config.Service.TCP.Port = 53
	Config.Service.HTTP.Port = 80

	Config.Logger.Level = "debug"
	Config.Logger.FileMode = 0600
	Config.Logger.Encode = "console"
	Config.Logger.TimeFormat = "y-m-d h:i:s"
}

// 加载配置
func LoadConfig() (err error) {
	// 加载本地默认配置
	defaultConfig()

	// 解析启动参数
	// flag.StringVar(&LaunchFlag.ConfigSource, "cfg", LaunchFlag.ConfigSource, "配置来源，可以是file表示本地配置文件或者配置中心地址ip:port，默认file")
	flag.StringVar(&LaunchFlag.Env, "env", LaunchFlag.Env, "环境变量，默认为空")
	flag.Parse()

	LaunchFlag.Env = strings.ToLower(LaunchFlag.Env)

	log.Info().Str("配置来源(cfg)", LaunchFlag.ConfigSource).Str("环境变量(env)", LaunchFlag.Env).Msg("启动参数")

	// 加载本地配置文件
	if LaunchFlag.ConfigSource == "file" {
		// 加载本地配置文件
		if err = loadConfigFile(); err != nil {
			log.Err(err).Caller().Msg("加载本地配置文件失败")
			return
		}
	}
	Config.Service.Upstream.Count = len(Config.Service.Upstream.Addrs)
	if Config.Service.Upstream.Count < 1 {
		log.Warn().Msg("注意：没有从配置中获取至少一个有效的上游DNS服务地址，DNS转发服务无法工作")
	} else {
		log.Info().Int("Upstream Count", Config.Service.Upstream.Count).Msg("启用DNS转发服务")
	}
	if Config.Service.TLS.Port > 0 {
		if Config.Service.TLS.CertFile == "" {
			err = errors.New("启用DNS over TLS服务时，certFile参数值不能为空")
			log.Err(err).Caller().Msg("解析配置失败")
			return
		}
		if Config.Service.TLS.KeyFile == "" {
			err = errors.New("启用DNS over TLS服务时，keyFile参数值不能为空")
			log.Err(err).Caller().Msg("解析配置失败")
			return
		}
	}
	if Config.Service.HTTPS.Port > 0 {
		if Config.Service.HTTPS.CertFile == "" {
			err = errors.New("启用DNS over HTTPS服务时，certFile参数值不能为空")
			log.Err(err).Caller().Msg("解析配置失败")
			return
		}
		if Config.Service.HTTPS.KeyFile == "" {
			err = errors.New("启用DNS over HTTPS服务时，keyFile参数值不能为空")
			log.Err(err).Caller().Msg("解析配置失败")
			return
		}
	}
	return
}

// 加载本地配置文件
func loadConfigFile() (err error) {
	var (
		filePath string
		file     *os.File
	)
	if LaunchFlag.Env == "" {
		filePath = "./config.toml"
	} else {
		filePath = "./config." + LaunchFlag.Env + ".toml"
	}
	file, err = os.Open(filepath.Clean(filePath))
	if err != nil {
		log.Err(err).Caller().Str("path", filePath).Send()
		return
	}

	// 解析配置文件到Config
	err = toml.NewDecoder(file).Decode(&Config)
	if err != nil {
		log.Err(err).Caller().Msg("加载本地配置文件失败")
		return
	}
	log.Info().Str("路径", filePath).Msg("加载本地配置文件")
	return
}
