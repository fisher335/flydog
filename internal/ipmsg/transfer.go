package ipmsg

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SendFiles sends one or more files/folders to a peer. Paths may refer to
// regular files or directories; directories are transferred recursively.
func (e *Engine) SendFiles(ip string, port int, text string, paths []string) ([]FileInfo, error) {
	if !e.started.Load() {
		return nil, fmt.Errorf("engine not started")
	}
	member, ok := e.members.Lookup(ip, port)
	utf8Mode := ok && member.UTF8

	packetNo := e.nextPacketNo()
	var files []FileInfo
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			e.errf("cannot stat %s: %v", p, err)
			continue
		}
		fi := FileInfo{
			ID:    e.nextFileID(),
			Name:  filepath.Base(p),
			MTime: uint32(info.ModTime().Unix()),
		}
		isDir := info.IsDir()
		if isDir {
			fi.Attr = FileDir
		} else {
			fi.Attr = FileRegular
			fi.Size = uint64(info.Size())
		}
		e.mu.Lock()
		e.outgoing[outKey(packetNo, fi.ID)] = &outgoing{
			path:    p,
			info:    fi,
			isDir:   isDir,
			remote:  ip,
			utf8:    utf8Mode,
			created: time.Now(),
		}
		e.mu.Unlock()
		files = append(files, fi)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no valid files to send")
	}

	extra := encodeAttachment(text, files, utf8Mode)
	if err := e.send(ip, port, CmdSendMsg|OptFileAttach, extra, utf8Mode, packetNo); err != nil {
		return nil, err
	}
	return files, nil
}

func outKey(packetNo, fileID uint32) string {
	return strconv.FormatUint(uint64(packetNo), 10) + ":" + strconv.FormatUint(uint64(fileID), 10)
}

func (e *Engine) lookupOutgoing(packetNo, fileID uint32) *outgoing {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.outgoing[outKey(packetNo, fileID)]
}

// purgeOutgoing drops offers that were never (or no longer) requested.
func (e *Engine) purgeOutgoing(maxAge time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, o := range e.outgoing {
		if time.Since(o.created) > maxAge {
			delete(e.outgoing, k)
		}
	}
}

// ---- TCP file server ----

func (e *Engine) tcpLoop() {
	defer e.wg.Done()
	for {
		conn, err := e.tcp.Accept()
		if err != nil {
			select {
			case <-e.quit:
				return
			default:
				continue
			}
		}
		go e.serveConn(conn)
	}
}

func (e *Engine) serveConn(conn net.Conn) {
	defer conn.Close()

	// The request is a single unframed packet; don't wait forever for one.
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	pkt, err := readFileRequest(conn)
	if err != nil {
		return
	}
	parts := strings.Split(string(pkt.Extra), ":")
	if len(parts) < 2 {
		return
	}
	packetNo64, err1 := strconv.ParseUint(strings.TrimSpace(parts[0]), 16, 32)
	fileID64, err2 := strconv.ParseUint(strings.TrimSpace(parts[1]), 16, 32)
	if err1 != nil || err2 != nil {
		return
	}
	packetNo, fileID := uint32(packetNo64), uint32(fileID64)
	var offset uint64
	if len(parts) >= 3 {
		if v, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 16, 64); err == nil {
			offset = v
		}
	}
	out := e.lookupOutgoing(packetNo, fileID)
	if out == nil {
		return
	}
	// Rolling deadlines: an absolute deadline would cut off large files or
	// folders on slow links, while a per-write deadline still detects a dead
	// peer.
	w := idleTimeoutConn{Conn: conn, timeout: 2 * time.Minute}
	switch GetMode(pkt.Command) {
	case CmdGetFileData:
		if out.isDir {
			return
		}
		e.sendFileData(w, out, offset)
	case CmdGetDirFiles:
		if !out.isDir {
			// Some clients request files via GETDIRFILES too.
			e.sendFileData(w, out, offset)
			return
		}
		e.sendDirData(w, out)
	}
}

// idleTimeoutConn applies a fresh deadline to every read and write, so a slow
// but healthy transfer keeps going while a dead peer still fails quickly.
type idleTimeoutConn struct {
	net.Conn
	timeout time.Duration
}

