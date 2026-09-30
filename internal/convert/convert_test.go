package convert

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

func TestEMLPSTRoundTrip(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "input.eml")
	pstPath := filepath.Join(temp, "archive.pst")
	outputDir := filepath.Join(temp, "exported")

	wantHTML := "<html><body><p>" + strings.Repeat("large HTML body from MailSalonTools ", 2000) + "</p></body></html>"
	wantAttachment := bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 8192)

	raw, err := model.BuildRFC822(&model.Message{
		Subject:   "MailSalonTools PST round trip",
		MessageID: "<mailsalontools-test@example.com>",
		From:      model.Address{Name: "Britney", Email: "britney@example.com"},
		To:        []model.Address{{Name: "Friend", Email: "friend@example.com"}},
		TextBody:  "hello from MailSalonTools",
		HTMLBody:  wantHTML,
		Attachments: []model.Attachment{{
			Filename: "hello.bin",
			MIMEType: "application/octet-stream",
			Data:     wantAttachment,
		}},
		Parsed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	parsedInput, err := model.ParseRFC822(raw)
	if err != nil {
		t.Fatalf("parse generated EML: %v", err)
	}
	if parsedInput.HTMLBody != wantHTML {
		t.Fatalf("generated EML HTML length = %d, want %d", len(parsedInput.HTMLBody), len(wantHTML))
	}
	if err := os.WriteFile(input, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Run(Options{
		Input:        input,
		Output:       pstPath,
		InputFormat:  mailbox.FormatEML,
		OutputFormat: mailbox.FormatPST,
	})
	if err != nil {
		t.Fatalf("EML to PST: %v", err)
	}
	if first.Messages != 1 {
		t.Fatalf("EML to PST converted %d messages", first.Messages)
	}

	second, err := Run(Options{
		Input:        pstPath,
		Output:       outputDir,
		InputFormat:  mailbox.FormatPST,
		OutputFormat: mailbox.FormatEML,
	})
	if err != nil {
		t.Fatalf("PST to EML: %v", err)
	}
	if second.Messages != 1 {
		t.Fatalf("PST to EML converted %d messages", second.Messages)
	}

	exported := filepath.Join(outputDir, "Imported", "000001.eml")
	gotRaw, err := os.ReadFile(exported)
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.ParseRFC822(gotRaw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "MailSalonTools PST round trip" {
		t.Fatalf("subject = %q", got.Subject)
	}
	if got.HTMLBody != wantHTML {
		t.Fatalf("HTML length = %d, want %d", len(got.HTMLBody), len(wantHTML))
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("attachments = %d", len(got.Attachments))
	}
	if got.Attachments[0].Filename != "hello.bin" {
		t.Fatalf("attachment filename = %q", got.Attachments[0].Filename)
	}
	if !bytes.Equal(got.Attachments[0].Data, wantAttachment) {
		t.Fatalf("attachment length = %d, want %d", len(got.Attachments[0].Data), len(wantAttachment))
	}
}
