package device

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// TestTriggerDevMenuViaMetro spins up a bare TCP server that performs the
// RFC 6455 handshake and asserts the single frame we send is a masked text
// frame carrying the exact devMenu broadcast payload — the same shape
// Metro's createMessageSocketEndpoint expects.
func TestTriggerDevMenuViaMetro(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	_, port, _ := net.SplitHostPort(ln.Addr().String())

	frameCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		requestLine, err := reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		if !strings.HasPrefix(requestLine, "GET /message ") {
			errCh <- errUnexpected(requestLine)
			return
		}
		var sawUpgrade bool
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			if strings.TrimRight(line, "\r\n") == "" {
				break
			}
			if strings.HasPrefix(line, "Upgrade: websocket") {
				sawUpgrade = true
			}
		}
		if !sawUpgrade {
			errCh <- errUnexpected("missing Upgrade header")
			return
		}

		if _, err := conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")); err != nil {
			errCh <- err
			return
		}

		header := make([]byte, 2)
		if _, err := readFull(reader, header); err != nil {
			errCh <- err
			return
		}
		payloadLen := int(header[1] & 0x7f)
		masked := header[1]&0x80 != 0

		mask := make([]byte, 4)
		if masked {
			if _, err := readFull(reader, mask); err != nil {
				errCh <- err
				return
			}
		}
		payload := make([]byte, payloadLen)
		if _, err := readFull(reader, payload); err != nil {
			errCh <- err
			return
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		frameCh <- payload
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := triggerDevMenuViaMetro(ctx, port); err != nil {
		t.Fatalf("triggerDevMenuViaMetro: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("server: %v", err)
	case payload := <-frameCh:
		want := `{"version":2,"method":"devMenu"}`
		if string(payload) != want {
			t.Errorf("payload = %q, want %q", payload, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for frame")
	}
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		b, err := r.ReadByte()
		if err != nil {
			return n, err
		}
		buf[n] = b
		n++
	}
	return n, nil
}

type errUnexpected string

func (e errUnexpected) Error() string { return string(e) }
