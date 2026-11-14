package duplicates

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

func TestScanAndRemoval(t *testing.T) {
	for _, format := range []mailbox.Format{mailbox.FormatEML, mailbox.FormatMaildir, mailbox.FormatMbox, mailbox.FormatPST} {
		t.Run(string(format), func(t *testing.T) {
			root := t.TempDir()
			input, output := filepath.Join(root, "input"), filepath.Join(root, "output")
			writer, err := mailbox.OpenWriter(format, input, false)
			if err != nil {
				t.Fatal(err)
			}
			var firstRaw []byte
			for i, body := range []string{"first", "first", "different"} {
				raw, err := model.BuildRFC822(&model.Message{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Subject: "same", MessageID: "<same@example.com>", TextBody: body, Attachments: []model.Attachment{{Filename: "test.bin", Data: []byte{0, 1, 2}}}})
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					firstRaw = raw
				}
				if i == 1 {
					raw = firstRaw
				}
				if err := writer.Write(&model.Message{Raw: raw}); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			opts := Options{Input: input, InputFormat: format, ScanOnly: true}
			want := Result{Messages: 3, Unique: 2, Duplicates: 1}
			if got, err := Run(opts); err != nil || got != want {
				t.Fatalf("scan = %+v, %v; want %+v", got, err, want)
			}
			opts.ScanOnly, opts.Output = false, output
			if got, err := Run(opts); err != nil || got != want {
				t.Fatalf("removal = %+v, %v; want %+v", got, err, want)
			}
			reader, err := mailbox.OpenReader(format, output)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			for _, body := range []string{"first", "different"} {
				msg, err := reader.Next()
				if err != nil {
					t.Fatal(err)
				}
				if err := msg.EnsureParsed(); err != nil {
					t.Fatal(err)
				}
				if !bytes.HasPrefix([]byte(msg.TextBody), []byte(body)) || len(msg.Attachments) != 1 || !bytes.Equal(msg.Attachments[0].Data, []byte{0, 1, 2}) {
					t.Fatalf("content lost: %+v", msg)
				}
			}
			if _, err := reader.Next(); err != io.EOF {
				t.Fatalf("expected EOF, got %v", err)
			}
			if _, err := Run(opts); err == nil {
				t.Fatal("existing output accepted")
			}
			// Removing duplicates must not alter the source.
			opts.Output, opts.ScanOnly = "", true
			if got, err := Run(opts); err != nil || got != want {
				t.Fatalf("source changed: %+v, %v", got, err)
			}
		})
	}
}

func TestFingerprint(t *testing.T) {
	a, _ := fingerprint(&model.Message{Raw: []byte("Subject: test\r\n\r\nbody\r\n")})
	b, _ := fingerprint(&model.Message{Raw: []byte("Subject: test\n\nbody\n"), Folder: "other", Flags: "S"})
	if a != b {
		t.Fatal("line endings and storage metadata affected matching")
	}
	b, _ = fingerprint(&model.Message{Raw: []byte("Subject: test\n\nchanged\n")})
	if a == b {
		t.Fatal("different bodies matched")
	}
}

func TestRejectOverlappingPaths(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "source.eml")
	original := []byte("Subject: keep\n\ncontent\n")
	if err := os.WriteFile(input, original, 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias.eml")
	if err := os.Symlink(input, alias); err != nil {
		t.Fatal(err)
	}
	hardlink := filepath.Join(root, "hard.eml")
	if err := os.Link(input, hardlink); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{input, alias, hardlink, root, filepath.Join(input, "child")} {
		if _, err := Run(Options{Input: input, Output: output, Overwrite: true}); err == nil {
			t.Fatalf("accepted %s", output)
		}
	}
	got, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("source changed")
	}
}
