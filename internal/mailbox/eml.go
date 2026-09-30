package mailbox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

type emlReader struct {
	root string
	files []string
	index int
}

func newEMLReader(path string) (*emlReader, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	r := &emlReader{root: path}
	if !info.IsDir() {
		r.files = []string{path}
		r.root = filepath.Dir(path)
		return r, nil
	}
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".eml") {
			r.files = append(r.files, p)
		}
		return nil
	})
	return r, err
}

func (r *emlReader) Next() (*model.Message, error) {
	if r.index >= len(r.files) {
		return nil, io.EOF
	}
	p := r.files[r.index]
	r.index++
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(r.root, filepath.Dir(p))
	return &model.Message{Raw: raw, Folder: cleanFolder(rel)}, nil
}

func (r *emlReader) Close() error { return nil }

type emlWriter struct {
	path string
	fileMode bool
	count int
}

func newEMLWriter(path string, overwrite bool) (*emlWriter, error) {
	w := &emlWriter{path: path, fileMode: strings.EqualFold(filepath.Ext(path), ".eml")}
	if _, err := os.Stat(path); err == nil && !overwrite {
		return nil, fmt.Errorf("output exists: %s (use -overwrite)", path)
	}
	if !w.fileMode {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, err
		}
	}
	return w, nil
}

func (w *emlWriter) Write(msg *model.Message) error {
	raw, err := msg.RFC822()
	if err != nil {
		return err
	}
	w.count++
	if w.fileMode {
		if w.count > 1 {
			return fmt.Errorf("an .eml output file can contain only one message; use a directory for multiple messages")
		}
		return os.WriteFile(w.path, raw, 0o644)
	}
	dir := filepath.Join(w.path, filepath.FromSlash(cleanFolder(msg.Folder)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%06d.eml", w.count)), raw, 0o644)
}

func (w *emlWriter) Close() error { return nil }
