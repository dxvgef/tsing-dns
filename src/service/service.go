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
	"local/storage/redis"
	"local/storage/voltdb"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// 存储器
var sa storage.Interface

// 构建存储器实例
func makeStorage() (err error) {
	// 初始化存储器
	switch global.Config.Storage.Type {
	case "redis":
		sa, err = redis.NewWithJSON(global.Config.Storage.Config)
		if err != nil {
			log.Fatal().Err(err).Caller().Msg("初始化存储器失败")
			break
		}
		log.Info().Str("type", "redis").Msg("使用Redis存储引擎")
	case "voltdb":
		sa, err = voltdb.NewWithJSON(global.Config.Storage.Config)
		if err != nil {
			log.Fatal().Err(err).Caller().Msg("初始化存储器失败")
			break
		}
		log.Info().Str("type", "voltdb").Msg("使用VoltDB存储引擎")
	default:
		log.Fatal().Caller().Str("type", global.Config.Storage.Type).Msg("未知的存储器类型")
	}
	return
}

// 启动socket服务
func Start() {
	var (
		err           error
		tcpService    *dns.Server
		tlsService    *dns.Server
		udpService    *dns.Server
		httpService   *http.Server
		httpsService  *http.Server
		socketHandler = new(GeneralHandler)
		httpHandler   = new(HTTPHandler)
	)

	err = makeStorage()
	if err != nil {
		log.Fatal().Caller().Msg("存储器构建失败")
		return
	}

	go func() {
		if global.Config.Service.UDP.Port < 1 {
			return
		}
		udpService = &dns.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.UDP.Port), 10),
			Net:     "udp",
			Handler: socketHandler,
		}
		log.Info().Str("Addr", udpService.Addr).Msg("启动DNS over UDP服务")
		if err = udpService.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Caller().Msg("启动DNS over UDP服务失败")
			return
		}
	}()

	go func() {
		if global.Config.Service.TCP.Port < 1 {
			return
		}
		tcpService = &dns.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.TCP.Port), 10),
			Net:     "tcp",
			Handler: socketHandler,
		}
		log.Info().Str("Addr", tcpService.Addr).Msg("启动DNS over TCP服务")
		if err = tcpService.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Caller().Msg("启动DNS over TCP服务失败")
			return
		}
	}()

	go func() {
		if global.Config.Service.TLS.Port < 1 {
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
			Handler:   socketHandler,
		}
		log.Info().Str("Addr", tlsService.Addr).Msg("启动DNS over TLS服务")
		if err = tlsService.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Caller().Msg("启动DNS over TLS服务失败")
			return
		}
	}()

	go func() {
		if global.Config.Service.HTTP.Port < 1 {
			return
		}
		httpService = &http.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.HTTP.Port), 10),
			Handler: httpHandler,
		}
		log.Info().Str("Addr", httpService.Addr).Msg("启动DNS over HTTP服务")
		if err = httpService.ListenAndServe(); err != nil {
			if err.Error() != http.ErrServerClosed.Error() {
				log.Fatal().Err(err).Caller().Msg("启动DNS over HTTP服务失败")
			}
			return
		}
	}()

	go func() {
		if global.Config.Service.HTTPS.Port < 1 {
			return
		}
		httpsService = &http.Server{
			Addr:    global.Config.Service.IP + ":" + strconv.FormatUint(uint64(global.Config.Service.HTTPS.Port), 10),
			Handler: httpHandler,
		}
		log.Info().Str("Addr", httpsService.Addr).Msg("启动DNS over HTTPS服务")
		if err = httpsService.ListenAndServeTLS(global.Config.Service.HTTPS.CertFile, global.Config.Service.HTTPS.KeyFile); err != nil {
			if err.Error() != http.ErrServerClosed.Error() {
				log.Fatal().Err(err).Caller().Msg("启动DNS over HTTPS服务失败")
			}
			return
		}
	}()

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
