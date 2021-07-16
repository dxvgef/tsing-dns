package service

import (
	"strings"

	"local/global"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type GeneralHandler struct {
}

func (gh GeneralHandler) ServeDNS(resp dns.ResponseWriter, reqMsg *dns.Msg) {
	var (
		err     error
		respMsg = new(dns.Msg)
	)
	defer func() {
		if resp != nil {
			if err = resp.Close(); err != nil {
				log.Err(err).Caller().Send()
			}
		}
	}()

	// 查询内部域的记录
	if strings.HasSuffix(reqMsg.Question[0].Name, ".uam.") {
		respMsg, err = queryStorage(reqMsg)
		if err != nil {
			log.Err(err).Caller().Msg("解析内部域名失败")
			// return
		}
	} else if global.Config.Service.Upstream.Count > 0 {
		// 查询上游服务
		upstream := Upstream{
			ReqMsg: reqMsg,
		}
		respMsg, err = upstream.Query()
		if err != nil {
			log.Err(err).Caller().Msg("查询上游服务失败")
			// return
		}
	}

	if err != nil {
		respMsg = &dns.Msg{}
		respMsg.SetReply(reqMsg)
		respMsg.Rcode = dns.RcodeServerFailure
	}

	// 发送响应消息
	err = resp.WriteMsg(respMsg)
	if err != nil {
		log.Err(err).Caller().Msg("响应消息失败")
	}
}
