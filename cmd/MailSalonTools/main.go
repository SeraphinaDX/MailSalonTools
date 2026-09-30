package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/SeraphinaDX/MailSalonTools/internal/convert"
	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/tui"
	"github.com/SeraphinaDX/MailSalonTools/internal/version"
)

func main() {
	if len(os.Args) == 1 {
		selection, err := tui.Run()
		if err != nil {
			if err.Error() != "cancelled" {
				fmt.Fprintln(os.Stderr, "MailSalonTools:", err)
			}
			return
		}
		run(convert.Options{Input: selection.Input, Output: selection.Output})
		return
	}
	fs := flag.NewFlagSet("MailSalonTools", flag.ExitOnError)
	input := fs.String("input", "", "input mailbox path")
	output := fs.String("output", "", "output mailbox path")
	inputFormat := fs.String("input-format", "", "input format: eml, maildir, mbox, pst")
	outputFormat := fs.String("output-format", "", "output format: eml, maildir, mbox, pst")
	overwrite := fs.Bool("overwrite", false, "replace an existing output")
	showVersion := fs.Bool("version", false, "show version and exit")
	_ = fs.Parse(os.Args[1:])
	if *showVersion {
		fmt.Printf("MailSalonTools %s
", version.Version)
		return
	}
	opts := convert.Options{Input: *input, Output: *output, Overwrite: *overwrite}
	var err error
	if strings.TrimSpace(*inputFormat) != "" {
		opts.InputFormat, err = mailbox.ParseFormat(*inputFormat)
		if err != nil { fatal(err) }
	}
	if strings.TrimSpace(*outputFormat) != "" {
		opts.OutputFormat, err = mailbox.ParseFormat(*outputFormat)
		if err != nil { fatal(err) }
	}
	run(opts)
}

func run(opts convert.Options) {
	opts.Progress = func(n int) { fmt.Printf("✉ Converted %d message(s)...", n) }
	result, err := convert.Run(opts)
	if err != nil {
		if result.Messages > 0 { fmt.Println() }
		fatal(err)
	}
	if result.Messages > 0 { fmt.Println() }
	fmt.Printf("✓ Finished: %d message(s) converted.
", result.Messages)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "MailSalonTools:", err)
	os.Exit(1)
}
