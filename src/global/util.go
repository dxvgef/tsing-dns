package global

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"unsafe"

	"github.com/miekg/dns"
)

// []byte转string
func BytesToStr(value []byte) string {
	return *(*string)(unsafe.Pointer(&value))
}

// 字符串转[]byte
func StrToBytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

// 对字符串生成MD5 16Bit签名
func KeySign(rr dns.RR) (cipher string, err error) {
	data := strings.TrimPrefix(rr.String(), rr.Header().String())
	h := md5.New()
	if _, err = h.Write(StrToBytes(data)); err != nil {
		return
	}
	cipher = hex.EncodeToString(h.Sum(nil))
	cipher = cipher[8:24]
	return
}

// 查询是否是内部域名
func IsInternal(name string) bool {
	for k := range Config.Service.InternalSuffix {
		if strings.HasSuffix(name, Config.Service.InternalSuffix[k]) {
			return true
		}
	}
	return false
}
