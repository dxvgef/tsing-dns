# Tsing DNS
使用 Go 语言开发的 DNS 服务，功能特性如下：
- 支持全类型的记录解析
- 支持 UDP, DNS over TCP/TLS, DNS over HTTP/HTTPS / HTTP JSON协议的下游客户端查询
- 支持向上游 DNS 服务轮循转发查询
- 可通过 HTTP, Socks5 代理向上游 DNS 服务发起请求
- 上游 DNS 服务支持 UDP, TCP, DoT, DoH 协议
- 可内部解析指定后缀的域名
- 内部解析的存储器已支持 Redis(v6), VoltDB
