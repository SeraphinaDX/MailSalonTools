package convert

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

func TestEMLPSTRoundTrip(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "input.eml")
	pstPath := filepath.Join(temp, "archive.pst")
	outputDir := filepath.Join(temp, "exported")

	raw := []byte("From: Britney <britney@example.com>\r\n" +
		"To: Friend <friend@example.com>\r\n" +
		"Subject: MailSalonTools PST round trip\r\n" +
		"Message-ID: <mailsalontools-test@example.com>\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=x\r\n\r\n" +
		"--x\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello from MailSalonTools\r\n" +
		"--x\r\nContent-Type: application/octet-stream; name=hello.bin\r\n" +
		"Content-Disposition: attachment; filename=hello.bin\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\nAAECAwQF\r\n" +
		"--x--\r\n")

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
	if len(got.Attachments) != 1 {
		t.Fatalf("attachments = %d", len(got.Attachments))
	}
	if got.Attachments[0].Filename != "hello.bin" {
		t.Fatalf("attachment filename = %q", got.Attachments[0].Filename)
	}
	if !bytes.Equal(got.Attachments[0].Data, []byte{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("attachment data = %v", got.Attachments[0].Data)
	}
}
