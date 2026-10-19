package ipmsg

import (
	"bytes"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// encodeText converts a Go (UTF-8) string into the wire encoding.
//
// Chinese IPMSG clients (飞鸽传书 / 飞秋) use the local code page (GBK) by
// default. When the peer advertised UTF-8 capability we may talk UTF-8.
func encodeText(s string, utf8Mode bool) []byte {
	if utf8Mode {
		return []byte(s)
	}
	out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(s))
	if err != nil {
		return []byte(s)
	}
	return out
}

// decodeText converts wire bytes into a Go (UTF-8) string.
func decodeText(b []byte, utf8Mode bool) string {
	if utf8Mode {
		if !utf8.Valid(b) {
			// Peer claimed UTF-8 but sent something else; fall back gracefully.
			return string(b)
		}
		return string(b)
	}
	out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), b)
	if err != nil {
		return string(b)
	}
	return string(out)
}

// encodeFileName converts a file name for the wire. It also escapes ':' as
// "::" as required by the IPMSG file-info format.
func encodeFileName(name string, utf8Mode bool) []byte {
	escaped := escapeColon(name)
	return encodeText(escaped, utf8Mode)
}

// decodeFileName is the inverse of encodeFileName.
func decodeFileName(b []byte, utf8Mode bool) string {
	return decodeText(b, utf8Mode)
}

// escapeColon escapes every ':' as "::" so file names survive the
// colon-delimited file-info format.
func escapeColon(s string) string {
	if !bytes.ContainsRune([]byte(s), ':') {
		return s
	}
	var b bytes.Buffer
	for _, r := range s {
		if r == ':' {
			b.WriteString("::")
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitColon splits s on ':' but treats "::" as a literal ':'. This is the
// inverse of escapeColon. It works at the byte level so that non-UTF-8 wire
// encodings (GBK) are preserved; ':' (0x3a) can never appear inside a GBK
// multi-byte character.
func splitColon(s string) []string {
	b := []byte(s)
	out := make([]string, 0, 6)
	var cur []byte
	for i := 0; i < len(b); i++ {
		if b[i] == ':' {
			if i+1 < len(b) && b[i+1] == ':' {
				cur = append(cur, ':')
				i++
				continue
			}
			out = append(out, string(cur))
			cur = cur[:0]
			continue
		}
		cur = append(cur, b[i])
	}
	out = append(out, string(cur))
	return out
}
