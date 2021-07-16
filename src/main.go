package main

import (
	"local/global"
	"local/service"

	_ "github.com/VoltDB/voltdb-client-go/voltdbclient"
)

func main() {
	var err error

	global.UseDefaultLogger()

	if err = global.LoadConfig(); err != nil {
		return
	}

	if err = global.SetupLogger(); err != nil {
		return
	}

	// 启动socket服务
	service.Start()
}
