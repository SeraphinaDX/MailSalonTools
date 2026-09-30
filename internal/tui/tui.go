package tui

import (
	"fmt"
	"strings"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"github.com/SeraphinaDX/MailSalonTools/internal/mailbox"
)

type Selection struct {
	Input string
	Output string
}

func Run() (Selection, error) {
	if err := ui.Init(); err != nil {
		return Selection{}, err
	}
	defer ui.Close()

	input := widgets.NewInput()
	input.Title = " Input "
	input.Placeholder = "/path/to/mailbox"
	input.BorderStyle.Fg = ui.ColorMagenta
	output := widgets.NewInput()
	output.Title = " Output "
	output.Placeholder = "/path/to/output.pst"
	output.BorderStyle.Fg = ui.ColorMagenta
	info := widgets.NewParagraph()
	info.Title = " MailSalonTools "
	info.Text = "Mailbox conversion, minus the drama.

Tab: switch field   Enter: convert   Esc/q: quit"
	info.BorderStyle.Fg = ui.ColorMagenta
	info.TitleStyle.Fg = ui.ColorMagenta
	status := widgets.NewParagraph()
	status.Title = " Formats "
	status.BorderStyle.Fg = ui.ColorCyan

	focus := 0
	events := ui.PollEvents()
	redraw := func() {
		width, height := ui.TerminalDimensions()
		if width < 50 { width = 50 }
		if height < 16 { height = 16 }
		left, right := width/2-24, width/2+24
		info.SetRect(left, 1, right, 7)
		input.SetRect(left, 7, right, 10)
		output.SetRect(left, 10, right, 13)
		status.SetRect(left, 13, right, 16)
		inFmt := "?"
		if strings.TrimSpace(input.Text) != "" {
			if f, err := mailbox.DetectInput(strings.TrimSpace(input.Text)); err == nil { inFmt = string(f) }
		}
		outFmt := "?"
		if strings.TrimSpace(output.Text) != "" {
			if f, err := mailbox.DetectOutput(strings.TrimSpace(output.Text)); err == nil { outFmt = string(f) }
		}
		status.Text = fmt.Sprintf("Input: %s    Output: %s", inFmt, outFmt)
		if focus == 0 {
			input.TitleStyle.Fg = ui.ColorCyan
			output.TitleStyle.Fg = ui.ColorWhite
		} else {
			input.TitleStyle.Fg = ui.ColorWhite
			output.TitleStyle.Fg = ui.ColorCyan
		}
		ui.Clear()
		ui.Render(info, input, output, status)
	}
	active := func() *widgets.Input {
		if focus == 0 { return input }
		return output
	}
	redraw()
	for {
		e := <-events
		switch e.ID {
		case "q", "<Escape>", "<C-c>":
			return Selection{}, fmt.Errorf("cancelled")
		case "<Tab>", "<Down>", "<Up>":
			focus = 1 - focus
		case "<Enter>":
			if strings.TrimSpace(input.Text) != "" && strings.TrimSpace(output.Text) != "" {
				return Selection{Input: strings.TrimSpace(input.Text), Output: strings.TrimSpace(output.Text)}, nil
			}
		case "<Backspace>":
			active().Backspace()
		case "<Left>":
			active().MoveCursorLeft()
		case "<Right>":
			active().MoveCursorRight()
		default:
			if len([]rune(e.ID)) == 1 { active().InsertRune([]rune(e.ID)[0]) }
		}
		redraw()
	}
}
