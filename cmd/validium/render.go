package main

import (
	"fmt"
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
	"github.com/franium/validium/internal/validium"
)

func terminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return 80
	}
	if w > 120 {
		return 120
	}
	return w
}

var (
	red   = lipgloss.Color("#FF5F5F")
	green = lipgloss.Color("#5FFF87")
	muted = lipgloss.Color("#626262")
	white = lipgloss.Color("#FFFFFF")

	errorBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(red).
			PaddingTop(1).
			PaddingBottom(1).
			PaddingLeft(3).
			PaddingRight(3)

	successBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(green).
			PaddingTop(1).
			PaddingBottom(1).
			PaddingLeft(3).
			PaddingRight(3)

	titleErrorStyle   = lipgloss.NewStyle().Bold(true).Foreground(red)
	titleSuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(green)
	varNameStyle      = lipgloss.NewStyle().Bold(true).Foreground(white)
	msgStyle          = lipgloss.NewStyle().Foreground(muted)
	errorMark         = lipgloss.NewStyle().Foreground(red).Render("✗")
	successMark       = lipgloss.NewStyle().Foreground(green).Render("✓")
)

func renderValidationErrors(errors []validium.ValidiumValidationError) {
	var sb strings.Builder

	sb.WriteString(titleErrorStyle.Render("Validation Failed"))
	sb.WriteString("\n\n")

	for _, e := range errors {
		value := ""
		if e.Secret {
			value = msgStyle.Render("--> ••••••")
		} else if e.Value != "" {
			value = msgStyle.Render("--> " + e.Value)
		}
		styledName := varNameStyle.Render(e.VariableName)
		styledMsg := msgStyle.Render(e.Message)
		namePad := strings.Repeat(" ", max(0, 24-lipgloss.Width(styledName)))
		msgPad := strings.Repeat(" ", max(0, 30-lipgloss.Width(styledMsg)))
		sb.WriteString(fmt.Sprintf("  %s  %s%s %s%s %s\n",
			errorMark, styledName, namePad, styledMsg, msgPad, value))
	}

	sb.WriteString(fmt.Sprintf("\n  %s", msgStyle.Render(fmt.Sprintf("%d error(s) found in .env", len(errors)))))

	fmt.Println(errorBox.Width(terminalWidth()).Render(sb.String()))
}

func renderSuccess(msg string) {
	content := fmt.Sprintf("%s  %s", successMark, msg)
	fmt.Println(successBox.Width(terminalWidth()).Render(titleSuccessStyle.Render("All Good") + "\n\n  " + content))
}

func renderWarning(msg string) {
	content := fmt.Sprintf("%s  %s", errorMark, msg)
	fmt.Println(errorBox.Width(terminalWidth()).Render(titleErrorStyle.Render("Warning") + "\n\n  " + content))
}
