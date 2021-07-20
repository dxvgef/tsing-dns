package service

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"local/global"
	"local/storage"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type HTTPHandler struct {
	resp http.ResponseWriter
	req  *http.Request
}

func (hh HTTPHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	hh.resp = resp
	hh.req = req

	switch req.URL.Path {
	case global.Config.Service.HTTP.DNSQueryPath:
		if global.Config.Service.HTTP.DNSQueryPath == "" {
			break
		}
		if req.Method == http.MethodGet {
			hh.dnsQueryByGET()
			break
		} else if req.Method == http.MethodPost {
			hh.dnsQueryByPOST()
			break
		}
		hh.respStatus(http.StatusMethodNotAllowed)
	case global.Config.Service.HTTP.JSONQueryPath:
		if global.Config.Service.HTTP.JSONQueryPath == "" {
			break
		}
		if req.Method != http.MethodGet {
			hh.respStatus(http.StatusMethodNotAllowed)
			break
		}
		hh.jsonQueryHandler()
	case global.Config.Service.HTTP.RegisterPath:
		if global.Config.Service.HTTP.RegisterPath == "" {
			break
		}
		if req.Method == http.MethodPost {
			hh.register(false)
			break
		}
		if req.Method == http.MethodPut {
			hh.register(true)
			break
		}
		hh.respStatus(http.StatusMethodNotAllowed)
	case global.Config.Service.HTTP.DeletePath:
		if global.Config.Service.HTTP.DeletePath == "" {
			break
		}
		if req.Method == http.MethodDelete {
			hh.delete()
			break
		}
		hh.respStatus(http.StatusMethodNotAllowed)
	}
	hh.respStatus(http.StatusNotFound)
}

func (hh *HTTPHandler) respStatus(status int) {
	hh.resp.WriteHeader(status)
	_, err := hh.resp.Write(global.StrToBytes(http.StatusText(status)))
	if err != nil {
		log.Warn().Err(err).Caller().Msg("响应数据时出错")
	}
}

// -> GET /dns-query -> dns.Msg
func (hh *HTTPHandler) dnsQueryByGET() {
	var (
		err        error
		param      string
		paramBytes []byte
		reqMsg     dns.Msg
		respMsg    *dns.Msg
		respData   []byte
	)

	if global.Config.Service.HTTP.DNSQueryAuth && hh.req.Header.Get("Authorization") != global.Config.Service.HTTP.Authorization {
		hh.respStatus(http.StatusUnauthorized)
		return
	}

	defer func() {
		if hh.req.Body != nil {
			err = hh.req.Body.Close()
			if err != nil {
				log.Warn().Err(err).Caller().Send()
			}
		}
	}()

	param = hh.req.URL.Query().Get("dns")
	paramBytes, err = base64.RawURLEncoding.DecodeString(param)
	if err != nil {
		log.Err(err).Caller().Msg("解析dns参数值失败")
		hh.respStatus(http.StatusBadRequest)
		return
	}
	err = reqMsg.Unpack(paramBytes)
	if err != nil {
		log.Err(err).Caller().Msg("解析dns参数值失败")
		hh.respStatus(http.StatusBadRequest)
		return
	}

	if reqMsg.Question[0].Name == "" || reqMsg.Question[0].Qtype == 0 {
		hh.respStatus(http.StatusBadRequest)
		return
	}
	if !strings.HasSuffix(reqMsg.Question[0].Name, ".") {
		reqMsg.Question[0].Name += "."
	}

	// 查询内部域的记录
	if global.IsInternal(reqMsg.Question[0].Name) {
		respMsg, err = queryStorage(&reqMsg)
		if err != nil {
			log.Err(err).Caller().Send()
			hh.respStatus(http.StatusInternalServerError)
			return
		}
	} else {
		// 查询上游服务
		upstream := Upstream{
			ReqMsg:      &reqMsg,
			MethodByDoT: hh.req.Method,
		}
		respMsg, err = upstream.Query()
		if err != nil {
			log.Err(err).Caller().Msg("查询上游服务失败")
			hh.respStatus(http.StatusInternalServerError)
			return
		}
	}

	respData, err = respMsg.Pack()
	if err != nil {
		log.Err(err).Caller().Msg("编码响应数据失败")
		hh.respStatus(http.StatusInternalServerError)
		return
	}

	_, err = hh.resp.Write(respData)
	if err != nil {
		log.Warn().Err(err).Caller().Msg("响应数据时出错")
	}
}

