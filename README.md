# DNS Service
使用 Go 语言开发的 DNS 服务，功能特性如下：
- 支持全类型的记录解析
- 支持 UDP, DNS over TCP/TLS, DNS over HTTP/HTTPS / HTTP JSON协议的下游客户端查询
- 支持向上游 DNS 服务轮循转发查询
- 可通过 HTTP, Socks5 代理向上游 DNS 服务发起请求
- 上游 DNS 服务支持 UDP, TCP, DoT, DoH 协议
- 可内部解析指定后缀的域名
- 内部解析的存储器已支持 Redis(v6), VoltDB

## 服务端口
- UDP/TCP : 53
- DNS over TLS (RFC 7858) : 853
- DNS over HTTP : 80
- DNS over HTTPS : 443
  
## 服务端点
### DNS over HTTP/HTTPS
- 方法：GET/POST
- 路径：/dns-query

需要按照 RFC-8484 中的标准要求生成 DoH 的 HTTP Request 请求。可参考此文档：https://help.aliyun.com/document_detail/171664.html?spm=a2c4g.11186623.6.567.788a6b83x3mcsk

### HTTP API 域名查询
- 方法：GET
- 路径：/resolve
- 请求参数：
  - name：string,要解析的域名
  - type：integer，要解析的记录类型，例如A,CNAME,MX等等

返回参数示例如下：
```json
{
  "Status": 0,
  "TC": false,
  "RD": true,
  "RA": true,
  "AD": false,
  "CD": false,
  "Question": [
    {
      "name": "test.com.",
      "type": 1
    }
  ],
  "Answer": [
    {
      "name": "test.com.",
      "type": 1,
      "TTL": 300,
      "data": "127.0.0.1"
    }
  ]
}
```

### HTTP API 域名注册
- 方法：PUT
- 路径：/set
- 请求参数：用空格拼接各参数值组成的字符串
- 说明：当记录存在时覆盖，不存在则创建

常见域名记录类型的请求参数说明：

| 域名 | 记录类型 | 字符串结构 | 示例 |
| --- | --- | --- | --- |
| test.com | A | name  TTL  class  type  IP | test.com  300  IN  A  127.0.0.1 |
| www.test.com | CNAME | name TTL class type target | www.test.com  300  IN  CNAME  test.com |
| test.com | MX | name TTL class type preference MX | mail.test.com  300  IN  MX  1  mail.test.com | 
| txt.test.com | TXT | name TTL class type text | mail.test.com  300  IN  TXT  "string" |
| srv.test.com | SRV | name TTL class type priority weight port target | srv.test.com  300  IN  SRV  1  10  80  127.0.0.1 |
| ipv6.test.com | AAAA | name TTL class type AAAA | ipv6.test.com  300  IN  AAAA  ::1 |
| test.com | NS | name TTL class type dns-server | test.com  300  IN  NS  dns.test.com |
| test.com | CAA | name TTL class type flag tag value | test.com  300  IN  CAA  0 issue "test.com" |
