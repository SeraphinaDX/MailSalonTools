package convert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
)

func TestMboxToPSTMalformedQuotedPrintable(t *testing.T) {
	temp := t.TempDir()
	mboxPath := filepath.Join(temp, "broken.mbox")
	pstPath := filepath.Join(temp, "broken.pst")

	mbox := "From MAILER-DAEMON Wed Sep 30 20:00:00 2026\n" +
		"From: sender@example.com\r\n" +
		"To: receiver@example.com\r\n" +
		"Subject: Broken quoted printable\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=broken\r\n\r\n" +
		"--broken\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"valid=20text, invalid=ZZ and dangling=\r\n" +
		"--broken--\r\n"

	if err := os.WriteFile(mboxPath, []byte(mbox), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(Options{
		Input:        mboxPath,
		Output:       pstPath,
		InputFormat:  mailbox.FormatMbox,
		OutputFormat: mailbox.FormatPST,
	})
	if err != nil {
		t.Fatalf("mbox to PST failed on malformed quoted-printable: %v", err)
	}
	if result.Messages != 1 {
		t.Fatalf("converted %d messages, want 1", result.Messages)
	}
	if info, err := os.Stat(pstPath); err != nil || info.Size() == 0 {
		t.Fatalf("PST output missing or empty: info=%v err=%v", info, err)
	}
}


func TestMboxToPSTTruncatedMultipart(t *testing.T) {
	temp := t.TempDir()
	mboxPath := filepath.Join(temp, "truncated.mbox")
	pstPath := filepath.Join(temp, "truncated.pst")

	mbox := "From MAILER-DAEMON Wed Sep 30 21:00:00 2026\n" +
		"From: sender@example.com\r\n" +
		"To: receiver@example.com\r\n" +
		"Subject: Truncated multipart\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=broken\r\n\r\n" +
		"--broken\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
		"body survives without closing boundary"

	if err := os.WriteFile(mboxPath, []byte(mbox), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(Options{
		Input:        mboxPath,
		Output:       pstPath,
		InputFormat:  mailbox.FormatMbox,
		OutputFormat: mailbox.FormatPST,
	})
	if err != nil {
		t.Fatalf("mbox to PST failed on truncated multipart: %v", err)
	}
	if result.Messages != 1 {
		t.Fatalf("converted %d messages, want 1", result.Messages)
	}
	if info, err := os.Stat(pstPath); err != nil || info.Size() == 0 {
		t.Fatalf("PST output missing or empty: info=%v err=%v", info, err)
	}
}
