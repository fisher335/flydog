package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestShare(t *testing.T, dir string, allowUpload bool) (*httptest.Server, *shareServer) {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &shareServer{
		rootDir:     dir,
		root:        root,
		token:       "testtoken",
		allowUpload: allowUpload,
	}
	ts := httptest.NewServer(s)
	t.Cleanup(func() {
		ts.Close()
		_ = root.Close()
	})
	return ts, s
}

func TestShareListAndDownload(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a&b.txt"), []byte("amp"), 0o644); err != nil {
		t.Fatal(err)
	}
	ts, _ := newTestShare(t, dir, true)

	resp, err := http.Get(ts.URL + "/s/testtoken/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("listing status = %d", resp.StatusCode)
	}
	for _, want := range []string{"hello.txt", "docs", "a&amp;b.txt", "上传到当前目录"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("listing does not contain %q", want)
		}
	}

	resp, err = http.Get(ts.URL + "/s/testtoken/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "hello" {
		t.Fatalf("download = %q", body)
	}

	resp, err = http.Get(ts.URL + "/s/wrongtoken/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong token status = %d", resp.StatusCode)
	}
}

func TestShareRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	ts, _ := newTestShare(t, dir, true)
	for _, p := range []string{"/s/testtoken/../secret.txt", "/s/testtoken/%2e%2e/secret.txt"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", p, resp.StatusCode)
		}
	}
}

func uploadFile(t *testing.T, url, name, content string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("files", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestShareUploadAndConflict(t *testing.T) {
	dir := t.TempDir()
	ts, _ := newTestShare(t, dir, true)

	resp := uploadFile(t, ts.URL+"/s/testtoken/", "note.txt", "first")
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	got, err := os.ReadFile(filepath.Join(dir, "note.txt"))
	if err != nil || string(got) != "first" {
		t.Fatalf("uploaded file = %q, %v", got, err)
	}

	resp = uploadFile(t, ts.URL+"/s/testtoken/", "note.txt", "second")
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("second upload status = %d", resp.StatusCode)
	}
	got, err = os.ReadFile(filepath.Join(dir, "note (1).txt"))
	if err != nil || string(got) != "second" {
		t.Fatalf("conflict file = %q, %v", got, err)
	}
}

func TestShareReadOnly(t *testing.T) {
	dir := t.TempDir()
	ts, _ := newTestShare(t, dir, false)
	resp := uploadFile(t, ts.URL+"/s/testtoken/", "note.txt", "nope")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only upload status = %d, want 403", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(dir, "note.txt")); !os.IsNotExist(err) {
		t.Fatalf("read-only upload wrote a file")
	}
}

func TestSafeShareName(t *testing.T) {
	cases := map[string]string{
		"../../evil.txt": "evil.txt",
		"notes.txt":      "notes.txt",
		"..":             "upload.bin",
		"":               "upload.bin",
		"CON":            "_CON",
		"a:b?.txt":       "a_b_.txt",
	}
	for in, want := range cases {
		if got := safeShareName(in); got != want {
			t.Errorf("safeShareName(%q) = %q, want %q", in, got, want)
		}
	}
}
