// Package main implements a SOCKS5 server according to RFC 1928 and RFC 1929
package main

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
)

// SOCKS5 protocol constants
const (
	Version5 = uint8(5)
)

// Authentication methods
const (
	MethodNoAuth = uint8(0x00)
	MethodGSSAPI = uint8(0x01)
	MethodUserPass = uint8(0x02)
	MethodNoAcceptable = uint8(0xFF)
)

// Authentication versions
const (
	AuthUserPassVersion = uint8(0x01)
	AuthUserPassSuccess = uint8(0x00)
	AuthUserPassFailure = uint8(0x01)
)

// Command types
const (
	CmdConnect = uint8(0x01)
	CmdBind = uint8(0x02)
	CmdUDPAssociate = uint8(0x03)
)

// Address types
const (
	TypeIPv4 = uint8(0x01)
	TypeDomain = uint8(0x03)
	TypeIPv6 = uint8(0x04)
)

// Reply codes
const (
	RepSuccess = uint8(0x00)
	RepServerFailure = uint8(0x01)
	RepConnectionNotAllowed = uint8(0x02)
	RepNetworkUnreachable = uint8(0x03)
	RepHostUnreachable = uint8(0x04)
	RepConnectionRefused = uint8(0x05)
	RepTTLExpired = uint8(0x06)
	RepCommandNotSupported = uint8(0x07)
	RepAddressTypeNotSupported = uint8(0x08)
)

// Credentials represents username/password authentication credentials
type Credentials struct {
	Username string
	Password string
}

// Server represents a SOCKS5 server
type Server struct {
	addr        string
	credentials map[string]string // username -> password
	authEnabled bool
	tlsConfig   *tls.Config
	useTLS      bool
	udpHandler  *UDPHandler      // UDP处理器
	config      *Config          // 服务器配置
}

// NewServer creates a new SOCKS5 server
func NewServer(config *Config) *Server {
	var tlsConfig *tls.Config
	useTLS := false
	
	if config.TLS.Enable {
		cert, err := tls.LoadX509KeyPair(config.TLS.CertFile, config.TLS.KeyFile)
		if err == nil {
			tlsConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:  tls.VersionTLS12,
			}
			useTLS = true
		} else {
			log.Printf("TLS证书加载失败: %v, 将使用非TLS模式", err)
		}
	}
	
	server := &Server{
		addr:        config.Address,
		credentials: config.Users,
		authEnabled: len(config.Users) > 0,
		tlsConfig:   tlsConfig,
		useTLS:      useTLS,
		config:      config,
	}

	if config.UDP.Enable {
		server.udpHandler = NewUDPHandler(config)
	}
	
	return server
}

// Start starts the SOCKS5 server
func (s *Server) Start() error {
	var listener net.Listener
	var err error

	// 启动UDP服务（如果启用）
	if s.udpHandler != nil {
		if err := s.udpHandler.Start(); err != nil {
			return fmt.Errorf("启动UDP服务失败: %v", err)
		}
	}

	// 启动TCP服务
	if s.useTLS {
		listener, err = tls.Listen("tcp", s.addr, s.tlsConfig)
		if err != nil {
			return fmt.Errorf("启动TLS服务器失败: %v", err)
		}
		log.Printf("SOCKS5 服务器正在监听 %s (TLS模式, 认证模式: %v)", s.addr, s.authEnabled)
	} else {
		listener, err = net.Listen("tcp", s.addr)
		if err != nil {
			return fmt.Errorf("启动服务器失败: %v", err)
		}
		log.Printf("SOCKS5 服务器正在监听 %s (认证模式: %v)", s.addr, s.authEnabled)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接受连接失败: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// Stop stops the SOCKS5 server
func (s *Server) Stop() {
	// 停止UDP服务
	if s.udpHandler != nil {
		s.udpHandler.Stop()
	}
}

// handleConnection processes a client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	if err := s.handleHandshake(conn); err != nil {
		log.Printf("握手失败: %v", err)
		return
	}

	if err := s.handleRequest(conn); err != nil {
		log.Printf("请求处理失败: %v", err)
		return
	}
}