func (c idleTimeoutConn) Read(p []byte) (int, error) {
	_ = c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
	return c.Conn.Read(p)
}

func (c idleTimeoutConn) Write(p []byte) (int, error) {
	_ = c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.Conn.Write(p)
}

// readFileRequest reads one IPMSG packet from a file-transfer connection.
// These packets are not length-framed: the header ends at the fifth ':', and
// the additional section is collected until it looks complete for the command.
func readFileRequest(conn net.Conn) (*Packet, error) {
	br := bufio.NewReaderSize(conn, 4096)
	var buf []byte
	for i := 0; i < 5; i++ {
		part, err := br.ReadBytes(HostListSeparator)
		if err != nil {
			return nil, err
		}
		buf = append(buf, part...)
		if len(buf) > MaxHeader {
			return nil, errBadPacket
		}
	}
	headLen := len(buf)
	pkt, err := ParsePacket(buf)
	if err != nil {
		return nil, err
	}
	for !fileRequestExtraComplete(pkt, buf[headLen:]) {
		if n := br.Buffered(); n > 0 {
			b, _ := br.Peek(n)
			buf = append(buf, b...)
			_, _ = br.Discard(n)
			continue
		}
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		tmp := make([]byte, 512)
		n, rerr := br.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > MaxHeader {
				return nil, errBadPacket
			}
		}
		if rerr != nil {
			break
		}
	}
	return ParsePacket(buf)
}

// fileRequestExtraComplete reports whether the additional section has all the
// fields this command needs. GETFILEDATA is "packetNo:fileID[:offset]";
// GETDIRFILES is "packetNo:fileID". Clients that omit the optional offset make
// readFileRequest wait out its short deadline instead.
func fileRequestExtraComplete(pkt *Packet, extra []byte) bool {
	switch GetMode(pkt.Command) {
	case CmdGetFileData:
		return bytes.Count(extra, []byte{HostListSeparator}) >= 2
	case CmdGetDirFiles:
		return bytes.Count(extra, []byte{HostListSeparator}) >= 1
	default:
		return true
	}
}

func (e *Engine) sendFileData(conn net.Conn, out *outgoing, offset uint64) {
	f, err := os.Open(out.path)
	if err != nil {
		e.errf("open %s: %v", out.path, err)
		return
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
			return
		}
	}
	tr := Transfer{
		ID:        fmt.Sprintf("send:%s:%d", out.remote, out.info.ID),
		Direction: "send",
		Peer:      out.remote,
		Name:      out.info.Name,
		Size:      out.info.Size,
		Done:      offset,
		State:     StateRunning,
	}
	e.emitTransfer(tr)
	buf := make([]byte, 64*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := conn.Write(buf[:n]); werr != nil {
				tr.State = StateError
				tr.Error = werr.Error()
				e.emitTransfer(tr)
				return
			}
			tr.Done += uint64(n)
			e.emitTransfer(tr)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			tr.State = StateError
			tr.Error = err.Error()
			e.emitTransfer(tr)
			return
		}
	}
	tr.Done = tr.Size
	tr.State = StateDone
	e.emitTransfer(tr)
}

// sendDirData streams a directory tree in the IPMSG hierarchical format.
func (e *Engine) sendDirData(conn net.Conn, out *outgoing) {
	w := bufio.NewWriterSize(conn, 64*1024)
	tr := Transfer{
		ID:        fmt.Sprintf("send:%s:%d", out.remote, out.info.ID),
		Direction: "send",
		Peer:      out.remote,
		Name:      out.info.Name,
		State:     StateRunning,
	}
	e.emitTransfer(tr)

	err := streamTree(w, out.path, out.info.Name, out.info.MTime, out.utf8, func(n uint64) {
		tr.Done += n
		e.emitTransfer(tr)
	})
	if err != nil {
		tr.State = StateError
		tr.Error = err.Error()
		e.emitTransfer(tr)
		return
	}
	if err := w.Flush(); err != nil {
		tr.State = StateError
		tr.Error = err.Error()
		e.emitTransfer(tr)
		return
	}
	tr.State = StateDone
	e.emitTransfer(tr)
}

