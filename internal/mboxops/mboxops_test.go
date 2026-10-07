package mboxops

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

func TestCombineMboxPreservesOrderAndFromLines(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.mbox")
	second := filepath.Join(dir, "second.mbox")
	output := filepath.Join(dir, "combined.mbox")

	writeMessages(t, first,
		rawMessage("First", "Mon, 02 Jan 2023 10:00:00 +0000", "hello\nFrom body line\n"),
	)
	writeMessages(t, second,
		rawMessage("Second", "Tue, 03 Jan 2023 10:00:00 +0000", "world\n"),
	)

	result, err := Combine([]string{first, second}, output, false, nil)
	if err != nil {
		t.Fatalf("Combine: %v", err)
	}
	if result.Messages != 2 || result.Inputs != 2 {
		t.Fatalf("result = %+v", result)
	}

	reader, err := mailbox.OpenReader(mailbox.FormatMbox, output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	msg1, err := reader.Next()
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	if !bytes.Contains(msg1.Raw, []byte("Subject: First")) {
		t.Fatalf("first message out of order: %q", msg1.Raw)
	}
	if !bytes.Contains(msg1.Raw, []byte("\nFrom body line\n")) {
		t.Fatalf("From body line was not preserved: %q", msg1.Raw)
	}

	msg2, err := reader.Next()
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if !bytes.Contains(msg2.Raw, []byte("Subject: Second")) {
		t.Fatalf("second message out of order: %q", msg2.Raw)
	}
	if _, err := reader.Next(); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestSplitByYear(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "archive.mbox")
	output := filepath.Join(dir, "years")

	writeMessages(t, input,
		rawMessage("2020 A", "Thu, 02 Jan 2020 10:00:00 +0000", "a\n"),
		rawMessage("2021", "Fri, 05 Feb 2021 11:00:00 +0000", "b\n"),
		rawMessage("2020 B", "Sat, 07 Mar 2020 12:00:00 +0000", "c\n"),
		rawMessage("Unknown", "not-a-date", "d\n"),
		rawMessage("Missing", "", "e\n"),
	)

	result, err := SplitByYear(input, output, false, nil)
	if err != nil {
		t.Fatalf("SplitByYear: %v", err)
	}
	if result.Messages != 5 {
		t.Fatalf("messages = %d, want 5", result.Messages)
	}
	wantCounts := map[string]int{
		"2020.mbox":    2,
		"2021.mbox":    1,
		"unknown.mbox": 2,
	}
	for name, want := range wantCounts {
		if got := result.Files[name]; got != want {
			t.Fatalf("%s count = %d, want %d", name, got, want)
		}
		if got := countMbox(t, filepath.Join(output, name)); got != want {
			t.Fatalf("%s file count = %d, want %d", name, got, want)
		}
	}
}

func TestSplitByYearProtectsExistingOutputs(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "archive.mbox")
	output := filepath.Join(dir, "years")
	writeMessages(t, input, rawMessage("2022", "Sun, 02 Jan 2022 10:00:00 +0000", "body\n"))

	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(output, "2022.mbox")
	if err := os.WriteFile(existing, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SplitByYear(input, output, false, nil); err == nil {
		t.Fatal("expected existing-output error")
	}
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep me" {
		t.Fatalf("existing file changed: %q", data)
	}

	result, err := SplitByYear(input, output, true, nil)
	if err != nil {
		t.Fatalf("SplitByYear overwrite: %v", err)
	}
	if result.Files["2022.mbox"] != 1 {
		t.Fatalf("overwrite result = %+v", result.Files)
	}
}

func writeMessages(t *testing.T, path string, raws ...[]byte) {
	t.Helper()
	writer, err := mailbox.OpenWriter(mailbox.FormatMbox, path, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range raws {
		if err := writer.Write(&model.Message{Raw: raw}); err != nil {
			_ = writer.Close()
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func countMbox(t *testing.T, path string) int {
	t.Helper()
	reader, err := mailbox.OpenReader(mailbox.FormatMbox, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	count := 0
	for {
		_, err := reader.Next()
		if err == io.EOF {
			return count
		}
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
}

func rawMessage(subject, date, body string) []byte {
	var b bytes.Buffer
	b.WriteString("From: sender@example.com\r\n")
	b.WriteString("To: receiver@example.com\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	if date != "" {
		b.WriteString("Date: " + date + "\r\n")
	}
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.Bytes()
}
