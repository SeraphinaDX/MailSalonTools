package convert

import (
	"fmt"
	"io"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
)

type Options struct {
	Input string
	Output string
	InputFormat mailbox.Format
	OutputFormat mailbox.Format
	Overwrite bool
	Progress func(int)
}

type Result struct{ Messages int }

func Run(opts Options) (Result, error) {
	if opts.Input == "" || opts.Output == "" {
		return Result{}, fmt.Errorf("both input and output paths are required")
	}
	inFmt := opts.InputFormat
	if inFmt == "" {
		var err error
		inFmt, err = mailbox.DetectInput(opts.Input)
		if err != nil {
			return Result{}, fmt.Errorf("detect input format: %w", err)
		}
	}
	outFmt := opts.OutputFormat
	if outFmt == "" {
		var err error
		outFmt, err = mailbox.DetectOutput(opts.Output)
		if err != nil {
			return Result{}, fmt.Errorf("detect output format: %w", err)
		}
	}
	reader, err := mailbox.OpenReader(inFmt, opts.Input)
	if err != nil {
		return Result{}, fmt.Errorf("open %s input: %w", inFmt, err)
	}
	defer reader.Close()
	writer, err := mailbox.OpenWriter(outFmt, opts.Output, opts.Overwrite)
	if err != nil {
		return Result{}, fmt.Errorf("open %s output: %w", outFmt, err)
	}
	result := Result{}
	for {
		msg, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = writer.Close()
			return result, fmt.Errorf("read message %d: %w", result.Messages+1, err)
		}
		if err := writer.Write(msg); err != nil {
			_ = writer.Close()
			return result, fmt.Errorf("write message %d: %w", result.Messages+1, err)
		}
		result.Messages++
		if opts.Progress != nil {
			opts.Progress(result.Messages)
		}
	}
	if err := writer.Close(); err != nil {
		return result, fmt.Errorf("close output: %w", err)
	}
	return result, nil
}
