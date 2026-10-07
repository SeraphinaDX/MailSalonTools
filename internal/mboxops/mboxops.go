package mboxops

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

type ProgressFunc func(int)

type CombineResult struct {
	Messages int
	Inputs   int
}

type SplitResult struct {
	Messages int
	Files    map[string]int
}

func Combine(inputs []string, output string, overwrite bool, progress ProgressFunc) (CombineResult, error) {
	if len(inputs) < 2 {
		return CombineResult{}, fmt.Errorf("combine requires at least two input mbox files")
	}
	if output == "" {
		return CombineResult{}, fmt.Errorf("combine output path is required")
	}

	outAbs, err := filepath.Abs(output)
	if err != nil {
		return CombineResult{}, fmt.Errorf("resolve output path: %w", err)
	}
	for _, input := range inputs {
		inAbs, err := filepath.Abs(input)
		if err != nil {
			return CombineResult{}, fmt.Errorf("resolve input path %s: %w", input, err)
		}
		if inAbs == outAbs {
			return CombineResult{}, fmt.Errorf("output must not be one of the input mbox files: %s", input)
		}
	}

	writer, err := mailbox.OpenWriter(mailbox.FormatMbox, output, overwrite)
	if err != nil {
		return CombineResult{}, fmt.Errorf("open combined mbox: %w", err)
	}

	result := CombineResult{Inputs: len(inputs)}
	closeWriter := func() error {
		if writer == nil {
			return nil
		}
		err := writer.Close()
		writer = nil
		return err
	}

	for _, input := range inputs {
		reader, err := mailbox.OpenReader(mailbox.FormatMbox, input)
		if err != nil {
			_ = closeWriter()
			return result, fmt.Errorf("open input mbox %s: %w", input, err)
		}

		for {
			msg, readErr := reader.Next()
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				_ = reader.Close()
				_ = closeWriter()
				return result, fmt.Errorf("read %s message %d: %w", input, result.Messages+1, readErr)
			}
			if err := writer.Write(msg); err != nil {
				_ = reader.Close()
				_ = closeWriter()
				return result, fmt.Errorf("write combined message %d from %s: %w", result.Messages+1, input, err)
			}
			result.Messages++
			if progress != nil {
				progress(result.Messages)
			}
		}

		if err := reader.Close(); err != nil {
			_ = closeWriter()
			return result, fmt.Errorf("close input mbox %s: %w", input, err)
		}
	}

	if err := closeWriter(); err != nil {
		return result, fmt.Errorf("close combined mbox: %w", err)
	}
	return result, nil
}

var generatedYearFile = regexp.MustCompile(`^(?:[0-9]{4}|unknown)\.mbox$`)

func SplitByYear(input, outputDir string, overwrite bool, progress ProgressFunc) (SplitResult, error) {
	if input == "" || outputDir == "" {
		return SplitResult{}, fmt.Errorf("split-by-year requires input mbox and output directory")
	}
	if err := prepareOutputDir(outputDir, overwrite); err != nil {
		return SplitResult{}, err
	}

	reader, err := mailbox.OpenReader(mailbox.FormatMbox, input)
	if err != nil {
		return SplitResult{}, fmt.Errorf("open input mbox: %w", err)
	}
	defer reader.Close()

	result := SplitResult{Files: make(map[string]int)}
	writers := make(map[string]mailbox.Writer)
	closeWriters := func() error {
		names := make([]string, 0, len(writers))
		for name := range writers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := writers[name].Close(); err != nil {
				return fmt.Errorf("close %s: %w", name, err)
			}
		}
		return nil
	}

	for {
		msg, readErr := reader.Next()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = closeWriters()
			return result, fmt.Errorf("read message %d: %w", result.Messages+1, readErr)
		}

		name := yearFilename(msg)
		writer := writers[name]
		if writer == nil {
			path := filepath.Join(outputDir, name)
			writer, err = mailbox.OpenWriter(mailbox.FormatMbox, path, true)
			if err != nil {
				_ = closeWriters()
				return result, fmt.Errorf("open year output %s: %w", name, err)
			}
			writers[name] = writer
		}

		if err := writer.Write(msg); err != nil {
			_ = closeWriters()
			return result, fmt.Errorf("write message %d to %s: %w", result.Messages+1, name, err)
		}
		result.Messages++
		result.Files[name]++
		if progress != nil {
			progress(result.Messages)
		}
	}

	if err := closeWriters(); err != nil {
		return result, err
	}
	return result, nil
}

func prepareOutputDir(dir string, overwrite bool) error {
	info, err := os.Stat(dir)
	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("split output exists and is not a directory: %s", dir)
	case err == nil:
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read split output directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !generatedYearFile.MatchString(entry.Name()) {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if !overwrite {
				return fmt.Errorf("split output exists: %s (use -overwrite)", path)
			}
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove old split output %s: %w", path, err)
			}
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create split output directory: %w", err)
		}
	default:
		return fmt.Errorf("stat split output directory: %w", err)
	}
	return nil
}

func yearFilename(msg *model.Message) string {
	year := messageYear(msg)
	if year < 1 || year > 9999 {
		return "unknown.mbox"
	}
	return fmt.Sprintf("%04d.mbox", year)
}

func messageYear(msg *model.Message) int {
	if !msg.Date.IsZero() {
		return msg.Date.Year()
	}
	if len(msg.Raw) == 0 {
		return 0
	}
	rm, err := mail.ReadMessage(bytes.NewReader(msg.Raw))
	if err != nil {
		return 0
	}
	dateHeader := rm.Header.Get("Date")
	if dateHeader == "" {
		return 0
	}
	d, err := mail.ParseDate(dateHeader)
	if err != nil {
		return 0
	}
	if d.Before(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		return 0
	}
	return d.Year()
}
