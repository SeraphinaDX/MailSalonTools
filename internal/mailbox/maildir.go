package mailbox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

type maildirReader struct {
	root string
	files []string
	index int
}

func newMaildirReader(path string) (*maildirReader, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	r := &maildirReader{root: path}
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		parent := filepath.Base(filepath.Dir(p))
		if parent == "cur" || parent == "new" {
			r.files = append(r.files, p)
		}
		return nil
	})
	return r, err
}

func (r *maildirReader) Next() (*model.Message, error) {
	if r.index >= len(r.files) {
		return nil, io.EOF
	}
	p := r.files[r.index]
	r.index++
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	container := filepath.Dir(filepath.Dir(p))
	rel, _ := filepath.Rel(r.root, container)
	folder := cleanFolder(rel)
	if strings.HasPrefix(folder, ".") {
		folder = strings.ReplaceAll(strings.TrimPrefix(folder, "."), ".", "/")
	}
	flags := ""
	if i := strings.LastIndex(filepath.Base(p), ":2,"); i >= 0 {
		flags = filepath.Base(p)[i+3:]
	}
	return &model.Message{Raw: raw, Folder: folder, Flags: flags}, nil
}

func (r *maildirReader) Close() error { return nil }

type maildirWriter struct {
	root string
	counter uint64
}

func newMaildirWriter(path string, overwrite bool) (*maildirWriter, error) {
	if _, err := os.Stat(path); err == nil && !overwrite {
		return nil, fmt.Errorf("output exists: %s (use -overwrite)", path)
	}
	w := &maildirWriter{root: path}
	if err := ensureMaildir(path); err != nil {
		return nil, err
	}
	return w, nil
}

func ensureMaildir(path string) error {
	for _, sub := range []string{"cur", "new", "tmp"} {
		if err := os.MkdirAll(filepath.Join(path, sub), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (w *maildirWriter) Write(msg *model.Message) error {
	raw, err := msg.RFC822()
	if err != nil {
		return err
	}
	dir := w.root
	if folder := cleanFolder(msg.Folder); folder != "" {
		dir = filepath.Join(w.root, filepath.FromSlash(folder))
	}
	if err := ensureMaildir(dir); err != nil {
		return err
	}
	n := atomic.AddUint64(&w.counter, 1)
	host, _ := os.Hostname()
	base := fmt.Sprintf("%d.%d_%d.%s", time.Now().UnixNano(), os.Getpid(), n, strings.ReplaceAll(host, "/", "_"))
	tmp := filepath.Join(dir, "tmp", base)
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	targetDir, name := "new", base
	if msg.Flags != "" {
		targetDir = "cur"
		name += ":2," + msg.Flags
	}
	return os.Rename(tmp, filepath.Join(dir, targetDir, name))
}

func (w *maildirWriter) Close() error { return nil }
