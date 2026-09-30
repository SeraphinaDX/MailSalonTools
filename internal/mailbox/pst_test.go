package mailbox

import (
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

func TestPSTWriterContinuesAfterBatchReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batch-reopen.pst")
	w, err := newPSTWriter(path, true)
	if err != nil {
		t.Fatalf("newPSTWriter: %v", err)
	}

	const count = 105
	for i := 0; i < count; i++ {
		msg := &model.Message{
			Subject:   fmt.Sprintf("batch message %03d", i),
			MessageID: fmt.Sprintf("<batch-%03d@example.com>", i),
			TextBody:  "body",
			Parsed:    true,
		}
		if err := w.Write(msg); err != nil {
			_ = w.Close()
			t.Fatalf("Write message %d: %v", i+1, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	r, err := newPSTReader(path)
	if err != nil {
		t.Fatalf("newPSTReader: %v", err)
	}
	defer func() { _ = r.Close() }()

	got := 0
	for {
		msg, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read message %d: %v", got+1, err)
		}
		if msg.Subject == "" {
			t.Fatalf("message %d has empty subject", got+1)
		}
		got++
	}
	if got != count {
		t.Fatalf("message count = %d, want %d", got, count)
	}
}
