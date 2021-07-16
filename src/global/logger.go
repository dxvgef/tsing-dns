package global

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// 使用默认参数设置logger，用于没有读取配置时临时替代标准包的log使用
func UseDefaultLogger() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	zerolog.TimeFieldFormat = FormatTime("y-m-d h:i:s")

	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: zerolog.TimeFieldFormat,
	}

	log.Logger = log.Output(output)
}

// 格式化时间字符串
func FormatTime(str string) string {
	str = strings.ReplaceAll(str, "y", "2006")
	str = strings.ReplaceAll(str, "m", "01")
	str = strings.ReplaceAll(str, "d", "02")
	str = strings.ReplaceAll(str, "h", "15")
	str = strings.ReplaceAll(str, "i", "04")
	str = strings.ReplaceAll(str, "s", "05")
	return str
}

// 设置logger
func SetupLogger() (err error) {
	var (
		output  io.Writer
		logFile *os.File
	)

	// 设置级别
	Config.Logger.Level = strings.ToLower(Config.Logger.Level)
	// 如果是debug模式，则日志记录自动为debug级别
	if Config.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		switch Config.Logger.Level {
		case "":
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		case "info":
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		case "warn":
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		default:
			err = errors.New("level参数值无效")
			log.Err(err).Str("level", Config.Logger.Level).Msg("配置Logger失败")
			return
		}
	}

	// 设置时间格式
	Config.Logger.TimeFormat = strings.ToLower(Config.Logger.TimeFormat)
	if Config.Logger.TimeFormat == "timestamp" {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	} else {
		zerolog.TimeFieldFormat = FormatTime(Config.Logger.TimeFormat)
	}

	// 设置日志输出方式
	// 输出到日志文件，否则默认是输出到控制台
	if Config.Logger.Output != "" {
		// 打开文件
		logFile, err = os.OpenFile(Config.Logger.Output, os.O_RDWR|os.O_CREATE|os.O_APPEND, Config.Logger.FileMode)
		if nil != err {
			log.Err(err).Caller().Msg("配置Logger失败")
			return
		}
	}

	// 设置日志编码格式
	Config.Logger.Encode = strings.ToLower(Config.Logger.Encode)
	switch Config.Logger.Encode {
	case "":
	case "console":
		if logFile != nil {
			output = zerolog.ConsoleWriter{
				Out:        logFile,
				NoColor:    true,
				TimeFormat: zerolog.TimeFieldFormat,
			}
		} else {
			output = zerolog.ConsoleWriter{
				Out:        os.Stdout,
				NoColor:    Config.Logger.NoColor,
				TimeFormat: zerolog.TimeFieldFormat,
			}
		}
	// json编码
	case "json":
		if logFile != nil {
			output = logFile
		} else {
			output = os.Stdout
		}
	default:
		err = errors.New("encode参数值只支持json和console")
		log.Fatal().Err(err).Caller().Msg("配置Logger失败")
		return
	}

	log.Logger = log.Output(output)

	return
}
