package react

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"react-base-service/components/params"

	"github.com/gin-gonic/gin"
)

const (
	websocketGUID    = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	maxWSMessageSize = 8 * 1024 * 1024
)

// WSPingInterval 服务端协议层 ping 的发送间隔；浏览器收到 ping 会自动回 pong，前端无需改动。
// wsIOTimeout 是单次读/写的超时：读侧依赖 pong 持续刷新，静默断连最迟 wsIOTimeout 内被发现；
// 必须满足 wsIOTimeout > WSPingInterval，当前取值允许连丢三个 pong。声明为 var 仅为测试可注入。
var (
	WSPingInterval = 5 * time.Second
	wsIOTimeout    = 20 * time.Second
)

// WSConn 是 ReAct WebSocket 的轻量封装，只处理文本 JSON 帧和事件写回。
type WSConn struct {
	conn             net.Conn
	reader           *bufio.Reader
	writer           *bufio.Writer
	mu               sync.Mutex
	createdAt        time.Time
	lastReadAtUnixMs atomic.Int64
	lastWriteUnixMs  atomic.Int64
	lastPongUnixMs   atomic.Int64
	stateMu          sync.Mutex
	terminationCause string
	closeCause       string
}

type wsFrame struct {
	fin     bool
	opcode  byte
	payload []byte
}

type WSReadEndKind string

const (
	WSReadEndCloseFrame    WSReadEndKind = "close_frame"
	WSReadEndTCPEOF        WSReadEndKind = "tcp_eof"
	WSReadEndTCPReset      WSReadEndKind = "tcp_reset"
	WSReadEndTimeout       WSReadEndKind = "read_timeout"
	WSReadEndUnexpectedEOF WSReadEndKind = "unexpected_eof"
	WSReadEndLocalClose    WSReadEndKind = "local_close"
	WSReadEndOther         WSReadEndKind = "read_error"
)

// WSReadEndError 保留连接读取终止的协议层/传输层原因，避免把 Close 帧和 TCP EOF 都压成 io.EOF。
type WSReadEndError struct {
	Kind        WSReadEndKind
	Stage       string
	CloseCode   uint16
	CloseReason string
	Err         error
}

func (e *WSReadEndError) Error() string {
	if e.Kind == WSReadEndCloseFrame {
		return fmt.Sprintf("websocket close frame: code=%d reason=%q", e.CloseCode, e.CloseReason)
	}
	if e.Err != nil {
		return fmt.Sprintf("websocket read ended: kind=%s stage=%s err=%v", e.Kind, e.Stage, e.Err)
	}
	return fmt.Sprintf("websocket read ended: kind=%s stage=%s", e.Kind, e.Stage)
}

func (e *WSReadEndError) Unwrap() error {
	return e.Err
}

// WSDiagnostics 是单条连接日志需要的时序信息，不包含消息正文或鉴权数据。
type WSDiagnostics struct {
	ConnectionAgeMs  int64  `json:"connectionAgeMs"`
	LastReadAgoMs    *int64 `json:"lastReadAgoMs"`
	LastWriteAgoMs   *int64 `json:"lastWriteAgoMs"`
	LastPongAgoMs    *int64 `json:"lastPongAgoMs"`
	RemoteAddr       string `json:"remoteAddr,omitempty"`
	TerminationCause string `json:"terminationCause,omitempty"`
	CloseCause       string `json:"closeCause,omitempty"`
}

