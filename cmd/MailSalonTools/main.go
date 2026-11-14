package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/SeraphinaDX/MailSalonTools/internal/convert"
	"github.com/SeraphinaDX/MailSalonTools/internal/duplicates"
	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
	"github.com/SeraphinaDX/MailSalonTools/internal/mboxops"
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
	combineMbox := fs.Bool("combine-mbox", false, "combine positional mbox files into -output")
	splitByYear := fs.Bool("split-by-year", false, "split -input mbox into YYYY.mbox files under -output")
	findDuplicates := fs.Bool("find-duplicates", false, "scan for duplicate messages without writing output")
	removeDuplicates := fs.Bool("remove-duplicates", false, "write a cleaned mailbox to -output (same format by default)")
	showVersion := fs.Bool("version", false, "show version and exit")
	_ = fs.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("MailSalonTools %s\n", version.Version)
		return
	}

	operations := 0
	for _, enabled := range []bool{*combineMbox, *splitByYear, *findDuplicates, *removeDuplicates} {
		if enabled {
			operations++
		}
	}
	if operations > 1 {
		fatal(fmt.Errorf("choose only one of -combine-mbox, -split-by-year, -find-duplicates, and -remove-duplicates"))
	}
	if *combineMbox {
		inputs := append([]string(nil), fs.Args()...)
		if strings.TrimSpace(*input) != "" {
			inputs = append([]string{*input}, inputs...)
		}
		runCombineMbox(inputs, *output, *overwrite)
		return
	}
	if *splitByYear {
		if len(fs.Args()) != 0 {
			fatal(fmt.Errorf("-split-by-year does not accept positional inputs; use -input"))
		}
		runSplitByYear(*input, *output, *overwrite)
		return
	}
	if len(fs.Args()) != 0 {
		fatal(fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " ")))
	}

	opts := convert.Options{Input: *input, Output: *output, Overwrite: *overwrite}
	var err error
	if strings.TrimSpace(*inputFormat) != "" {
		opts.InputFormat, err = mailbox.ParseFormat(*inputFormat)
		if err != nil {
			fatal(err)
		}
	}
	if strings.TrimSpace(*outputFormat) != "" {
		opts.OutputFormat, err = mailbox.ParseFormat(*outputFormat)
		if err != nil {
			fatal(err)
		}
	}
	if *findDuplicates || *removeDuplicates {
		if *findDuplicates && (*overwrite || *outputFormat != "") {
			fatal(fmt.Errorf("-find-duplicates does not accept -overwrite or -output-format"))
		}
		result, err := duplicates.Run(duplicates.Options{Input: opts.Input, Output: opts.Output, InputFormat: opts.InputFormat, OutputFormat: opts.OutputFormat, Overwrite: opts.Overwrite, ScanOnly: *findDuplicates})
		if err != nil {
			fatal(err)
		}
		action := "found"
		if *removeDuplicates {
			action = "removed from output"
		}
		fmt.Printf("✓ Finished: %d message(s) scanned, %d unique, %d duplicate(s) %s.\n", result.Messages, result.Unique, result.Duplicates, action)
		return
	}
	run(opts)
}

func runCombineMbox(inputs []string, output string, overwrite bool) {
	result, err := mboxops.Combine(inputs, output, overwrite, func(n int) {
		fmt.Printf("\r✉ Combined %d message(s)...", n)
	})
	if result.Messages > 0 {
		fmt.Println()
	}
	if err != nil {
		fatal(err)
	}
	fmt.Printf("✓ Finished: %d message(s) combined from %d mbox file(s).\n", result.Messages, result.Inputs)
}

func runSplitByYear(input, output string, overwrite bool) {
	result, err := mboxops.SplitByYear(input, output, overwrite, func(n int) {
		fmt.Printf("\r✉ Split %d message(s)...", n)
	})
	if result.Messages > 0 {
		fmt.Println()
	}
	if err != nil {
		fatal(err)
	}
	fmt.Printf("✓ Finished: %d message(s) split into %d year file(s).\n", result.Messages, len(result.Files))
}

func run(opts convert.Options) {
	opts.Progress = func(n int) {
		fmt.Printf("\r✉ Converted %d message(s)...", n)
	}
	result, err := convert.Run(opts)
	if err != nil {
		if result.Messages > 0 {
			fmt.Println()
		}
		fatal(err)
	}
	if result.Messages > 0 {
		fmt.Println()
	}
	fmt.Printf("✓ Finished: %d message(s) converted.\n", result.Messages)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "MailSalonTools:", err)
	os.Exit(1)
}
