package main

import (
	"flag"
	"log"
	"os"

	"socks5-server"  // 注意这里导入模块本身，或者如果是根目录，import 可能不需要额外路径
)

func main() {
	configPath := flag.String("c", "config.json", "配置文件路径")
	flag.Parse()

	log.Printf("尝试加载配置文件: %s", *configPath)

	cfg, err := socks5.LoadConfig(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("配置文件 %s 不存在，尝试使用环境变量配置", *configPath)
			cfg = socks5.ParseOSEnvCfg()
		} else {
			log.Fatalf("加载配置文件失败: %v", err)
		}
	}

	socks5.CheckServerCfgDefault(cfg)

	log.Printf("加载到配置: %+v", cfg)

	server, err := socks5.NewServer(cfg)
	if err != nil {
		log.Fatalf("创建服务器失败: %v", err)
	}

	if err := server.Run(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
