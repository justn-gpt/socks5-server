package main

import (
	"flag"
	"log"
)

func main() {
	configPath := flag.String("c", "config.json", "配置文件路径")
	flag.Parse()

	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	server := NewServer(config)

	if len(config.Users) > 0 {
		log.Printf("已加载 %d 个用户的认证信息", len(config.Users))
	} else {
		log.Printf("未配置认证信息，将使用无认证模式")
	}

	if config.TLS.Enable {
		log.Printf("TLS 加密已启用，证书文件: %s, 密钥文件: %s", config.TLS.CertFile, config.TLS.KeyFile)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
