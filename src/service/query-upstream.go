package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"local/global"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type RR struct {
	Name string `json:"name"`
	Type uint16 `json:"type"`
	TTL  uint32 `json:"TTL"`
	Data string `json:"data"`
}

type JSONResult struct {
	Status    uint16 `json:"Status"`
	TC        bool   `json:"TC"`
	RD        bool   `json:"RD"`
	RA        bool   `json:"RA"`
	AD        bool   `json:"AD"`
	CD        bool   `json:"CD"`
	Question  []RR   `json:"Question"`
	Answer    []RR   `json:"Answer,omitempty"`
	Authority []RR   `json:"Authority,omitempty"`
	Extra     []RR   `json:"Extra,omitempty"`
}

type Upstream struct {
	MethodByDoT string
	ReqMsg      *dns.Msg
}

// 遍历上游进行查询
func (upstream *Upstream) Query() (respMsg *dns.Msg, err error) {
	var abort bool

	// 遍历查询上游服务
	for k := range global.Config.Service.Upstream.Addrs {
		if abort {
			break
		}
		switch {
		case strings.HasPrefix(global.Config.Service.Upstream.Addrs[k], "udp://"):
			upstreamAddr := strings.TrimPrefix(global.Config.Service.Upstream.Addrs[k], "udp://")
			respMsg, err = upstream.QueryDNS("udp", upstreamAddr)
			if err != nil {
				log.Err(err).Caller().Str("addr", global.Config.Service.Upstream.Addrs[k]).Msg("向上游UDP服务查询失败")
				break
			}
			abort = true
		case strings.HasPrefix(global.Config.Service.Upstream.Addrs[k], "tcp://"):
			upstreamAddr := strings.TrimPrefix(global.Config.Service.Upstream.Addrs[k], "tcp://")
			respMsg, err = upstream.QueryDNS("tcp", upstreamAddr)
			if err != nil {
				log.Err(err).Caller().Str("addr", global.Config.Service.Upstream.Addrs[k]).Msg("向上游TCP服务查询失败")
				break
			}
			abort = true
		case strings.HasPrefix(global.Config.Service.Upstream.Addrs[k], "tls://"):
			upstreamAddr := strings.TrimPrefix(global.Config.Service.Upstream.Addrs[k], "tls://")
			respMsg, err = upstream.QueryDNS("tcp-tls", upstreamAddr)
			if err != nil {
				log.Err(err).Caller().Str("addr", global.Config.Service.Upstream.Addrs[k]).Msg("向上游DoT服务查询失败")
				break
			}
			abort = true
		case strings.HasPrefix(global.Config.Service.Upstream.Addrs[k], "https://"):
			if upstream.MethodByDoT == http.MethodGet {
				respMsg, err = upstream.QueryByGET(global.Config.Service.Upstream.Addrs[k], global.Config.Service.Upstream.HTTPProxy)
			} else {
				respMsg, err = upstream.QueryByPOST(global.Config.Service.Upstream.Addrs[k], global.Config.Service.Upstream.HTTPProxy)
			}
			if err != nil {
				log.Err(err).Caller().Str("addr", global.Config.Service.Upstream.Addrs[k]).Msg("向上游DoH服务查询失败")
				break
			}
			abort = true
		default:
			err = errors.New("不支持的上游服务协议 " + global.Config.Service.Upstream.Addrs[k])
		}
	}
	return
}

