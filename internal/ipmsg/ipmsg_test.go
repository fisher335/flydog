package ipmsg

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPacketRoundTrip(t *testing.T) {
	extra := []byte("hello\x00world\x07tail")
	p := &Packet{
		Version:  Version,
		PacketNo: 12345,
		Name:     "user:one", // the ':' must be sanitized
		Host:     "my-host",
		Command:  CmdSendMsg | OptFileAttach,
		Extra:    extra,
	}
	got, err := ParsePacket(p.Encode())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.PacketNo != p.PacketNo {
		t.Errorf("packetNo = %d, want %d", got.PacketNo, p.PacketNo)
	}
	if got.Command != p.Command {
		t.Errorf("command = %x, want %x", got.Command, p.Command)
	}
	if got.Name != "user;one" {
		t.Errorf("name = %q, want %q", got.Name, "user;one")
	}
	if !bytes.Equal(got.Extra, extra) {
		t.Errorf("extra = %q, want %q", got.Extra, extra)
	}
}

func TestAttachmentRoundTrip(t *testing.T) {
	files := []FileInfo{
		{ID: 1001, Name: "report.pdf", Size: 2048, MTime: 1600000000, Attr: FileRegular},
		{ID: 1002, Name: "a:b.txt", Size: 12, MTime: 1600000001, Attr: FileRegular},
	}
	for _, utf8Mode := range []bool{true, false} {
		extra := encodeAttachment("请看附件：报告", files, utf8Mode)
		text, got := decodeAttachment(extra, utf8Mode)
		if text != "请看附件：报告" {
			t.Errorf("utf8=%v text = %q", utf8Mode, text)
		}
		if len(got) != len(files) {
			t.Fatalf("utf8=%v got %d files, want %d", utf8Mode, len(got), len(files))
		}
		if got[1].Name != "a:b.txt" {
			t.Errorf("utf8=%v name = %q, want %q", utf8Mode, got[1].Name, "a:b.txt")
		}
		if !reflect.DeepEqual(got[0].ID, files[0].ID) || got[0].Size != files[0].Size || got[0].MTime != files[0].MTime || got[0].Attr != files[0].Attr {
			t.Errorf("utf8=%v entry = %+v", utf8Mode, got[0])
		}
	}
}

func TestPlainTextNoAttachment(t *testing.T) {
	text, files := decodeAttachment(encodeText("just text", false), false)
	if text != "just text" || len(files) != 0 {
		t.Errorf("got %q %v", text, files)
	}
}

func TestDirHeaderRoundTrip(t *testing.T) {
	hdr := encodeDirHeader("子目录", 4096, FileRegular, 1600000000, 1600000001, false)
	got, err := decodeDirHeader(bufio.NewReader(bytes.NewReader(hdr)), false)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "子目录" {
		t.Errorf("name = %q", got.Name)
	}
	if got.Size != 4096 || got.Attr != FileRegular || got.MTime != 1600000000 {
		t.Errorf("entry = %+v", got)
	}
}

func TestSplitColon(t *testing.T) {
	got := splitColon("a::b:c")
	want := []string{"a:b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitColon = %v, want %v", got, want)
	}
}

func TestDirStreamRoundTrip(t *testing.T) {
	src := t.TempDir()
	// Build: src/top/{a.txt, sub/{b.txt, deep/{c.txt}}}
	top := filepath.Join(src, "top")
	mustMkdir(t, top)
	mustWrite(t, filepath.Join(top, "a.txt"), "hello a")
	sub := filepath.Join(top, "sub")
	mustMkdir(t, sub)
	mustWrite(t, filepath.Join(sub, "b.txt"), "content of b")
	deep := filepath.Join(sub, "深 目录")
	mustMkdir(t, deep)
	mustWrite(t, filepath.Join(deep, "c.txt"), "c file data")

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := streamTree(w, top, "top", 1600000000, true, func(uint64) {}); err != nil {
		t.Fatalf("streamTree: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	dst := t.TempDir()
	if err := recvDirStream(bufio.NewReader(&buf), dst, "top", true, func(uint64) {}); err != nil {
		t.Fatalf("recvDirStream: %v", err)
	}

	for _, tc := range []struct {
		rel  string
		want string
	}{
		{"a.txt", "hello a"},
		{filepath.Join("sub", "b.txt"), "content of b"},
		{filepath.Join("sub", "深 目录", "c.txt"), "c file data"},
	} {
		got, err := os.ReadFile(filepath.Join(dst, "top", tc.rel))
		if err != nil {
			t.Fatalf("read %s: %v", tc.rel, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s = %q, want %q", tc.rel, got, tc.want)
		}
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
