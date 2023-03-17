package example

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// 测试注册域名
func TestRegister(t *testing.T) {
	var (
		err    error
		client = http.DefaultClient
		req    *http.Request
		resp   *http.Response
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err = http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1/register", strings.NewReader("rr=dxvgef.test 3600 IN A 127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "123456")
	resp, err = client.Do(req)
	defer func() {
		if err = resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode, resp.Status)
	}
	t.Log(resp.StatusCode, resp.Status)
}

// 测试使用UDP查询域名
func TestUDP(t *testing.T) {
	var (
		err     error
		client  dns.Client
		reqMsg  = new(dns.Msg)
		respMsg *dns.Msg
	)
	client.Net = "udp"
	client.Timeout = 5 * time.Second
	reqMsg.SetQuestion("dxvgef.test.", dns.TypeA)
	respMsg, _, err = client.Exchange(reqMsg, "127.0.0.1:153")
	if err != nil {
		t.Fatal(err)
	}

	// rCode为0表示正常，3表示没有记录，其它都是错误
	t.Log(respMsg.String())
}

// 测试使用TCP查询域名
func TestTCP(t *testing.T) {
	var (
		err     error
		client  dns.Client
		reqMsg  = new(dns.Msg)
		respMsg *dns.Msg
	)
	client.Net = "tcp"
	client.Timeout = 5 * time.Second
	reqMsg.SetQuestion("dxvgef.test.", dns.TypeA)
	respMsg, _, err = client.Exchange(reqMsg, "127.0.0.1:153")
	if err != nil {
		t.Fatal(err)
	}

	// rCode为0表示正常，3表示没有记录，其它都是错误
	t.Log(respMsg.String())
}

// 测试使用HTTP GET查询域名
func TestHTTPGET(t *testing.T) {
	var (
		err     error
		req     *http.Request
		resp    *http.Response
		body    []byte
		reqMsg  dns.Msg
		respMsg dns.Msg
	)

	reqMsg.SetQuestion("dxvgef.test.", dns.TypeA)
	body, err = reqMsg.Pack()
	if err != nil {
		t.Fatal(err)
	}

	bodyStr := base64.RawURLEncoding.EncodeToString(body)

	client := http.DefaultClient

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err = http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1/dns-query?dns="+bodyStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode, resp.Status)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	err = respMsg.Unpack(body)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(respMsg.String())
}

// 测试使用HTTP POST查询域名
func TestHTTPPOST(t *testing.T) {
	var (
		err     error
		req     *http.Request
		resp    *http.Response
		body    []byte
		reqMsg  dns.Msg
		respMsg dns.Msg
	)

	reqMsg.SetQuestion("dxvgef.test.", dns.TypeA)
	body, err = reqMsg.Pack()
	if err != nil {
		t.Fatal(err)
	}

	client := http.DefaultClient
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err = http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1/dns-query", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	err = respMsg.Unpack(body)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(respMsg.String())
}

// 测试HTTP JSON
func TestResolve(t *testing.T) {
	var (
		err  error
		req  *http.Request
		resp *http.Response
		body []byte
	)

	qName := "dxvgef.test"
	qType := "A"
	client := http.DefaultClient
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err = http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1/resolve?name="+qName+"&type="+qType, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log(string(body))
}

// 测试查询外部域名
func TestInternet(t *testing.T) {
	var (
		err     error
		client  dns.Client
		reqMsg  = new(dns.Msg)
		respMsg *dns.Msg
	)
	client.Net = "udp"
	client.Timeout = 5 * time.Second
	reqMsg.SetQuestion("163.com.", dns.TypeA)
	respMsg, _, err = client.Exchange(reqMsg, "127.0.0.1:153")
	if err != nil {
		t.Fatal(err)
	}

	// rCode为0表示正常，3表示没有记录，其它都是错误
	t.Log(respMsg.String())
}
