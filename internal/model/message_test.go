package model

import (
	"bytes"
	"testing"
)

func TestParseRFC822Attachment(t *testing.T) {
	raw := []byte("From: Britney <britney@example.com>\r\n" +
		"To: Friend <friend@example.com>\r\n" +
		"Subject: Test\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=x\r\n\r\n" +
		"--x\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello\r\n" +
		"--x\r\nContent-Type: text/plain; name=note.txt\r\n" +
		"Content-Disposition: attachment; filename=note.txt\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"aGVsbG8gYXR0YWNobWVudA==\r\n" +
		"--x--\r\n")

	msg, err := ParseRFC822(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Test" {
		t.Fatalf("subject = %q", msg.Subject)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("attachments = %d", len(msg.Attachments))
	}
	if !bytes.Equal(msg.Attachments[0].Data, []byte("hello attachment")) {
		t.Fatalf("attachment data = %q", msg.Attachments[0].Data)
	}
}

func TestBuildRFC822Structured(t *testing.T) {
	msg := &Message{
		Subject:  "Round trip",
		From:     Address{Name: "Britney", Email: "britney@example.com"},
		To:       []Address{{Name: "Friend", Email: "friend@example.com"}},
		TextBody: "hello",
		HTMLBody: "<p>hello</p>",
		Attachments: []Attachment{{
			Filename: "note.txt",
			MIMEType: "text/plain",
			Data:     []byte("attachment"),
		}},
		Parsed: true,
	}

	raw, err := BuildRFC822(msg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRFC822(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Subject != msg.Subject || len(parsed.Attachments) != 1 {
		t.Fatalf("round trip failed: subject=%q attachments=%d", parsed.Subject, len(parsed.Attachments))
	}
}

func TestParseRFC822MalformedQuotedPrintable(t *testing.T) {
	raw := []byte("From: sender@example.com\r\n" +
		"To: receiver@example.com\r\n" +
		"Subject: Broken quoted printable\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=broken\r\n\r\n" +
		"--broken\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"valid=20text, invalid=ZZ and dangling=\r\n" +
		"--broken--\r\n")

	msg, err := ParseRFC822(raw)
	if err != nil {
		t.Fatalf("ParseRFC822 rejected malformed quoted-printable: %v", err)
	}
	want := "valid text, invalid=ZZ and dangling"
	if msg.TextBody != want {
		t.Fatalf("text body = %q, want %q", msg.TextBody, want)
	}
	if !bytes.Equal(msg.Raw, raw) {
		t.Fatal("raw RFC822 message was not preserved")
	}
}

func TestParseRFC822DanglingQuotedPrintableAtEOF(t *testing.T) {
	raw := []byte("From: sender@example.com\r\n" +
		"Subject: Dangling equals\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"body ends with=")

	msg, err := ParseRFC822(raw)
	if err != nil {
		t.Fatalf("ParseRFC822 rejected dangling quoted-printable: %v", err)
	}
	if msg.TextBody != "body ends with=" {
		t.Fatalf("text body = %q", msg.TextBody)
	}
}
