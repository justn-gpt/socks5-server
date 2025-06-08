package main

import (
	"flag"
	"log"
	"os"

	"yourmodulepath/socks5"  // 根据你的项目模块名调整
)

func main() {
	configPath := flag.String("c", "ss5.json", "配置文件路径")
	flag.Parse()

	log.Printf("尝试加载配置文件: %s", *configPath)

	cfg, err := socks5.LoadConfig(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("配置文件不存在，尝试从环境变量读取配置")
			cfg = socks5.ParseOSEnvCfg()
		} else {
			log.Fatalf("加载配置文件失败: %v", err)
		}
	}

	log.Printf("加载到配置: %+v", cfg)

	server, err := socks5.NewServer(cfg)
	if err != nil {
		log.Fatalf("创建服务器失败: %v", err)
	}

	if err := server.Run(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
