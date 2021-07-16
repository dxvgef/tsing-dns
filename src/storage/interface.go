package storage

import "github.com/miekg/dns"

// 存储器接口
type Interface interface {
	Set(rr dns.RR, ttl uint32) (err error)
	Get(question dns.Question) (result []dns.RR, err error)
	Del(rr dns.RR) (err error)
}
