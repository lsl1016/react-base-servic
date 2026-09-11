package react

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWSConnReadMessageDistinguishesCloseFrameAndEchoesIt(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := newTestWSConn(server)
	payload := make([]byte, 2+len("leaving"))
	binary.BigEndian.PutUint16(payload[:2], 1000)
	copy(payload[2:], "leaving")
	maskedFrame := maskedClientFrame(0x88, payload)

	errCh := make(chan error, 1)
	go func() {
		_, err := conn.ReadMessage()
		errCh <- err
	}()

	if _, err := client.Write(maskedFrame); err != nil {
		t.Fatalf("write close frame: %v", err)
	}
	echo := make([]byte, 2+len(payload))
	if _, err := io.ReadFull(client, echo); err != nil {
		t.Fatalf("read close echo: %v", err)
	}
	if echo[0] != 0x88 || int(echo[1]) != len(payload) || string(echo[2:]) != string(payload) {
		t.Fatalf("unexpected close echo: %v", echo)
	}

	var readErr *WSReadEndError
	if err := <-errCh; !errors.As(err, &readErr) {
		t.Fatalf("expected WSReadEndError, got %T: %v", err, err)
	}
	if readErr.Kind != WSReadEndCloseFrame || readErr.CloseCode != 1000 || readErr.CloseReason != "leaving" {
		t.Fatalf("unexpected close details: %+v", readErr)
	}
	if got := conn.Diagnostics().TerminationCause; got != "read_close_frame" {
		t.Fatalf("termination cause = %q", got)
	}
}

func TestWSConnReadMessageReassemblesFragmentedTextWithInterleavedPing(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := newTestWSConn(server)
	payload := []byte(`{"type":"run","sessionId":"session_1","payload":{"userPrompt":"line 1\\nline 2"}}`)
	splitAt := len(payload) / 2

	resultCh := make(chan struct {
		msgType   string
		sessionID string
		err       error
	}, 1)
	go func() {
		msg, err := conn.ReadMessage()
		resultCh <- struct {
			msgType   string
			sessionID string
			err       error
		}{msgType: msg.Type, sessionID: msg.SessionID, err: err}
	}()

	if _, err := client.Write(maskedClientFrame(0x01, payload[:splitAt])); err != nil {
		t.Fatalf("write first fragment: %v", err)
	}
	if _, err := client.Write(maskedClientFrame(0x89, []byte("alive"))); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	pong := make([]byte, 2+len("alive"))
	if _, err := io.ReadFull(client, pong); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if pong[0] != 0x8A || string(pong[2:]) != "alive" {
		t.Fatalf("unexpected pong: %v", pong)
	}
	if _, err := client.Write(maskedClientFrame(0x80, payload[splitAt:])); err != nil {
		t.Fatalf("write final fragment: %v", err)
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("read fragmented message: %v", result.err)
	}
	if result.msgType != "run" || result.sessionID != "session_1" {
		t.Fatalf("unexpected message: type=%q sessionId=%q", result.msgType, result.sessionID)
	}
}

func TestWSConnReadMessageReassemblesLargeFragmentedText(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := newTestWSConn(server)
	userPrompt := strings.Repeat("x", 70*1024)
	payload := []byte(`{"type":"run","sessionId":"session_large","payload":{"userPrompt":"` + userPrompt + `"}}`)
	splitAt := len(payload) / 2

	resultCh := make(chan struct {
		sessionID string
		err       error
	}, 1)
	go func() {
		msg, err := conn.ReadMessage()
		resultCh <- struct {
			sessionID string
			err       error
		}{sessionID: msg.SessionID, err: err}
	}()

	if _, err := client.Write(maskedClientFrame(0x01, payload[:splitAt])); err != nil {
		t.Fatalf("write first large fragment: %v", err)
	}
	if _, err := client.Write(maskedClientFrame(0x80, payload[splitAt:])); err != nil {
		t.Fatalf("write final large fragment: %v", err)
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("read large fragmented message: %v", result.err)
	}
	if result.sessionID != "session_large" {
		t.Fatalf("unexpected sessionId: %q", result.sessionID)
	}
}

