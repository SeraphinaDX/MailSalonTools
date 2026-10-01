package mailbox

import (
	"bytes"
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

	const count = 700
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


func TestPSTWriterVeryLargeAttachment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xxblock-attachment.pst")
	w, err := newPSTWriter(path, true)
	if err != nil {
		t.Fatalf("newPSTWriter: %v", err)
	}

	// 16 MiB requires more than 2,000 Unicode PST data blocks, matching the
	// real mailbox attachment that exposed the single-XBLOCK limit.
	wantAttachment := bytes.Repeat([]byte{0x5C}, 16*1024*1024)
	msg := &model.Message{
		Subject:  "very large attachment",
		TextBody: "body",
		Attachments: []model.Attachment{{
			Filename: "large.bin",
			MIMEType: "application/octet-stream",
			Data:     wantAttachment,
		}},
		Parsed: true,
	}
	if err := w.Write(msg); err != nil {
		_ = w.Close()
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	r, err := newPSTReader(path)
	if err != nil {
		t.Fatalf("newPSTReader: %v", err)
	}
	defer func() { _ = r.Close() }()

	got, err := r.Next()
	if err != nil {
		t.Fatalf("Read message: %v", err)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(got.Attachments))
	}
	if got.Attachments[0].Filename != "large.bin" {
		t.Fatalf("attachment filename = %q", got.Attachments[0].Filename)
	}
	if !bytes.Equal(got.Attachments[0].Data, wantAttachment) {
		t.Fatalf("attachment bytes differ: got=%d want=%d", len(got.Attachments[0].Data), len(wantAttachment))
	}
}
