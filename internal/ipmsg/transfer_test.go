package ipmsg

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startTestEngine(t *testing.T, port int) *Engine {
	t.Helper()
	dir := t.TempDir()
	e := New(Config{
		Name:        "tester",
		Host:        "host",
		Port:        port,
		TCPPort:     freePort(t),
		DownloadDir: dir,
		AutoReceive: true,
	}, Callbacks{})
	if err := e.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(e.Stop)
	return e
}

func TestServeFileData(t *testing.T) {
	port := freePort(t)
	e := startTestEngine(t, port)

	src := filepath.Join(t.TempDir(), "hello.txt")
	content := bytes.Repeat([]byte("abcdefghij"), 1000)
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	packetNo, fileID := uint32(42), uint32(7)
	e.mu.Lock()
	e.outgoing[outKey(packetNo, fileID)] = &outgoing{
		path: src, isDir: false,
		info: FileInfo{ID: fileID, Name: "hello.txt", Size: uint64(len(content)), Attr: FileRegular},
	}
	e.mu.Unlock()

	got := request(t, port, CmdGetFileData, packetNo, fileID, 0)
	if !bytes.Equal(got, content) {
		t.Fatalf("payload mismatch: got %d bytes want %d", len(got), len(content))
	}
}

func TestServeFileDataOffset(t *testing.T) {
	port := freePort(t)
	e := startTestEngine(t, port)
	src := filepath.Join(t.TempDir(), "data.bin")
	content := []byte("0123456789ABCDEF")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	packetNo, fileID := uint32(1), uint32(2)
	e.mu.Lock()
	e.outgoing[outKey(packetNo, fileID)] = &outgoing{path: src, info: FileInfo{Name: "data.bin", Size: 16, Attr: FileRegular}}
	e.mu.Unlock()

	got := request(t, port, CmdGetFileData, packetNo, fileID, 8)
	if string(got) != "89ABCDEF" {
		t.Fatalf("offset payload = %q", got)
	}
}

func TestServeDirFiles(t *testing.T) {
	port := freePort(t)
	e := startTestEngine(t, port)

	src := t.TempDir()
	root := filepath.Join(src, "pkg")
	mustMkdir(t, filepath.Join(root, "sub"))
	mustWrite(t, filepath.Join(root, "readme.txt"), "top level")
	mustWrite(t, filepath.Join(root, "sub", "nested.txt"), "nested data")

	packetNo, fileID := uint32(100), uint32(200)
	e.mu.Lock()
	e.outgoing[outKey(packetNo, fileID)] = &outgoing{
		path: root, isDir: true, utf8: true,
		info: FileInfo{ID: fileID, Name: "pkg", Attr: FileDir},
	}
	e.mu.Unlock()

	raw := request(t, port, CmdGetDirFiles, packetNo, fileID, 0)

	dst := t.TempDir()
	if err := recvDirStream(bufio.NewReader(bytes.NewReader(raw)), dst, "pkg", true, func(uint64) {}); err != nil {
		t.Fatalf("recvDirStream: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "pkg", "readme.txt")); string(got) != "top level" {
		t.Errorf("readme = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "pkg", "sub", "nested.txt")); string(got) != "nested data" {
		t.Errorf("nested = %q", got)
	}
}

// request opens a TCP connection to the engine, sends a file request and
// returns everything the server writes back.
func request(t *testing.T, port int, cmd int, packetNo, fileID uint32, offset uint64) []byte {
	t.Helper()
	var extra string
	if cmd == CmdGetDirFiles {
		extra = fmt.Sprintf("%x:%x", packetNo, fileID)
	} else {
		extra = fmt.Sprintf("%x:%x:%x", packetNo, fileID, offset)
	}
	req := &Packet{Version: Version, PacketNo: 1, Name: "peer", Host: "peerhost", Command: cmd, Extra: []byte(extra)}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write(req.Encode()); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	data, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return data
}