// Upgrade 手动完成 WebSocket 握手，并接管底层 TCP 连接用于后续双向事件通信。
func Upgrade(ctx *gin.Context) (*WSConn, error) {
	key := strings.TrimSpace(ctx.GetHeader("Sec-WebSocket-Key"))
	if key == "" || !strings.EqualFold(ctx.GetHeader("Upgrade"), "websocket") {
		return nil, fmt.Errorf("invalid websocket upgrade request")
	}

	hijacker, ok := ctx.Writer.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("response writer does not support hijack")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	accept := computeWebSocketAccept(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	now := time.Now()
	wsConn := &WSConn{conn: conn, reader: rw.Reader, writer: rw.Writer, createdAt: now}
	wsConn.lastWriteUnixMs.Store(now.UnixMilli())
	return wsConn, nil
}

// computeWebSocketAccept 按 RFC6455 规则计算握手响应头 Sec-WebSocket-Accept。
func computeWebSocketAccept(key string) string {
	h := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(h[:])
}

// Close 兼容旧调用；诊断路径应使用 CloseWithCause 保留主动关闭原因。
func (c *WSConn) Close() error {
	return c.CloseWithCause("unspecified")
}

// CloseWithCause 记录第一个主动关闭原因后关闭底层 TCP 连接。
func (c *WSConn) CloseWithCause(cause string) error {
	c.stateMu.Lock()
	if c.terminationCause == "" {
		c.terminationCause = cause
	}
	if c.closeCause == "" {
		c.closeCause = cause
	}
	c.stateMu.Unlock()
	return c.conn.Close()
}

// Diagnostics 返回连接当前时序快照，用于关联断连两端日志。
func (c *WSConn) Diagnostics() WSDiagnostics {
	now := time.Now()
	diagnostics := WSDiagnostics{
		ConnectionAgeMs: now.Sub(c.createdAt).Milliseconds(),
	}
	if remoteAddr := c.conn.RemoteAddr(); remoteAddr != nil {
		diagnostics.RemoteAddr = remoteAddr.String()
	}
	diagnostics.LastReadAgoMs = durationSinceUnixMs(now, c.lastReadAtUnixMs.Load())
	diagnostics.LastWriteAgoMs = durationSinceUnixMs(now, c.lastWriteUnixMs.Load())
	diagnostics.LastPongAgoMs = durationSinceUnixMs(now, c.lastPongUnixMs.Load())
	c.stateMu.Lock()
	diagnostics.TerminationCause = c.terminationCause
	diagnostics.CloseCause = c.closeCause
	c.stateMu.Unlock()
	return diagnostics
}

// ReadMessage 读取并重组客户端文本消息，再解析成 ReAct WebSocket 消息。
// ping/pong/close 控制帧可穿插在分片消息之间：ping 按协议回 pong，pong 仅刷新读状态。
func (c *WSConn) ReadMessage() (params.ReactWSMessage, error) {
	var messagePayload []byte
	fragmented := false

	for {
		frame, err := c.readFrame()
		if err != nil {
			c.recordTerminationCause("read_" + string(readEndKind(err)))
			return params.ReactWSMessage{}, err
		}
		switch frame.opcode {
		case 0x8:
			if !frame.fin {
				return params.ReactWSMessage{}, fmt.Errorf("fragmented websocket close frame")
			}
			closeCode, closeReason := parseClosePayload(frame.payload)
			closeErr := &WSReadEndError{Kind: WSReadEndCloseFrame, Stage: "close_frame", CloseCode: closeCode, CloseReason: closeReason}
			if err := c.writeControlFrame(0x8, frame.payload); err != nil {
				closeErr.Err = fmt.Errorf("echo close frame: %w", err)
			}
			c.recordTerminationCause("read_close_frame")
			return params.ReactWSMessage{}, closeErr
		case 0x9:
			if !frame.fin {
				return params.ReactWSMessage{}, fmt.Errorf("fragmented websocket ping frame")
			}
			if err := c.writeControlFrame(0xA, frame.payload); err != nil {
				return params.ReactWSMessage{}, classifyWSReadError(err, "write_pong")
			}
		case 0xA:
			if !frame.fin {
				return params.ReactWSMessage{}, fmt.Errorf("fragmented websocket pong frame")
			}
			c.lastPongUnixMs.Store(time.Now().UnixMilli())
		case 0x1:
			if fragmented {
				return params.ReactWSMessage{}, fmt.Errorf("new websocket text frame before fragmented message completed")
			}
			messagePayload = append(messagePayload[:0], frame.payload...)
			if err := validateWSMessageSize(len(messagePayload)); err != nil {
				return params.ReactWSMessage{}, err
			}
			if !frame.fin {
				fragmented = true
				continue
			}
			return decodeReactWSMessage(messagePayload)
		case 0x0:
			if !fragmented {
				return params.ReactWSMessage{}, fmt.Errorf("unexpected websocket continuation frame")
			}
			messagePayload = append(messagePayload, frame.payload...)
			if err := validateWSMessageSize(len(messagePayload)); err != nil {
				return params.ReactWSMessage{}, err
			}
			if !frame.fin {
				continue
			}
			return decodeReactWSMessage(messagePayload)
		default:
			return params.ReactWSMessage{}, fmt.Errorf("unsupported websocket opcode: %d", frame.opcode)
		}
	}
}

func decodeReactWSMessage(payload []byte) (params.ReactWSMessage, error) {
	var msg params.ReactWSMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return msg, err
	}
	return msg, nil
}

func validateWSMessageSize(size int) error {
	if size > maxWSMessageSize {
		return fmt.Errorf("websocket message too large: %d", size)
	}
	return nil
}