func TestWSConnReadMessageRejectsUnexpectedContinuation(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := newTestWSConn(server)
	errCh := make(chan error, 1)
	go func() {
		_, err := conn.ReadMessage()
		errCh <- err
	}()

	if _, err := client.Write(maskedClientFrame(0x80, []byte(`{"type":"run"}`))); err != nil {
		t.Fatalf("write continuation: %v", err)
	}
	if err := <-errCh; err == nil || err.Error() != "unexpected websocket continuation frame" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSConnReadMessageClassifiesTCPEOF(t *testing.T) {
	server, client := tcpConnPair(t)
	conn := newTestWSConn(server)
	_ = client.Close()
	defer server.Close()

	_, err := conn.ReadMessage()
	assertWSReadEnd(t, err, WSReadEndTCPEOF, "read_header")
}

func TestWSConnReadMessageClassifiesUnexpectedEOFDuringPayload(t *testing.T) {
	server, client := tcpConnPair(t)
	conn := newTestWSConn(server)
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := conn.ReadMessage()
		errCh <- err
	}()
	if _, err := client.Write([]byte{0x81, 0x04, 'o', 'k'}); err != nil {
		t.Fatalf("write partial frame: %v", err)
	}
	_ = client.Close()
	assertWSReadEnd(t, <-errCh, WSReadEndUnexpectedEOF, "read_payload")
}

func TestClassifyWSReadError(t *testing.T) {
	assertWSReadEnd(t, classifyWSReadError(timeoutError{}, "read_header"), WSReadEndTimeout, "read_header")
	assertWSReadEnd(t, classifyWSReadError(syscall.ECONNRESET, "read_payload"), WSReadEndTCPReset, "read_payload")
}

func TestWSConnDiagnosticsKeepsFirstTerminationAndCloseCause(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	conn := newTestWSConn(server)

	conn.recordTerminationCause("read_tcp_eof")
	_ = conn.CloseWithCause("handler_exit")
	_ = conn.CloseWithCause("later_close")
	diagnostics := conn.Diagnostics()
	if diagnostics.TerminationCause != "read_tcp_eof" || diagnostics.CloseCause != "handler_exit" {
		t.Fatalf("unexpected causes: %+v", diagnostics)
	}
}

func newTestWSConn(conn net.Conn) *WSConn {
	now := time.Now()
	return &WSConn{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		writer:    bufio.NewWriter(conn),
		createdAt: now,
	}
}

func tcpConnPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		t.Fatalf("dial: %v", err)
	}
	defer listener.Close()
	select {
	case server := <-accepted:
		return server, client
	case err := <-acceptErr:
		_ = client.Close()
		t.Fatalf("accept: %v", err)
	case <-time.After(time.Second):
		_ = client.Close()
		t.Fatal("accept timed out")
	}
	return nil, nil
}

func maskedClientFrame(firstByte byte, payload []byte) []byte {
	mask := [4]byte{1, 2, 3, 4}
	frame := []byte{firstByte}
	switch payloadLen := len(payload); {
	case payloadLen < 126:
		frame = append(frame, 0x80|byte(payloadLen))
	case payloadLen <= 65535:
		frame = append(frame, 0x80|126, byte(payloadLen>>8), byte(payloadLen))
	default:
		frame = append(frame, 0x80|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(payloadLen))
		frame = append(frame, ext[:]...)
	}
	frame = append(frame, mask[:]...)
	for index, value := range payload {
		frame = append(frame, value^mask[index%len(mask)])
	}
	return frame
}

func assertWSReadEnd(t *testing.T, err error, kind WSReadEndKind, stage string) {
	t.Helper()
	var readErr *WSReadEndError
	if !errors.As(err, &readErr) {
		t.Fatalf("expected WSReadEndError, got %T: %v", err, err)
	}
	if readErr.Kind != kind || readErr.Stage != stage {
		t.Fatalf("unexpected read end: %+v", readErr)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
