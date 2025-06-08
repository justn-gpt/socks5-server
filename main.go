package main

import (
	"flag"
	"log"
	"os"

	"github.com/justn-gpt/socks5-server"
)

func main() {
	configPath := flag.String("c", "config.json", "配置文件路径")
	flag.Parse()

	log.Printf("尝试加载配置文件: %s", *configPath)

	cfg, err := socks5.LoadConfig(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("配置文件不存在，使用环境变量配置功能暂未实现")
			// 这里暂时不调用 parseOSEnvCfg，避免编译错误
			os.Exit(1)
		} else {
			log.Fatalf("加载配置文件失败: %v", err)
		}
	}

	// 如果需要校验配置，请自己实现
	// socks5.CheckServerCfgDefault(cfg)

	log.Printf("加载配置: %+v", cfg)

	server := socks5.NewServer(cfg)

	if err := server.Run(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
