package ipmsg

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// FileInfo describes one attachment entry.
type FileInfo struct {
	ID     uint32
	Name   string
	Size   uint64
	MTime  uint32
	Attr   uint32
	Crtime uint32
}

// Mode returns the file type (regular/dir/retparent/...).
func (f FileInfo) Mode() int { return int(f.Attr & 0xff) }

// IsDir reports whether the entry is a directory.
func (f FileInfo) IsDir() bool { return f.Mode() == FileDir }

// IsRetParent reports whether the entry ends a directory listing.
func (f FileInfo) IsRetParent() bool { return f.Mode() == FileRetParent }

// encodeAttachmentEntry renders "id:name:size:mtime:attr:" + FILELIST_SEPARATOR.
func encodeAttachmentEntry(fi FileInfo, utf8Mode bool) []byte {
	name := encodeFileName(fi.Name, utf8Mode)
	s := fmt.Sprintf("%d:%s:%x:%x:%x:%c",
		fi.ID, name, fi.Size, fi.MTime, fi.Attr, byte(FileListSeparator))
	return []byte(s)
}

// encodeAttachment builds the extra section for a SENDMSG|FILEATTACHOPT packet.
func encodeAttachment(text string, files []FileInfo, utf8Mode bool) []byte {
	out := encodeText(text, utf8Mode)
	if len(files) == 0 {
		return out
	}
	out = append(out, 0)
	for _, f := range files {
		out = append(out, encodeAttachmentEntry(f, utf8Mode)...)
	}
	return out
}

// decodeAttachment parses the extra section of a message. It returns the plain
// text and any attached files.
func decodeAttachment(extra []byte, utf8Mode bool) (string, []FileInfo) {
	if len(extra) == 0 {
		return "", nil
	}
	i := bytes.IndexByte(extra, 0)
	if i < 0 {
		return decodeText(extra, utf8Mode), nil
	}
	text := decodeText(extra[:i], utf8Mode)
	list := extra[i+1:]
	var files []FileInfo
	for _, entry := range bytes.Split(list, []byte{FileListSeparator}) {
		entry = bytes.TrimRight(entry, "\x00")
		if len(entry) == 0 {
			continue
		}
		fi, ok := parseAttachmentEntry(string(entry), utf8Mode)
		if ok {
			files = append(files, fi)
		}
	}
	return text, files
}

func parseAttachmentEntry(s string, utf8Mode bool) (FileInfo, bool) {
	parts := splitColon(s)
	if len(parts) < 5 {
		return FileInfo{}, false
	}
	id, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32)
	if err != nil {
		return FileInfo{}, false
	}
	size, err := strconv.ParseUint(parts[2], 16, 64)
	if err != nil {
		return FileInfo{}, false
	}
	mtime, err := strconv.ParseUint(parts[3], 16, 32)
	if err != nil {
		return FileInfo{}, false
	}
	attr, err := strconv.ParseUint(parts[4], 16, 32)
	if err != nil {
		return FileInfo{}, false
	}
	fi := FileInfo{
		ID:    uint32(id),
		Name:  decodeText([]byte(parts[1]), utf8Mode),
		Size:  size,
		MTime: uint32(mtime),
		Attr:  uint32(attr),
	}
	for _, ex := range parts[5:] {
		kv := strings.SplitN(ex, "=", 2)
		if len(kv) != 2 {
			continue
		}
		t, err1 := strconv.ParseUint(kv[0], 16, 32)
		v, err2 := strconv.ParseUint(kv[1], 16, 32)
		if err1 != nil || err2 != nil {
			continue
		}
		if uint32(t) == ExtAttrCreateTime {
			fi.Crtime = uint32(v)
		}
	}
	return fi, true
}

// encodeDirHeader renders one directory-stream header:
//
//	LLLL:name:size:attr:MTIME=h:CREATETIME=h:
//
// where LLLL is the total number of bytes of the header (including "LLLL:").
func encodeDirHeader(name string, size uint64, attr uint32, mtime, crtime uint32, utf8Mode bool) []byte {
	encoded := encodeFileName(name, utf8Mode)
	inner := fmt.Sprintf("%s:%x:%x:%x=%x:%x=%x:",
		encoded, size, attr,
		ExtAttrMTime, mtime, ExtAttrCreateTime, crtime)
	n := len("0000:") + len(inner)
	return append([]byte(fmt.Sprintf("%04x:", n)), inner...)
}

// decodeDirHeader reads one directory-stream header from r.
func decodeDirHeader(r *bufio.Reader, utf8Mode bool) (FileInfo, error) {
	var lenBuf [5]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return FileInfo{}, err
	}
	if lenBuf[4] != HostListSeparator {
		return FileInfo{}, errBadPacket
	}
	n, err := strconv.ParseUint(string(lenBuf[:4]), 16, 32)
	if err != nil || n < 6 || n > MaxHeader {
		return FileInfo{}, errBadPacket
	}
	body := make([]byte, int(n)-5)
	if _, err := io.ReadFull(r, body); err != nil {
		return FileInfo{}, err
	}
	parts := splitColon(string(body))
	if len(parts) < 3 {
		return FileInfo{}, errBadPacket
	}
	size, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		return FileInfo{}, errBadPacket
	}
	attr, err := strconv.ParseUint(parts[2], 16, 32)
	if err != nil {
		return FileInfo{}, errBadPacket
	}
	fi := FileInfo{
		Name: decodeText([]byte(parts[0]), utf8Mode),
		Size: size,
		Attr: uint32(attr),
	}
	for _, ex := range parts[3:] {
		kv := strings.SplitN(ex, "=", 2)
		if len(kv) != 2 {
			continue
		}
		t, err1 := strconv.ParseUint(kv[0], 16, 32)
		v, err2 := strconv.ParseUint(kv[1], 16, 32)
		if err1 != nil || err2 != nil {
			continue
		}
		switch uint32(t) {
		case ExtAttrMTime:
			fi.MTime = uint32(v)
		case ExtAttrCreateTime:
			fi.Crtime = uint32(v)
		}
	}
	return fi, nil
}