// handleHandshake performs the SOCKS5 handshake
func (s *Server) handleHandshake(conn net.Conn) error {
	// Read version and number of methods
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("读取握手头部失败: %v", err)
	}

	version := header[0]
	if version != Version5 {
		return fmt.Errorf("不支持的SOCKS版本: %d", version)
	}

	nmethods := header[1]
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("读取认证方法列表失败: %v", err)
	}

	// Check supported authentication methods
	var method uint8 = MethodNoAcceptable
	if s.authEnabled {
		// If authentication is enabled, we only accept username/password
		for _, m := range methods {
			if m == MethodUserPass {
				method = MethodUserPass
				break
			}
		}
	} else {
		// If authentication is disabled, we accept no authentication
		for _, m := range methods {
			if m == MethodNoAuth {
				method = MethodNoAuth
				break
			}
		}
	}

	// Send selected method
	if _, err := conn.Write([]byte{Version5, method}); err != nil {
		return fmt.Errorf("failed to send auth method: %v", err)
	}

	if method == MethodNoAcceptable {
		return errors.New("no supported authentication methods")
	}

	// Perform authentication if required
	if method == MethodUserPass {
		return s.handleUserPassAuth(conn)
	}

	return nil
}

// handleUserPassAuth handles username/password authentication
func (s *Server) handleUserPassAuth(conn net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("failed to read auth header: %v", err)
	}

	version := header[0]
	if version != AuthUserPassVersion {
		return fmt.Errorf("unsupported auth version: %d", version)
	}

	// Read username
	userLen := int(header[1])
	username := make([]byte, userLen)
	if _, err := io.ReadFull(conn, username); err != nil {
		return fmt.Errorf("failed to read username: %v", err)
	}

	// Read password
	passLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, passLenBuf); err != nil {
		return fmt.Errorf("failed to read password length: %v", err)
	}
	passLen := int(passLenBuf[0])
	password := make([]byte, passLen)
	if _, err := io.ReadFull(conn, password); err != nil {
		return fmt.Errorf("failed to read password: %v", err)
	}

	// Verify credentials
	if s.verifyCredentials(string(username), string(password)) {
		_, err := conn.Write([]byte{AuthUserPassVersion, AuthUserPassSuccess})
		return err
	}

	conn.Write([]byte{AuthUserPassVersion, AuthUserPassFailure})
	return errors.New("invalid credentials")
}

// verifyCredentials verifies the provided username and password
func (s *Server) verifyCredentials(username, password string) bool {
	if storedPass, ok := s.credentials[username]; ok {
		return storedPass == password
	}
	return false
}

// handleRequest processes the client's connection request
func (s *Server) handleRequest(conn net.Conn) error {
	// Read version, command, reserved, and address type
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("读取请求失败: %v", err)
	}

	version := header[0]
	if version != Version5 {
		return fmt.Errorf("不支持的版本: %d", version)
	}

	command := header[1]
	addrType := header[3]

	// 读取目标地址
	var addr string
	var err error

	switch addrType {
	case TypeIPv4:
		addr, err = s.readIPv4(conn)
	case TypeDomain:
		addr, err = s.readDomain(conn)
	case TypeIPv6:
		addr, err = s.readIPv6(conn)
	default:
		s.sendReply(conn, RepAddressTypeNotSupported, nil)
		return fmt.Errorf("不支持的地址类型: %d", addrType)
	}

	if err != nil {
		s.sendReply(conn, RepServerFailure, nil)
		return fmt.Errorf("读取地址失败: %v", err)
	}

	// 读取端口
	var port uint16
	if err := binary.Read(conn, binary.BigEndian, &port); err != nil {
		s.sendReply(conn, RepServerFailure, nil)
		return fmt.Errorf("读取端口失败: %v", err)
	}

	target := fmt.Sprintf("%s:%d", addr, port)

	// 根据命令类型处理请求
	switch command {
	case CmdConnect:
		return s.handleConnect(conn, target)
	case CmdUDPAssociate:
		return s.handleUDPAssociate(conn)
	default:
		s.sendReply(conn, RepCommandNotSupported, nil)
		return fmt.Errorf("不支持的命令: %d", command)
	}
}