// streamTree writes the top directory header, its contents and the trailing
// RETPARENT for the top directory.
func streamTree(w *bufio.Writer, path, name string, mtime uint32, utf8Mode bool, progress func(uint64)) error {
	if err := writeStr(w, encodeDirHeader(name, 0, FileDir, mtime, 0, utf8Mode)); err != nil {
		return err
	}
	if err := streamDirContents(w, path, mtime, utf8Mode, progress); err != nil {
		return err
	}
	return writeStr(w, encodeDirHeader(".", 0, FileRetParent, mtime, 0, utf8Mode))
}

// writeStr writes a byte slice to the buffered writer.
func writeStr(w *bufio.Writer, b []byte) error {
	_, err := w.Write(b)
	return err
}

// streamDirContents writes all entries of path; directories are entered and
// followed by their RETPARENT. It does not write the RETPARENT for path itself.
func streamDirContents(w *bufio.Writer, path string, dirMtime uint32, utf8Mode bool, progress func(uint64)) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		child := filepath.Join(path, ent.Name())
		info, err := ent.Info()
		if err != nil {
			continue
		}
		mtime := uint32(info.ModTime().Unix())
		if ent.IsDir() {
			if err := writeStr(w, encodeDirHeader(ent.Name(), 0, FileDir, mtime, 0, utf8Mode)); err != nil {
				return err
			}
			if err := streamDirContents(w, child, mtime, utf8Mode, progress); err != nil {
				return err
			}
			if err := writeStr(w, encodeDirHeader(".", 0, FileRetParent, mtime, 0, utf8Mode)); err != nil {
				return err
			}
			continue
		}
		size := uint64(info.Size())
		if err := writeStr(w, encodeDirHeader(ent.Name(), size, FileRegular, mtime, 0, utf8Mode)); err != nil {
			return err
		}
		if err := copyFileTo(w, child, progress); err != nil {
			return err
		}
	}
	return nil
}

// copyFileTo copies a whole file into w without reading it all into memory.
func copyFileTo(w *bufio.Writer, path string, progress func(uint64)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 64*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			if progress != nil {
				progress(uint64(n))
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// ---- TCP client (receiving) ----

func (e *Engine) receiveFiles(peer Member, packetNo uint32, files []FileInfo, utf8Mode bool) {
	base := e.cfg.DownloadDir
	if base == "" {
		base = "."
	}
	dest := filepath.Join(base, safeName(peer.Name))
	for _, fi := range files {
		fi := fi
		go e.receiveOne(peer, packetNo, fi, dest, utf8Mode)
	}
}

func (e *Engine) receiveOne(peer Member, packetNo uint32, fi FileInfo, dest string, utf8Mode bool) {
	port := peer.Port
	if port == 0 {
		port = Port
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(peer.IP, strconv.Itoa(port)), 10*time.Second)
	if err != nil {
		e.errf("connect to %s: %v", peer.IP, err)
		e.emitTransfer(Transfer{
			ID:        fmt.Sprintf("recv:%x:%x", packetNo, fi.ID),
			Direction: "recv", Peer: peer.IP, Name: fi.Name,
			Size: fi.Size, State: StateError, Error: err.Error(),
		})
		return
	}
	defer conn.Close()
	conn = idleTimeoutConn{Conn: conn, timeout: 2 * time.Minute}

	cmd := CmdGetFileData
	var extra string
	if fi.IsDir() {
		cmd = CmdGetDirFiles
		extra = fmt.Sprintf("%x:%x", packetNo, fi.ID)
	} else {
		extra = fmt.Sprintf("%x:%x:%x", packetNo, fi.ID, 0)
	}
	req := &Packet{
		Version:  Version,
		PacketNo: e.nextPacketNo(),
		Name:     e.Name(),
		Host:     e.cfg.Host,
		Command:  cmd,
		Extra:    []byte(extra),
	}
	if _, err := conn.Write(req.Encode()); err != nil {
		e.errf("request file: %v", err)
		return
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		e.errf("mkdir %s: %v", dest, err)
		return
	}

	if fi.IsDir() {
		e.recvDir(conn, dest, fi, utf8Mode, packetNo)
		return
	}
	e.recvFile(conn, dest, fi, packetNo)
}

func (e *Engine) recvFile(conn net.Conn, dest string, fi FileInfo, packetNo uint32) {
	tr := Transfer{
		ID:        fmt.Sprintf("recv:%x:%x", packetNo, fi.ID),
		Direction: "recv",
		Peer:      "",
		Name:      fi.Name,
		Size:      fi.Size,
		State:     StateRunning,
	}
	e.emitTransfer(tr)

	target := filepath.Join(dest, safeName(fi.Name))
	f, err := createUnique(target)
	if err != nil {
		tr.State = StateError
		tr.Error = err.Error()
		e.emitTransfer(tr)
		return
	}
	defer f.Close()

	remaining := int64(fi.Size)
	buf := make([]byte, 64*1024)
	for remaining > 0 {
		want := int64(len(buf))
		if want > remaining {
			want = remaining
		}
		n, err := conn.Read(buf[:want])
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				tr.State = StateError
				tr.Error = werr.Error()
				e.emitTransfer(tr)
				return
			}
			remaining -= int64(n)
			tr.Done += uint64(n)
			e.emitTransfer(tr)
		}
		if err != nil {
			tr.State = StateError
			tr.Error = err.Error()
			e.emitTransfer(tr)
			return
		}
		if n == 0 {
			// Read returning (0, nil) would spin this loop forever; a healthy
			// connection reports either data or an error, so treat it as closed.
			tr.State = StateError
			tr.Error = "连接已关闭"
			e.emitTransfer(tr)
			return
		}
	}
	tr.Done = tr.Size
	tr.State = StateDone
	e.emitTransfer(tr)
}

