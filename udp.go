package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// UDPSession 表示一个UDP会话
type UDPSession struct {
	clientAddr *net.UDPAddr
	targetConn *net.UDPConn
	lastActive time.Time
}

// UDPAssociateRequest UDP关联请求的地址信息
type UDPAssociateRequest struct {
	ClientConn *net.TCPConn
	BindAddr   *net.UDPAddr
}

// UDPHandler 处理UDP请求
type UDPHandler struct {
	sessions     map[string]*UDPSession
	sessionsLock sync.RWMutex
	config       *Config
	listener     *net.UDPConn
}

// NewUDPHandler 创建新的UDP处理器
func NewUDPHandler(config *Config) *UDPHandler {
	return &UDPHandler{
		sessions: make(map[string]*UDPSession),
		config:   config,
	}
}

// Start 启动UDP监听
func (h *UDPHandler) Start() error {
	addr := h.config.UDP.Address
	if addr == "" {
		addr = h.config.Address
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("解析UDP地址失败: %v", err)
	}

	h.listener, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("启动UDP监听失败: %v", err)
	}

	log.Printf("UDP服务器正在监听 %s", addr)

	// 启动会话清理
	go h.cleanSessions()

	// 处理UDP数据
	go h.handleUDP()

	return nil
}

// cleanSessions 定期清理过期的会话
func (h *UDPHandler) cleanSessions() {
	ticker := time.NewTicker(time.Duration(h.config.UDP.Timeout) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.sessionsLock.Lock()
		now := time.Now()
		for key, session := range h.sessions {
			if now.Sub(session.lastActive) > time.Duration(h.config.UDP.Timeout)*time.Second {
				session.targetConn.Close()
				delete(h.sessions, key)
				log.Printf("清理过期UDP会话: %s", key)
			}
		}
		h.sessionsLock.Unlock()
	}
}

// handleUDP 处理UDP数据
func (h *UDPHandler) handleUDP() {
	buffer := make([]byte, h.config.UDP.BufferSize)
	for {
		n, clientAddr, err := h.listener.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("读取UDP数据失败: %v", err)
			continue
		}

		// UDP数据报格式：
		// +----+------+------+----------+----------+----------+
		// |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA  |
		// +----+------+------+----------+----------+----------+
		// | 2  |  1   |  1   | Variable |    2     | Variable|
		// +----+------+------+----------+----------+----------+

		if n < 4 {
			continue
		}

		// 跳过RSV和FRAG字段
		headerSize := 4
		atyp := buffer[3]

		var dstAddr string
		var dstPort uint16

		switch atyp {
		case 0x01: // IPv4
			if n < headerSize+4+2 {
				continue
			}
			ip := net.IP(buffer[headerSize : headerSize+4])
			dstAddr = ip.String()
			dstPort = binary.BigEndian.Uint16(buffer[headerSize+4 : headerSize+6])
			headerSize += 6
		case 0x03: // Domain name
			if n < headerSize+1 {
				continue
			}
			domainLen := int(buffer[headerSize])
			if n < headerSize+1+domainLen+2 {
				continue
			}
			dstAddr = string(buffer[headerSize+1 : headerSize+1+domainLen])
			dstPort = binary.BigEndian.Uint16(buffer[headerSize+1+domainLen : headerSize+1+domainLen+2])
			headerSize += 1 + domainLen + 2
		case 0x04: // IPv6
			if n < headerSize+16+2 {
				continue
			}
			ip := net.IP(buffer[headerSize : headerSize+16])
			dstAddr = ip.String()
			dstPort = binary.BigEndian.Uint16(buffer[headerSize+16 : headerSize+18])
			headerSize += 18
		default:
			continue
		}

		sessionKey := clientAddr.String()
		h.sessionsLock.Lock()
		session, exists := h.sessions[sessionKey]
		if !exists {
			targetAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", dstAddr, dstPort))
			if err != nil {
				h.sessionsLock.Unlock()
				continue
			}

			targetConn, err := net.DialUDP("udp", nil, targetAddr)
			if err != nil {
				h.sessionsLock.Unlock()
				continue
			}

			session = &UDPSession{
				clientAddr: clientAddr,
				targetConn: targetConn,
				lastActive: time.Now(),
			}
			h.sessions[sessionKey] = session

			// 启动目标数据读取协程
			go h.handleTargetData(session)
		}
		session.lastActive = time.Now()
		h.sessionsLock.Unlock()

		// 转发数据到目标地址
		_, err = session.targetConn.Write(buffer[headerSize:n])
		if err != nil {
			log.Printf("转发UDP数据失败: %v", err)
		}
	}
}

// handleTargetData 处理来自目标的数据
func (h *UDPHandler) handleTargetData(session *UDPSession) {
	buffer := make([]byte, h.config.UDP.BufferSize)
	for {
		n, _, err := session.targetConn.ReadFromUDP(buffer[4:])
		if err != nil {
			return
		}

		// 构建响应头
		// RSV(2) + FRAG(1) + ATYP(1) = 4 bytes
		copy(buffer[0:4], []byte{0, 0, 0, 0x01})

		// 发送数据到客户端
		_, err = h.listener.WriteToUDP(buffer[:n+4], session.clientAddr)
		if err != nil {
			log.Printf("发送UDP响应失败: %v", err)
			return
		}

		h.sessionsLock.Lock()
		session.lastActive = time.Now()
		h.sessionsLock.Unlock()
	}
}

// Stop 停止UDP处理器
func (h *UDPHandler) Stop() {
	if h.listener != nil {
		h.listener.Close()
	}

	h.sessionsLock.Lock()
	for _, session := range h.sessions {
		session.targetConn.Close()
	}
	h.sessions = make(map[string]*UDPSession)
	h.sessionsLock.Unlock()
}