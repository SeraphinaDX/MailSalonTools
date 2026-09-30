package mailbox

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

type mboxReader struct {
	file *os.File
	br *bufio.Reader
	ready bool
	eof bool
}

func newMboxReader(path string) (*mboxReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &mboxReader{file: f, br: bufio.NewReaderSize(f, 128*1024)}, nil
}

func (r *mboxReader) Next() (*model.Message, error) {
	if r.eof {
		return nil, io.EOF
	}
	if !r.ready {
		for {
			line, err := r.br.ReadString('
')
			if strings.HasPrefix(line, "From ") {
				r.ready = true
				break
			}
			if err != nil {
				r.eof = true
				return nil, io.EOF
			}
		}
	}
	var b bytes.Buffer
	for {
		line, err := r.br.ReadString('
')
		if strings.HasPrefix(line, "From ") && b.Len() > 0 {
			r.ready = true
			return &model.Message{Raw: append([]byte(nil), b.Bytes()...)}, nil
		}
		if strings.HasPrefix(line, ">From ") {
			line = line[1:]
		}
		b.WriteString(line)
		if err == io.EOF {
			r.eof = true
			if b.Len() == 0 {
				return nil, io.EOF
			}
			return &model.Message{Raw: append([]byte(nil), b.Bytes()...)}, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func (r *mboxReader) Close() error { return r.file.Close() }

type mboxWriter struct{ file *os.File }

func newMboxWriter(path string, overwrite bool) (*mboxWriter, error) {
	flags := os.O_CREATE | os.O_WRONLY
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("output exists: %s (use -overwrite)", path)
		}
		return nil, err
	}
	return &mboxWriter{file: f}, nil
}

func (w *mboxWriter) Write(msg *model.Message) error {
	raw, err := msg.RFC822()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w.file, "From MAILER-DAEMON %s
", time.Now().Format(time.ANSIC)); err != nil {
		return err
	}
	for _, line := range bytes.SplitAfter(raw, []byte("
")) {
		if bytes.HasPrefix(line, []byte("From ")) {
			if _, err := w.file.Write([]byte(">")); err != nil {
				return err
			}
		}
		if _, err := w.file.Write(line); err != nil {
			return err
		}
	}
	if len(raw) == 0 || raw[len(raw)-1] != '
' {
		if _, err := w.file.Write([]byte("
")); err != nil {
			return err
		}
	}
	_, err = w.file.Write([]byte("
"))
	return err
}

func (w *mboxWriter) Close() error { return w.file.Close() }
