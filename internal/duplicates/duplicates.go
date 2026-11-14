// Package duplicates scans mailboxes and writes copies containing unique messages.
package duplicates

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/model"
)

type Options struct {
	Input, Output             string
	InputFormat, OutputFormat mailbox.Format
	ScanOnly, Overwrite       bool
	Progress                  func(int)
}

type Result struct{ Messages, Unique, Duplicates int }

func Run(opts Options) (Result, error) {
	var result Result
	if opts.Input == "" {
		return result, fmt.Errorf("input path is required")
	}
	if opts.ScanOnly && opts.Output != "" {
		return result, fmt.Errorf("duplicate scanning does not accept an output path")
	}
	if !opts.ScanOnly {
		if opts.Output == "" {
			return result, fmt.Errorf("output path is required for duplicate removal")
		}
		if err := separatePaths(opts.Input, opts.Output); err != nil {
			return result, err
		}
	}
	format := opts.InputFormat
	if format == "" {
		var err error
		format, err = mailbox.DetectInput(opts.Input)
		if err != nil {
			return result, err
		}
	}
	reader, err := mailbox.OpenReader(format, opts.Input)
	if err != nil {
		return result, err
	}
	defer reader.Close()
	var writer mailbox.Writer
	if !opts.ScanOnly {
		outFormat := opts.OutputFormat
		if outFormat == "" {
			outFormat = format
		}
		writer, err = mailbox.OpenWriter(outFormat, opts.Output, opts.Overwrite)
		if err != nil {
			return result, err
		}
		defer func() {
			if writer != nil {
				_ = writer.Close()
			}
		}()
	}
	seen := make(map[[32]byte]struct{})
	for {
		msg, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, fmt.Errorf("read message %d: %w", result.Messages+1, err)
		}
		key, err := fingerprint(msg)
		if err != nil {
			return result, fmt.Errorf("fingerprint message %d: %w", result.Messages+1, err)
		}
		result.Messages++
		if _, exists := seen[key]; exists {
			result.Duplicates++
		} else {
			if writer != nil {
				if err := writer.Write(msg); err != nil {
					return result, fmt.Errorf("write message %d: %w", result.Messages, err)
				}
			}
			seen[key] = struct{}{}
			result.Unique++
		}
		if opts.Progress != nil {
			opts.Progress(result.Messages)
		}
	}
	if writer != nil {
		err := writer.Close()
		writer = nil
		if err != nil {
			return result, fmt.Errorf("close output: %w", err)
		}
	}
	return result, nil
}

func fingerprint(msg *model.Message) ([32]byte, error) {
	if len(msg.Raw) > 0 {
		return sha256.Sum256(bytes.ReplaceAll(msg.Raw, []byte("\r\n"), []byte("\n"))), nil
	}
	// PST messages have structured content. Avoid RFC822 serialization, whose
	// generated MIME boundaries vary, and exclude storage location/read state.
	content := *msg
	content.Folder, content.Flags, content.Parsed = "", "", false
	content.Date = content.Date.UTC()
	raw, err := json.Marshal(content)
	return sha256.Sum256(raw), err
}

// Resolve existing ancestors too, so a new output under a symlink cannot
// accidentally overwrite or become part of the source mailbox.
func resolvedPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(absolute)
	if parent == absolute {
		return "", err
	}
	resolved, err = resolvedPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(absolute)), nil
}

func separatePaths(input, output string) error {
	in, err := resolvedPath(input)
	if err != nil {
		return err
	}
	out, err := resolvedPath(output)
	if err != nil {
		return err
	}
	overlap := func(a, b string) bool {
		rel, err := filepath.Rel(a, b)
		return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
	}
	inInfo, inErr := os.Stat(in)
	outInfo, outErr := os.Stat(out)
	if overlap(in, out) || overlap(out, in) || (inErr == nil && outErr == nil && os.SameFile(inInfo, outInfo)) {
		return fmt.Errorf("input and output must be separate, non-overlapping paths")
	}
	return nil
}
