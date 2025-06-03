# SOCKS5 代理服务器

这是一个用Go语言实现的SOCKS5代理服务器，支持TCP和UDP代理，并具有用户认证和TLS加密功能。

## 功能特性

- 支持SOCKS5协议标准
- 支持TCP和UDP代理
- 支持用户名/密码认证
- 支持TLS加密连接
- 可配置的UDP缓冲区大小和超时时间
- 跨平台支持（Windows、Linux）

## 配置

服务器通过 `config.json` 文件进行配置。以下是配置文件的示例：

```json
{
  "address": ":1080",
  "users": {
    "admin": "admin"
  },
  "tls": {
    "enable": false,
    "cert_file": "cert.pem",
    "key_file": "key.pem"
  },
  "udp": {
    "enable": true,
    "address": "",
    "buffer_size": 4096,
    "timeout": 60
  }
}
```

### 配置选项说明

- `address`: 服务器监听地址，格式为 "IP:端口"。默认为 ":1080"
- `users`: 用户认证信息，key为用户名，value为密码。留空则不启用认证
- `tls`: TLS加密配置
  - `enable`: 是否启用TLS加密
  - `cert_file`: TLS证书文件路径
  - `key_file`: TLS私钥文件路径
- `udp`: UDP代理配置
  - `enable`: 是否启用UDP代理
  - `address`: UDP监听地址，留空则使用与TCP相同的地址
  - `buffer_size`: UDP缓冲区大小（字节）
  - `timeout`: UDP会话超时时间（秒）

## 使用方法

1. 创建配置文件 `config.json`，根据需要修改配置选项

2. 运行服务器：
   ```bash
   ./socks5-server
   ```

3. 服务器启动后，可以在支持SOCKS5代理的客户端中使用：
   - 代理服务器地址：你的服务器IP
   - 代理服务器端口：配置文件中指定的端口（默认1080）
   - 如果配置了认证，需要填写用户名和密码
   - 如果启用了TLS，需要在客户端配置使用TLS连接

## 注意事项

1. 如果启用TLS，请确保证书和私钥文件路径配置正确
2. 建议在生产环境中启用用户认证以提高安全性
3. UDP代理功能可能会占用较多系统资源，请根据服务器配置适当调整缓冲区大小
4. 确保防火墙允许配置的端口访问
