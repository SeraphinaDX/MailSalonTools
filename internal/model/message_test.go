package model

import (
	"bytes"
	"testing"
)

func TestParseRFC822Attachment(t *testing.T) {
	raw := []byte("From: Britney <britney@example.com>
" +
		"To: Friend <friend@example.com>
" +
		"Subject: Test
MIME-Version: 1.0
" +
		"Content-Type: multipart/mixed; boundary=x

" +
		"--x
Content-Type: text/plain; charset=utf-8

hello
" +
		"--x
Content-Type: text/plain; name=note.txt
" +
		"Content-Disposition: attachment; filename=note.txt
" +
		"Content-Transfer-Encoding: base64

aGVsbG8gYXR0YWNobWVudA==
--x--
")
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
		Subject: "Round trip",
		From: Address{Name: "Britney", Email: "britney@example.com"},
		To: []Address{{Name: "Friend", Email: "friend@example.com"}},
		TextBody: "hello", HTMLBody: "<p>hello</p>",
		Attachments: []Attachment{{Filename: "note.txt", MIMEType: "text/plain", Data: []byte("attachment")}},
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
