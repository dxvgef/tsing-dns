package main

import (
	"local/global"
	"local/service"
	"local/storage"

	_ "github.com/VoltDB/voltdb-client-go/voltdbclient"
)

func main() {
	var err error

	// 使用默认的日志记录器
	global.UseDefaultLogger()

	// 加载配置
	if err = global.LoadConfig(); err != nil {
		return
	}

	// 配置日志记录器
	if err = global.SetupLogger(); err != nil {
		return
	}

	// 构建存储器
	if err = storage.MakeStorage(); err != nil {
		return
	}

	// 启动socket服务
	service.Start()
}