// recvDir parses the hierarchical stream and writes it below dest.
func (e *Engine) recvDir(conn net.Conn, dest string, fi FileInfo, utf8Mode bool, packetNo uint32) {
	tr := Transfer{
		ID:        fmt.Sprintf("recv:%x:%x", packetNo, fi.ID),
		Direction: "recv",
		Name:      fi.Name,
		State:     StateRunning,
	}
	e.emitTransfer(tr)

	err := recvDirStream(bufio.NewReaderSize(conn, 64*1024), dest, fi.Name, utf8Mode, func(n uint64) {
		tr.Done += n
		e.emitTransfer(tr)
	})
	if err != nil {
		tr.State = StateError
		tr.Error = err.Error()
		e.emitTransfer(tr)
		return
	}
	tr.State = StateDone
	e.emitTransfer(tr)
}

// recvDirStream decodes the IPMSG hierarchical file stream below dest.
func recvDirStream(r *bufio.Reader, dest, topName string, utf8Mode bool, progress func(uint64)) error {
	path := dest
	dirCnt := 0
	for {
		hdr, err := decodeDirHeader(r, utf8Mode)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch hdr.Mode() {
		case FileDir:
			name := hdr.Name
			if dirCnt == 0 {
				name = topName
			}
			path = filepath.Join(path, safeName(name))
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			dirCnt++
		case FileRetParent:
			dirCnt--
			if dirCnt <= 0 {
				return nil
			}
			path = filepath.Dir(path)
		default:
			if dirCnt == 0 {
				continue
			}
			target := filepath.Join(path, safeName(hdr.Name))
			if err := writeN(r, target, int64(hdr.Size)); err != nil {
				return err
			}
			if progress != nil {
				progress(hdr.Size)
			}
		}
	}
}

func writeN(r io.Reader, path string, n int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := createUnique(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if n <= 0 {
		return nil
	}
	_, err = io.CopyN(f, r, n)
	return err
}

// createUnique opens path for writing, inserting " (n)" before the extension
// when the name is already taken, so an incoming transfer never silently
// overwrites an earlier one. O_EXCL makes the check atomic.
func createUnique(path string) (*os.File, error) {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 0; i < 10000; i++ {
		p := path
		if i > 0 {
			p = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("无法创建文件（名称冲突过多）: %s", path)
}

func safeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	for _, c := range []string{":", "*", "?", "\"", "<", ">", "|", "\x00"} {
		name = strings.ReplaceAll(name, c, "_")
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		name = "_"
	}
	return name
}
