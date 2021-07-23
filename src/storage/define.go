package storage

import (
	"local/global"
	"local/storage/redis"
	"local/storage/voltdb"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// 存储器实例
var Storage Interface

// 存储器接口
type Interface interface {
	Set(rr dns.RR) (err error)
	Get(question dns.Question) (result []dns.RR, err error)
	Del(rr dns.RR) (err error)
}

// 构建存储器实例
func MakeStorage() (err error) {
	// 初始化存储器
	switch global.Config.Storage.Type {
	case "redis":
		Storage, err = redis.NewWithJSON(global.Config.Storage.Config)
		if err != nil {
			log.Fatal().Err(err).Caller().Msg("构建 Redis 存储器失败")
			break
		}
		log.Info().Msg("使用 Redis 存储器")
	case "voltdb":
		Storage, err = voltdb.NewWithJSON(global.Config.Storage.Config)
		if err != nil {
			log.Fatal().Err(err).Caller().Msg("构建 VoltDB 存储器失败")
			break
		}
		log.Info().Msg("使用 VoltDB 存储器")
	default:
		log.Fatal().Caller().Str("type", global.Config.Storage.Type).Msg("不支持的存储器类型")
	}

	return
}