// handleConnect 处理 CONNECT 命令
func (s *Server) handleConnect(conn net.Conn, target string) error {
	// 连接目标服务器
	dest, err := net.Dial("tcp", target)
	if err != nil {
		s.sendReply(conn, RepConnectionRefused, nil)
		return fmt.Errorf("连接目标服务器失败: %v", err)
	}
	defer dest.Close()

	// 发送成功响应
	local := dest.LocalAddr().(*net.TCPAddr)
	if err := s.sendReply(conn, RepSuccess, local); err != nil {
		return fmt.Errorf("发送响应失败: %v", err)
	}

	// 开始数据转发
	errCh := make(chan error, 2)
	go s.proxy(conn, dest, errCh)
	go s.proxy(dest, conn, errCh)

	// 等待连接关闭
	err = <-errCh
	return err
}

// handleUDPAssociate 处理 UDP ASSOCIATE 命令
func (s *Server) handleUDPAssociate(conn net.Conn) error {
	// 检查是否启用了UDP支持
	if s.udpHandler == nil {
		s.sendReply(conn, RepCommandNotSupported, nil)
		return fmt.Errorf("UDP支持未启用")
	}

	// 获取UDP监听地址
	udpAddr := s.udpHandler.listener.LocalAddr().(*net.UDPAddr)

	// 发送UDP服务器地址给客户端
	if err := s.sendReply(conn, RepSuccess, &net.TCPAddr{
		IP:   udpAddr.IP,
		Port: udpAddr.Port,
	}); err != nil {
		return fmt.Errorf("发送UDP绑定地址失败: %v", err)
	}

	// 保持TCP连接，直到客户端断开
	// 这是必要的，因为UDP关联需要依赖于TCP控制连接
	buffer := make([]byte, 1)
	for {
		_, err := conn.Read(buffer)
		if err != nil {
			return nil // 客户端断开连接，正常退出
		}
	}
}

// readIPv4 reads an IPv4 address
func (s *Server) readIPv4(conn net.Conn) (string, error) {
	addr := make([]byte, 4)
	if _, err := io.ReadFull(conn, addr); err != nil {
		return "", err
	}
	return net.IP(addr).String(), nil
}

// readIPv6 reads an IPv6 address
func (s *Server) readIPv6(conn net.Conn) (string, error) {
	addr := make([]byte, 16)
	if _, err := io.ReadFull(conn, addr); err != nil {
		return "", err
	}
	return net.IP(addr).String(), nil
}

// readDomain reads a domain name
func (s *Server) readDomain(conn net.Conn) (string, error) {
	length := make([]byte, 1)
	if _, err := io.ReadFull(conn, length); err != nil {
		return "", err
	}

	domain := make([]byte, length[0])
	if _, err := io.ReadFull(conn, domain); err != nil {
		return "", err
	}

	return string(domain), nil
}

// sendReply sends a reply to the client
func (s *Server) sendReply(conn net.Conn, rep uint8, addr *net.TCPAddr) error {
	response := make([]byte, 4)
	response[0] = Version5
	response[1] = rep
	response[2] = 0x00 // Reserved

	if addr == nil {
		response[3] = TypeIPv4
		response = append(response, make([]byte, 6)...)
	} else {
		ip := addr.IP.To4()
		if ip != nil {
			response[3] = TypeIPv4
			response = append(response, ip...)
		} else {
			response[3] = TypeIPv6
			response = append(response, addr.IP.To16()...)
		}
		port := make([]byte, 2)
		binary.BigEndian.PutUint16(port, uint16(addr.Port))
		response = append(response, port...)
	}

	_, err := conn.Write(response)
	return err
}

// proxy copies data between two connections
func (s *Server) proxy(dst io.Writer, src io.Reader, errCh chan error) {
	_, err := io.Copy(dst, src)
	errCh <- err
}