// -> POST /dns-query -> dns.Msg
func (hh *HTTPHandler) dnsQueryByPOST() {
	var (
		err      error
		body     []byte
		respData []byte
		reqMsg   dns.Msg
		respMsg  *dns.Msg
	)

	if global.Config.Service.HTTP.DNSQueryAuth && hh.req.Header.Get("Authorization") != global.Config.Service.HTTP.Authorization {
		hh.respStatus(http.StatusUnauthorized)
		return
	}

	defer func() {
		if hh.req.Body != nil {
			err = hh.req.Body.Close()
			if err != nil {
				log.Warn().Err(err).Caller().Send()
			}
		}
	}()

	body, err = ioutil.ReadAll(hh.req.Body)
	if err != nil {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	err = reqMsg.Unpack(body)
	if err != nil {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	if !strings.HasSuffix(reqMsg.Question[0].Name, ".") {
		reqMsg.Question[0].Name += "."
	}

	// 查询内部域的记录
	if global.IsInternal(reqMsg.Question[0].Name) {
		respMsg, err = queryStorage(&reqMsg)
		if err != nil {
			log.Err(err).Caller().Msg("查询存储器失败")
			hh.respStatus(http.StatusInternalServerError)
			return
		}
	} else {
		// 查询上游服务
		upstream := Upstream{
			ReqMsg:      &reqMsg,
			MethodByDoT: hh.req.Method,
		}
		respMsg, err = upstream.Query()
		if err != nil {
			log.Err(err).Caller().Msg("查询上游服务失败")
			hh.respStatus(http.StatusInternalServerError)
			return
		}
	}

	respData, err = respMsg.Pack()
	if err != nil {
		log.Err(err).Caller().Msg("编码响应数据失败")
		hh.respStatus(http.StatusInternalServerError)
		return
	}

	_, err = hh.resp.Write(respData)
	if err != nil {
		log.Warn().Err(err).Caller().Msg("响应数据时出错")
	}
}

// -> GET /resolve -> json
func (hh *HTTPHandler) jsonQueryHandler() {
	var (
		err      error
		reqMsg   = new(dns.Msg)
		respData []byte
		respMsg  *dns.Msg
	)

	if global.Config.Service.HTTP.JSONQueryAuth && hh.req.Header.Get("Authorization") != global.Config.Service.HTTP.Authorization {
		hh.respStatus(http.StatusUnauthorized)
		return
	}

	defer func() {
		if hh.req.Body != nil {
			err = hh.req.Body.Close()
			if err != nil {
				log.Warn().Err(err).Caller().Send()
			}
		}
	}()

	if hh.req.URL.Query().Get("name") == "" || hh.req.URL.Query().Get("type") == "" {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	reqMsg.SetQuestion(hh.req.URL.Query().Get("name"), dns.StringToType[hh.req.URL.Query().Get("type")])
	if !strings.HasSuffix(reqMsg.Question[0].Name, ".") {
		reqMsg.Question[0].Name += "."
	}

	if global.IsInternal(reqMsg.Question[0].Name) {
		respMsg, err = queryStorage(reqMsg)
		if err != nil {
			log.Err(err).Caller().Msg("查询存储器失败")
			hh.respStatus(http.StatusInternalServerError)
			return
		}
	} else {
		upstream := Upstream{
			MethodByDoT: hh.req.Method,
			ReqMsg:      reqMsg,
		}
		respMsg, err = upstream.Query()
		if err != nil {
			log.Err(err).Caller().Msg("查询上游服务失败")
			hh.respStatus(http.StatusInternalServerError)
			return
		}
	}

	respData, err = json.Marshal(respMsg)
	if err != nil {
		log.Err(err).Caller().Msg("编码响应数据失败")
		hh.respStatus(http.StatusInternalServerError)
		return
	}

	_, err = hh.resp.Write(respData)
	if err != nil {
		log.Warn().Err(err).Caller().Msg("响应数据时出错")
	}
}

// 添加域名
func (hh *HTTPHandler) register(replace bool) {
	var (
		err   error
		rr    dns.RR
		oldRR []dns.RR
	)

	if global.Config.Service.HTTP.RegisterAuth && hh.req.Header.Get("Authorization") != global.Config.Service.HTTP.Authorization {
		hh.respStatus(http.StatusUnauthorized)
		return
	}

	if !hh.checkContentType() {
		return
	}

	err = hh.req.ParseForm()
	if err != nil {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	rr, err = dns.NewRR(hh.req.PostFormValue("rr"))
	if err != nil {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	if !replace {
		if oldRR, err = storage.Storage.Get(dns.Question{
			Name:   rr.Header().Name,
			Qtype:  rr.Header().Rrtype,
			Qclass: rr.Header().Class,
		}); err != nil {
			log.Err(err).Caller().Str("name", rr.Header().Name).Str("type", dns.TypeToString[rr.Header().Rrtype]).Msg("查询存储器记录时出错")
			hh.respStatus(http.StatusInternalServerError)
			return
		}

		if len(oldRR) > 0 {
			hh.respStatus(http.StatusBadRequest)
			return
		}
	}

	err = storage.Storage.Set(rr)
	if err != nil {
		log.Err(err).Caller().Str("name", rr.Header().Name).Str("type", dns.TypeToString[rr.Header().Rrtype]).Str("data", strings.TrimPrefix(rr.String(), rr.Header().String())).Msg("写入记录失败")
		hh.respStatus(http.StatusInternalServerError)
		return
	}

	hh.respStatus(http.StatusNoContent)
}

// 设置记录
func (hh *HTTPHandler) delete() {
	var (
		err   error
		rrBuf []byte
		rr    dns.RR
	)

	if global.Config.Service.HTTP.DeleteAuth && hh.req.Header.Get("Authorization") != global.Config.Service.HTTP.Authorization {
		hh.respStatus(http.StatusUnauthorized)
		return
	}

	if hh.req.URL.Query().Get("rr") == "" {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	rrBuf, err = base64.RawURLEncoding.DecodeString(hh.req.URL.Query().Get("rr"))
	if err != nil {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	rr, err = dns.NewRR(global.BytesToStr(rrBuf))
	if err != nil {
		hh.respStatus(http.StatusBadRequest)
		return
	}

	err = storage.Storage.Del(rr)
	if err != nil {
		log.Err(err).Caller().Str("name", rr.Header().Name).Str("type", dns.TypeToString[rr.Header().Rrtype]).Str("data", strings.TrimPrefix(rr.String(), rr.Header().String())).Msg("删除记录失败")
		hh.respStatus(http.StatusInternalServerError)
		return
	}

	hh.respStatus(http.StatusNoContent)
}

func (hh *HTTPHandler) checkContentType() bool {
	if !strings.HasPrefix(hh.req.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		hh.respStatus(http.StatusBadRequest)
		return false
	}
	return true
}
