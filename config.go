package main

import (
	"encoding/json"
	"os"
)

// Config 表示服务器配置
type Config struct {
	// 服务器监听地址
	Address string `json:"address"`
	// 认证用户列表
	Users map[string]string `json:"users"`
	// TLS配置
	TLS struct {
		// 是否启用TLS
		Enable bool `json:"enable"`
		// 证书文件路径
		CertFile string `json:"cert_file"`
		// 私钥文件路径
		KeyFile string `json:"key_file"`
	} `json:"tls"`
	// UDP配置
	UDP struct {
		// 是否启用UDP
		Enable bool `json:"enable"`
		// UDP监听地址，如果为空则使用与TCP相同的地址
		Address string `json:"address"`
		// UDP缓冲区大小（字节）
		BufferSize int `json:"buffer_size"`
		// UDP会话超时时间（秒）
		Timeout int `json:"timeout"`
	} `json:"udp"`
}

// LoadConfig 从指定路径加载配置文件
func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	// 设置默认值
	if config.Address == "" {
		config.Address = ":1080"
	}
	if config.Users == nil {
		config.Users = make(map[string]string)
	}

	return &config, nil
}