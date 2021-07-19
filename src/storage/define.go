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
	Set(rr dns.RR, ttl uint32) (err error)
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
			log.Fatal().Err(err).Caller().Msg("初始化存储器失败")
			break
		}
		log.Info().Str("type", "redis").Msg("使用Redis存储引擎")
	case "voltdb":
		Storage, err = voltdb.NewWithJSON(global.Config.Storage.Config)
		if err != nil {
			log.Fatal().Err(err).Caller().Msg("初始化存储器失败")
			break
		}
		log.Info().Str("type", "voltdb").Msg("使用VoltDB存储引擎")
	default:
		log.Fatal().Caller().Str("type", global.Config.Storage.Type).Msg("未知的存储器类型")
	}

	// writeTestData()
	return
}

// func writeTestData() {
// 	var (
// 		err error
// 		a   dns.A
// 		rr  dns.RR
// 	)
// 	a.Hdr.Ttl = 3600
// 	a.Hdr.Name = "test.uam"
// 	a.Hdr.Rrtype = dns.TypeA
// 	a.Hdr.Class = dns.ClassINET
// 	a.A = net.ParseIP("192.168.51.159")
// 	rr, err = dns.NewRR(a.String())
// 	if err != nil {
// 		log.Fatal().Caller().Err(err).Msg("生成测试数据失败")
// 	}
// 	err = sa.Set(rr, 3600)
// 	if err != nil {
// 		log.Fatal().Caller().Err(err).Msg("写入测试数据失败")
// 	}
// }