// readFrame 解析一个 WebSocket 帧，处理客户端掩码和扩展长度后返回帧元信息。
// 每次等帧都会重置读超时：正常连接上 pong 至少每 WSPingInterval 到达一次，
// 静默断连（无 FIN/RST）最迟 wsIOTimeout 内在这里以 i/o timeout 报出。
func (c *WSConn) readFrame() (wsFrame, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(wsIOTimeout)); err != nil {
		return wsFrame{}, classifyWSReadError(err, "set_read_deadline")
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return wsFrame{}, classifyWSReadError(err, "read_header")
	}
	if header[0]&0x70 != 0 {
		return wsFrame{}, fmt.Errorf("unsupported websocket reserved bits: 0x%x", header[0]&0x70)
	}
	frame := wsFrame{
		fin:    header[0]&0x80 != 0,
		opcode: header[0] & 0x0f,
	}
	masked := header[1]&0x80 != 0
	payloadLen := uint64(header[1] & 0x7f)

	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return wsFrame{}, classifyWSReadError(err, "read_extended_length")
		}
		payloadLen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return wsFrame{}, classifyWSReadError(err, "read_extended_length")
		}
		payloadLen = binary.BigEndian.Uint64(ext)
	}
	if payloadLen > maxWSMessageSize {
		return wsFrame{}, fmt.Errorf("websocket frame payload too large: %d", payloadLen)
	}
	if frame.opcode >= 0x8 && (!frame.fin || payloadLen > 125) {
		return wsFrame{}, fmt.Errorf("invalid websocket control frame: fin=%t payloadLen=%d", frame.fin, payloadLen)
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.reader, maskKey[:]); err != nil {
			return wsFrame{}, classifyWSReadError(err, "read_mask")
		}
	}

	// 大 payload 单独续期，避免等帧头和收帧体共用同一个 deadline 误杀慢速上行。
	if err := c.conn.SetReadDeadline(time.Now().Add(wsIOTimeout)); err != nil {
		return wsFrame{}, classifyWSReadError(err, "set_payload_deadline")
	}
	frame.payload = make([]byte, payloadLen)
	if _, err := io.ReadFull(c.reader, frame.payload); err != nil {
		return wsFrame{}, classifyWSReadError(err, "read_payload")
	}
	if masked {
		for i := range frame.payload {
			frame.payload[i] ^= maskKey[i%4]
		}
	}
	c.lastReadAtUnixMs.Store(time.Now().UnixMilli())
	return frame, nil
}

// WriteEvent 将服务端 ReAct 事件编码为文本帧写回前端。
func (c *WSConn) WriteEvent(event params.ReactEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := c.writeTextFrame(data); err != nil {
		return fmt.Errorf("%w: %v", ErrReactClientDisconnected, err)
	}
	return nil
}

// WritePing 发送协议层 ping 帧；浏览器和标准 WS 客户端会自动回 pong，配合读超时实现断连检测。
func (c *WSConn) WritePing() error {
	if err := c.writeControlFrame(0x9, nil); err != nil {
		return fmt.Errorf("%w: %v", ErrReactClientDisconnected, err)
	}
	return nil
}

// writeTextFrame 将 JSON payload 写成服务端文本帧；服务端帧不需要 mask。
func (c *WSConn) writeTextFrame(payload []byte) error {
	return c.writeFrame(0x81, payload)
}

// writeControlFrame 写 ping/pong/close 等控制帧，opcode 只取低 4 位，FIN 恒为 1。
func (c *WSConn) writeControlFrame(opcode byte, payload []byte) error {
	return c.writeFrame(0x80|opcode, payload)
}

// writeFrame 写一个完整帧；带写超时，避免对端静默断连后写入阻塞在内核缓冲上长期持有锁。
func (c *WSConn) writeFrame(firstByte byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(wsIOTimeout)); err != nil {
		return err
	}

	header := []byte{firstByte}
	payloadLen := len(payload)
	switch {
	case payloadLen < 126:
		header = append(header, byte(payloadLen))
	case payloadLen <= 65535:
		header = append(header, 126, byte(payloadLen>>8), byte(payloadLen))
	default:
		header = append(header, 127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(payloadLen))
		header = append(header, ext[:]...)
	}

	if _, err := c.writer.Write(header); err != nil {
		return err
	}
	if _, err := c.writer.Write(payload); err != nil {
		return err
	}
	if err := c.writer.Flush(); err != nil {
		return err
	}
	c.lastWriteUnixMs.Store(time.Now().UnixMilli())
	return nil
}

func (c *WSConn) recordTerminationCause(cause string) {
	c.stateMu.Lock()
	if c.terminationCause == "" {
		c.terminationCause = cause
	}
	c.stateMu.Unlock()
}

func classifyWSReadError(err error, stage string) error {
	kind := WSReadEndOther
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
		kind = WSReadEndUnexpectedEOF
	case errors.Is(err, io.EOF):
		kind = WSReadEndTCPEOF
	case errors.Is(err, net.ErrClosed):
		kind = WSReadEndLocalClose
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.ECONNABORTED), errors.Is(err, syscall.EPIPE):
		kind = WSReadEndTCPReset
	default:
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			kind = WSReadEndTimeout
		}
	}
	return &WSReadEndError{Kind: kind, Stage: stage, Err: err}
}

func readEndKind(err error) WSReadEndKind {
	var readErr *WSReadEndError
	if errors.As(err, &readErr) {
		return readErr.Kind
	}
	return WSReadEndOther
}

func parseClosePayload(payload []byte) (uint16, string) {
	if len(payload) < 2 {
		return 0, ""
	}
	return binary.BigEndian.Uint16(payload[:2]), string(payload[2:])
}

func durationSinceUnixMs(now time.Time, timestamp int64) *int64 {
	if timestamp == 0 {
		return nil
	}
	duration := now.UnixMilli() - timestamp
	if duration < 0 {
		duration = 0
	}
	return &duration
}
