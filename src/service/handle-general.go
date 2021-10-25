package service

import (
	"strings"

	"local/global"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type GeneralHandler struct{}

func (handler GeneralHandler) ServeDNS(resp dns.ResponseWriter, reqMsg *dns.Msg) {
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

	if !strings.HasSuffix(reqMsg.Question[0].Name, ".") {
		reqMsg.Question[0].Name += "."
	}

	// 查询内部域的记录
	if global.IsInternal(reqMsg.Question[0].Name) {
		respMsg, err = queryStorage(reqMsg)
		if err != nil {
			log.Err(err).Caller().Msg("解析内部域名失败")
		}
	} else if global.Config.Service.Upstream.Count > 0 {
		// 查询上游服务
		upstream := Upstream{
			ReqMsg: reqMsg,
		}
		respMsg, err = upstream.Query()
		if err != nil {
			log.Err(err).Caller().Msg("查询上游服务失败")
		}
	}

	if err != nil {
		respMsg = &dns.Msg{}
		respMsg.SetReply(reqMsg)
		respMsg.Rcode = dns.RcodeServerFailure
	}

	if len(respMsg.Answer) == 0 {
		respMsg.SetReply(reqMsg)
		respMsg.Rcode = dns.RcodeNameError
	}

	// 防止UDP客户端无法接收超过512字节的数据，清空ns(AUTHORITY SECTION)和extra(ADDITIONAL SECTION)节点
	if resp.LocalAddr().Network() == "udp" {
		log.Debug().Msg("udp协议")
		respMsg.Extra = nil
		respMsg.Ns = nil
	}

	// 发送响应消息
	err = resp.WriteMsg(respMsg)
	if err != nil {
		log.Err(err).Caller().Msg("响应消息失败")
	}
}
