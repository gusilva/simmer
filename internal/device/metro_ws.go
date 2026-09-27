package device

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

// defaultMetroPort is Metro's default dev-server port. It is the only port
// we can assume when triggering the dev menu on a physical iOS device — we
// have no per-app state tracking a custom port set via "configure bundler
// location".
const defaultMetroPort = "8081"

// triggerDevMenuViaMetro sends a "devMenu" broadcast over Metro's dev-server
// WebSocket (ws://localhost:port/message) — the same mechanism the `d` key
// in `react-native start`'s terminal uses (see
// @react-native-community/cli-server-api's createMessageSocketEndpoint,
// which forwards any message with no id/target to every connected app as a
// broadcast). Unlike a shake gesture, which posts an in-process
// notification unreachable from outside the app, this reaches the app over
// its existing debug connection to Metro regardless of device kind — it is
// the only trigger that works on a physical iOS device, where there is no
// UI-automation or notification-based alternative.
//
// We connect to "localhost" because this process always runs on the same
// machine as Metro; that holds even when a physical device itself reaches
// Metro via a LAN IP.
func triggerDevMenuViaMetro(ctx context.Context, port string) error {
	conn, err := dialMetroMessageSocket(ctx, port)
	if err != nil {
		return err
	}
	defer conn.Close()
	return writeWSTextFrame(conn, `{"version":2,"method":"devMenu"}`)
}

// dialMetroMessageSocket performs a minimal RFC 6455 client handshake
// against Metro's "/message" WebSocket endpoint. No third-party WebSocket
// library is used — a client-side handshake plus a single masked text frame
// (see writeWSTextFrame) is small enough to hand-roll and this is the only
// place in simmer that needs one.
func dialMetroMessageSocket(ctx context.Context, port string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", "localhost:"+port)
	if err != nil {
		return nil, fmt.Errorf("connect to metro on :%s: %w", port, err)
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	req := "GET /message HTTP/1.1\r\n" +
		"Host: localhost:" + port + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read handshake response: %w", err)
	}
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		return nil, fmt.Errorf("unexpected handshake response: %s", strings.TrimSpace(statusLine))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("read handshake headers: %w", err)
		}
		if strings.TrimRight(line, "\r\n") == "" {
			break
		}
	}
	return conn, nil
}

// writeWSTextFrame writes payload as a single, final, masked WebSocket text
// frame. Client-to-server frames must be masked per RFC 6455 §5.3.
func writeWSTextFrame(conn net.Conn, payload string) error {
	data := []byte(payload)
	length := len(data)

	frame := []byte{0x81} // FIN=1, opcode=text
	switch {
	case length <= 125:
		frame = append(frame, 0x80|byte(length)) // MASK=1, length
	case length <= 65535:
		frame = append(frame, 0x80|126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(length))
		frame = append(frame, ext[:]...)
	default:
		frame = append(frame, 0x80|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(length))
		frame = append(frame, ext[:]...)
	}

	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	frame = append(frame, mask[:]...)

	masked := make([]byte, length)
	for i, b := range data {
		masked[i] = b ^ mask[i%4]
	}
	frame = append(frame, masked...)

	_, err := conn.Write(frame)
	return err
}
