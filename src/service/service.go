package service

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"local/global"
	"local/storage"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// 启用socket服务
func Start() {
	var (
		err            error
		tcpService     *dns.Server
		tlsService     *dns.Server
		udpService     *dns.Server
		httpService    *http.Server
		httpsService   *http.Server
		generalHandler = new(GeneralHandler)
		httpHandler    = new(HTTPHandler)
	)

	if global.Config.Service.Upstream.Count == 0 && len(global.Config.Service.InternalSuffix) == 0 {
		log.Fatal().Msg("程序已退出，因DNS转发和内部域名解析服务都未启用")
		os.Exit(0)
	}

	go func() {
		if global.Config.Service.UDP.Port < 1 {
			log.Warn().Msg("已禁用 DNS over UDP，因 service.udp.port 参数未配置")
			return
		}
		udpService = &dns.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.UDP.Port), 10),
			Net:     "udp",
			Handler: generalHandler,
		}
		log.Info().Str("Addr", udpService.Addr).Msg("启用 DNS over UDP")
		if err = udpService.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Caller().Msg("启用 DNS over UDP 失败")
			return
		}
	}()

	go func() {
		if global.Config.Service.TCP.Port < 1 {
			log.Warn().Msg("已禁用 DNS over TCP，因 service.tcp.port 参数未配置")
			return
		}
		tcpService = &dns.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.TCP.Port), 10),
			Net:     "tcp",
			Handler: generalHandler,
		}
		log.Info().Str("Addr", tcpService.Addr).Msg("启用 DNS over TCP")
		if err = tcpService.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Caller().Msg("启用 DNS over TCP 失败")
			return
		}
	}()

	go func() {
		if global.Config.Service.TLS.Port < 1 {
			log.Warn().Msg("已禁用 DNS over TLS，因 service.tls.port 参数未配置")
			return
		}
		var cert tls.Certificate
		cert, err = tls.LoadX509KeyPair(global.Config.Service.TLS.CertFile, global.Config.Service.TLS.KeyFile)
		if err != nil {
			log.Fatal().Err(err).Caller().Str("cert", global.Config.Service.TLS.CertFile).Str("key", global.Config.Service.TLS.KeyFile).Msg("加载TLS的证书或密钥文件失败")
			return
		}
		tlsService = &dns.Server{
			Addr:      global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.TLS.Port), 10),
			Net:       "tcp-tls",
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}, // nolint:gosec
			Handler:   generalHandler,
		}
		log.Info().Str("Addr", tlsService.Addr).Msg("启用 DNS over TLS")
		if err = tlsService.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Caller().Msg("启用 DNS over TLS 失败")
			return
		}
	}()

	go func() {
		if global.Config.Service.HTTP.Port < 1 {
			log.Warn().Msg("已禁用 HTTP，因 service.http.port 参数未配置")
			return
		}
		if global.Config.Service.HTTP.DNSQueryPath == "" &&
			global.Config.Service.HTTP.JSONQueryPath == "" &&
			global.Config.Service.HTTP.RegisterPath == "" {
			log.Warn().Msg("已禁用 HTTP，因依赖 HTTP 的功能全部未启用")
			return
		}
		httpService = &http.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.HTTP.Port), 10),
			Handler: httpHandler,
		}
		log.Info().Str("Addr", httpService.Addr).Msg("启用 HTTP")
		if err = httpService.ListenAndServe(); err != nil {
			if err.Error() != http.ErrServerClosed.Error() {
				log.Fatal().Err(err).Caller().Msg("启用 HTTP 失败")
			}
			return
		}
	}()

	go func() {
		if global.Config.Service.HTTP.SSLPort < 1 {
			log.Warn().Msg("已禁用 HTTPS，因 service.http.sslPort 参数未配置")
			return
		}
		if global.Config.Service.HTTP.DNSQueryPath == "" &&
			global.Config.Service.HTTP.JSONQueryPath == "" &&
			global.Config.Service.HTTP.RegisterPath == "" {
			log.Warn().Msg("已禁用 HTTPS，因依赖 HTTPS 的功能全部未启用")
			return
		}
		httpsService = &http.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.HTTP.SSLPort), 10),
			Handler: httpHandler,
		}
		log.Info().Str("Addr", httpsService.Addr).Msg("启用 HTTPS")
		if err = httpsService.ListenAndServeTLS(global.Config.Service.HTTP.CertFile, global.Config.Service.HTTP.KeyFile); err != nil {
			if err.Error() != http.ErrServerClosed.Error() {
				log.Fatal().Err(err).Caller().Msg("启用 HTTPS 失败")
			}
			return
		}
	}()

	if global.Config.Service.HTTP.Port > 0 || global.Config.Service.HTTP.SSLPort > 0 {
		if global.Config.Service.HTTP.DNSQueryPath != "" {
			log.Info().Str("method", "GET/POST").Str("path", global.Config.Service.HTTP.DNSQueryPath).Msg("启用 DNS over HTTP")
		} else {
			log.Warn().Msg("已禁用 DNS over HTTP，因 service.http.dnsQueryPath 参数未设置")
		}
		if global.Config.Service.HTTP.JSONQueryPath != "" {
			log.Info().Str("method", http.MethodGet).Str("path", global.Config.Service.HTTP.JSONQueryPath).Msg("启用 HTTP JSON")
		} else {
			log.Warn().Msg("已禁用 HTTP JSON，因 service.http.jsonQueryPath 参数未设置")
		}
		if global.Config.Service.HTTP.RegisterPath != "" {
			log.Info().Str("method", "POST/PUT").Str("path", global.Config.Service.HTTP.RegisterPath).Msg("启用 HTTP 注册")
		} else {
			log.Warn().Msg("已禁用 HTTP 注册，因 service.http.registerPath 参数未设置")
		}
	}

	if global.Config.Service.Upstream.Count < 1 {
		log.Warn().Msg("已禁用 DNS 转发，因 service.upstream.addr 参数为空")
	} else {
		log.Info().Msg("启用 DNS 转发")
	}

	if len(global.Config.Service.InternalSuffix) < 1 {
		log.Warn().Msg("已禁用内部域名解析，因 service.internalSuffix 参数为空")
	} else {
		// 构建存储器
		err = storage.MakeStorage()
		if err != nil {
			log.Fatal().Caller().Err(err).Msg("构建存储器失败")
			return
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(global.Config.Service.QuitWaitTimeout)*time.Second)
	defer cancel()
	if udpService != nil {
		if err = udpService.ShutdownContext(ctx); err != nil {
			if err.Error() != context.DeadlineExceeded.Error() {
				log.Err(err).Caller().Msg("DNS over UDP服务关闭时出现异常")
			}
		}
	}
	if tcpService != nil {
		if err = tcpService.ShutdownContext(ctx); err != nil {
			if err.Error() != context.DeadlineExceeded.Error() {
				log.Err(err).Caller().Msg("DNS over TCP服务关闭时出现异常")
			}
		}
	}
	if tlsService != nil {
		if err = tlsService.ShutdownContext(ctx); err != nil {
			if err.Error() != context.DeadlineExceeded.Error() {
				log.Err(err).Caller().Msg("DNS over TLS服务关闭时出现异常")
			}
		}
	}
	if httpService != nil {
		if err = httpService.Shutdown(ctx); err != nil {
			if err.Error() != context.DeadlineExceeded.Error() {
				log.Err(err).Caller().Msg("DNS over HTTP服务关闭时出现异常")
			}
		}
	}
	if httpsService != nil {
		if err = httpsService.Shutdown(ctx); err != nil {
			if err.Error() != context.DeadlineExceeded.Error() {
				log.Err(err).Caller().Msg("DNS over HTTPS服务关闭时出现异常")
			}
		}
	}
}