// 查询上游的DNS over UDP/TCP服务
func (upstream *Upstream) QueryDNS(netType string, addr string) (respMsg *dns.Msg, err error) {
	var (
		dnsReq = dns.Client{
			Timeout: 5 * time.Second,
			Net:     netType,
		}
	)

	if netType == "tcp-tls" {
		dnsReq.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 向上游发起请求
	respMsg, _, err = dnsReq.ExchangeContext(ctx, upstream.ReqMsg, addr)
	if err != nil {
		log.Err(err).Caller().Str("net", netType).Str("addr", addr).Msg("请求上游DNS服务失败")
		return
	}
	return
}

// DNS over TLS
func (upstream *Upstream) QueryTLS(addr string, tlsConfig *tls.Config) (respMsg *dns.Msg, err error) {
	dnsReq := dns.Client{
		Timeout: 5 * time.Second,
		Net:     "tcp-tls",
	}

	if tlsConfig == nil {
		dnsReq.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	} else {
		dnsReq.TLSConfig = tlsConfig
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 向上游发起请求
	respMsg, _, err = dnsReq.ExchangeContext(ctx, upstream.ReqMsg, addr)
	if err != nil {
		log.Err(err).Caller().Str("addr", addr).Msg("请求上游DoT服务失败")
		return
	}
	return
}

// DNS over HTTPS GET /dns-query
func (upstream *Upstream) QueryByGET(addr string, proxy string) (respMsg *dns.Msg, err error) {
	var (
		httpClient http.Client
		httpReq    *http.Request
		httpResp   *http.Response
		dnsParam   string
		reqMsgBuf  []byte
		respMsgBuf []byte
		proxyURL   *url.URL
	)
	respMsg = new(dns.Msg)

	// 设置代理
	if proxy != "" {
		proxyURL, err = url.Parse(proxy)
		if err != nil {
			log.Err(err).Caller().Msg("无效的HTTP代理地址")
			return
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 将请求消息体转为[]byte
	reqMsgBuf, err = upstream.ReqMsg.Pack()
	if err != nil {
		log.Err(err).Caller().Msg("构建请求消息失败")
		return
	}
	dnsParam = base64.RawURLEncoding.EncodeToString(reqMsgBuf)

	httpReq, err = http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		log.Err(err).Caller().Str("url", addr).Msg("请求DoT服务失败")
		return
	}
	values := make(url.Values, 1)
	values.Set("dns", dnsParam)
	httpReq.URL.RawQuery = values.Encode()
	httpResp, err = httpClient.Do(httpReq)
	if err != nil {
		log.Err(err).Caller().Str("name", upstream.ReqMsg.Question[0].Name).Str("type", dns.TypeToString[upstream.ReqMsg.Question[0].Qtype]).Str("addr", addr).Msg("请求上游DoT服务失败")
		return
	}
	defer func() {
		if err = httpResp.Body.Close(); err != nil {
			log.Err(err).Caller().Msg("关闭Body失败")
		}
	}()
	if httpResp.StatusCode != 200 {
		log.Err(err).Caller().Int("statusCode", httpResp.StatusCode).Str("respStatus", httpResp.Status).Str("addr", addr).Msg("收到错误响应")
		err = errors.New("收到错误响应：" + httpResp.Status)
		return
	}
	respMsgBuf, err = io.ReadAll(httpResp.Body)
	if err != nil {
		log.Err(err).Caller().Msg("读取DoT服务响应数据失败")
		return
	}
	err = respMsg.Unpack(respMsgBuf)
	return
}

// DNS over HTTPS POST /dns-query
func (upstream *Upstream) QueryByPOST(addr string, proxy string) (respMsg *dns.Msg, err error) {
	var (
		httpClient http.Client
		httpReq    *http.Request
		httpResp   *http.Response
		reqBody    []byte
		respBody   []byte
		proxyURL   *url.URL
	)
	respMsg = new(dns.Msg)

	// 设置代理
	if proxy != "" {
		proxyURL, err = url.Parse(proxy)
		if err != nil {
			log.Err(err).Caller().Msg("无效的HTTP代理地址")
			return
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 将请求消息体转为[]byte
	reqBody, err = upstream.ReqMsg.Pack()
	if err != nil {
		log.Err(err).Caller().Msg("构建请求消息失败")
		return
	}

	httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Err(err).Caller().Str("url", addr).Msg("请求DoT服务失败")
		return
	}
	httpReq.Header.Set("Content-Type", "application/dns-message")
	httpReq.Header.Set("accept", "application/dns-message")
	httpResp, err = httpClient.Do(httpReq)
	if err != nil {
		log.Err(err).Caller().Str("name", upstream.ReqMsg.Question[0].Name).Str("type", dns.TypeToString[upstream.ReqMsg.Question[0].Qtype]).Str("addr", addr).Msg("请求上游服务失败")
		return
	}
	defer func() {
		if err = httpResp.Body.Close(); err != nil {
			log.Err(err).Caller().Msg("关闭Body失败")
		}
	}()
	if httpResp.StatusCode != 200 {
		log.Err(err).Caller().Int("statusCode", httpResp.StatusCode).Str("respStatus", httpResp.Status).Str("addr", addr).Msg("收到错误响应")
		err = errors.New("收到错误响应：" + httpResp.Status)
		return
	}
	respBody, err = io.ReadAll(httpResp.Body)
	if err != nil {
		log.Err(err).Caller().Msg("读取DoT服务响应数据失败")
		return
	}
	err = respMsg.Unpack(respBody)
	return
}
