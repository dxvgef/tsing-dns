package service

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"local/global"

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
	case "/dns-query":
		if req.Method == http.MethodGet {
			hh.dnsQueryByGET()
			break
		} else if req.Method == http.MethodPost {
			hh.dnsQueryByPOST()
			break
		}
		hh.respStatus(http.StatusMethodNotAllowed)
	case "/resolve":
		if req.Method != http.MethodGet {
			hh.respStatus(http.StatusMethodNotAllowed)
			break
		}
		hh.jsonHandler()
	default:
		hh.respStatus(http.StatusNotFound)
	}
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
func (hh *HTTPHandler) jsonHandler() {
	var (
		err      error
		reqMsg   = new(dns.Msg)
		respData []byte
		respMsg  *dns.Msg
	)

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
