package service

import (
	"local/storage"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// 查询存储器
func queryStorage(reqMsg *dns.Msg) (respMsg *dns.Msg, err error) {
	var rr []dns.RR
	respMsg = new(dns.Msg)
	respMsg.SetReply(reqMsg)
	// 从存储器获取记录
	rr, err = storage.Storage.Get(reqMsg.Question[0])
	if err != nil {
		log.Err(err).Caller().Msg("查询内部存储器")
		return
	}
	if rr == nil {
		respMsg.Rcode = dns.RcodeNameError
		return
	}
	respMsg.Answer = rr
	return
}
