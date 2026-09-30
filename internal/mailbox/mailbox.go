package mailbox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

type Format string

const (
	FormatEML Format = "eml"
	FormatMaildir Format = "maildir"
	FormatMbox Format = "mbox"
	FormatPST Format = "pst"
)

type Reader interface {
	Next() (*model.Message, error)
	Close() error
}

type Writer interface {
	Write(*model.Message) error
	Close() error
}

func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case FormatEML:
		return FormatEML, nil
	case FormatMaildir:
		return FormatMaildir, nil
	case FormatMbox:
		return FormatMbox, nil
	case FormatPST:
		return FormatPST, nil
	default:
		return "", fmt.Errorf("unsupported mailbox format %q", s)
	}
}

func DetectInput(path string) (Format, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		if isMaildir(path) {
			return FormatMaildir, nil
		}
		foundEML := false
		_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".eml") {
				foundEML = true
				return io.EOF
			}
			return nil
		})
		if foundEML {
			return FormatEML, nil
		}
		return "", fmt.Errorf("cannot detect directory format for %s", path)
	}
	return formatFromExtension(path)
}

func DetectOutput(path string) (Format, error) {
	if f, err := formatFromExtension(path); err == nil {
		return f, nil
	}
	return FormatMaildir, nil
}

func formatFromExtension(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".eml":
		return FormatEML, nil
	case ".mbox", ".mbx":
		return FormatMbox, nil
	case ".pst":
		return FormatPST, nil
	default:
		return "", fmt.Errorf("cannot detect format from %s", path)
	}
}

func OpenReader(format Format, path string) (Reader, error) {
	switch format {
	case FormatEML:
		return newEMLReader(path)
	case FormatMaildir:
		return newMaildirReader(path)
	case FormatMbox:
		return newMboxReader(path)
	case FormatPST:
		return newPSTReader(path)
	default:
		return nil, fmt.Errorf("unsupported input format %q", format)
	}
}

func OpenWriter(format Format, path string, overwrite bool) (Writer, error) {
	switch format {
	case FormatEML:
		return newEMLWriter(path, overwrite)
	case FormatMaildir:
		return newMaildirWriter(path, overwrite)
	case FormatMbox:
		return newMboxWriter(path, overwrite)
	case FormatPST:
		return newPSTWriter(path, overwrite)
	default:
		return nil, fmt.Errorf("unsupported output format %q", format)
	}
}

func isMaildir(path string) bool {
	for _, name := range []string{"cur", "new", "tmp"} {
		info, err := os.Stat(filepath.Join(path, name))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

func cleanFolder(folder string) string {
	folder = filepath.ToSlash(filepath.Clean(folder))
	if folder == "." || folder == "/" {
		return ""
	}
	folder = strings.TrimPrefix(folder, "/")
	parts := strings.Split(folder, "/")
	safe := parts[:0]
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			continue
		}
		safe = append(safe, p)
	}
	return strings.Join(safe, "/")
}
