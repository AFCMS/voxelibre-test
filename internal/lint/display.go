// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package lint

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luals"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/workflowcmd"
	"golang.org/x/term"
)

// Display renders all lint output without affecting filtering or exit status.
type Display interface {
	WriteLogs([]byte) error
	WriteDiagnostic(Diagnostic, string) error
	WriteSummary(Counts) error
}

type displayOutput struct {
	output io.Writer
}

func newDisplayOutput(output io.Writer) displayOutput {
	if output == nil {
		output = io.Discard
	}
	return displayOutput{output: output}
}

func (display displayOutput) WriteSummary(counts Counts) error {
	_, err := fmt.Fprintf(display.output,
		"LuaLS: %d errors, %d warnings, %d information, %d hints\n",
		counts.Errors, counts.Warnings, counts.Information, counts.Hints)
	if err != nil {
		return fmt.Errorf("write LuaLS summary: %w", err)
	}
	return nil
}

type githubDisplay struct {
	displayOutput
}

func NewGitHubDisplay(output io.Writer) Display {
	return &githubDisplay{newDisplayOutput(output)}
}

func (display *githubDisplay) WriteLogs(logs []byte) error {
	group, err := workflowcmd.StartGroup(display.output, "LuaLS logs")
	if err != nil {
		return err
	}
	var writeErr error
	if len(logs) > 0 {
		_, writeErr = group.Write(logs)
		if writeErr != nil {
			writeErr = fmt.Errorf("write LuaLS container logs: %w", writeErr)
		}
	}
	return errors.Join(writeErr, group.End())
}

func (display *githubDisplay) WriteDiagnostic(diagnostic Diagnostic, _ string) error {
	var level workflowcmd.Level
	switch diagnostic.Severity {
	case luals.SeverityError:
		level = workflowcmd.LevelError
	case luals.SeverityWarning:
		level = workflowcmd.LevelWarning
	case luals.SeverityInformation, luals.SeverityHint:
		level = workflowcmd.LevelNotice
	default:
		return fmt.Errorf("unsupported severity %d", diagnostic.Severity)
	}
	return workflowcmd.WriteAnnotation(display.output, workflowcmd.Annotation{
		Level:     level,
		Title:     fmt.Sprintf("LuaLS %s: %s", diagnostic.Severity, diagnostic.Code),
		Message:   diagnostic.Message,
		File:      diagnostic.File,
		Line:      diagnostic.Line,
		EndLine:   diagnostic.EndLine,
		Column:    diagnostic.Column,
		EndColumn: diagnostic.EndColumn,
	})
}

type humanDisplay struct {
	displayOutput
	color bool
	// Diagnostics are sorted by file, so retain only the current source file.
	sourcePath  string
	sourceLines []string
}

func NewHumanDisplay(output io.Writer, color string) (Display, error) {
	useColor := false
	switch color {
	case "always":
		useColor = true
	case "never":
	case "auto":
		if file, ok := output.(*os.File); ok {
			useColor = os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" && term.IsTerminal(int(file.Fd()))
		}
	default:
		return nil, fmt.Errorf("unsupported color mode %q: expected auto, always, or never", color)
	}
	return &humanDisplay{displayOutput: newDisplayOutput(output), color: useColor}, nil
}

func (display *humanDisplay) style(code, value string) string {
	if !display.color {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func (display *humanDisplay) WriteLogs(logs []byte) error {
	var text strings.Builder
	fmt.Fprintln(&text, display.style("1", "LuaLS logs"))
	text.WriteString(readableText(string(logs)))
	if len(logs) > 0 && logs[len(logs)-1] != '\n' {
		text.WriteByte('\n')
	}
	text.WriteByte('\n')
	if _, err := io.WriteString(display.output, text.String()); err != nil {
		return fmt.Errorf("write LuaLS container logs: %w", err)
	}
	return nil
}

func (display *humanDisplay) WriteDiagnostic(diagnostic Diagnostic, workingDirectory string) error {
	severityColor := "36"
	switch diagnostic.Severity {
	case luals.SeverityError:
		severityColor = "31"
	case luals.SeverityWarning:
		severityColor = "33"
	case luals.SeverityInformation, luals.SeverityHint:
	default:
		return fmt.Errorf("unsupported severity %d", diagnostic.Severity)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "%s:%s:%s - %s %s: %s\n",
		display.style("36", readableText(diagnostic.File)),
		display.style("33", fmt.Sprint(diagnostic.Line)),
		display.style("33", fmt.Sprint(diagnostic.Column)),
		display.style(severityColor, strings.ToLower(diagnostic.Severity.String())),
		display.style("90", "["+readableText(diagnostic.Code)+"]"),
		strings.ReplaceAll(readableText(diagnostic.Message), "\n", "\n  "))

	if line, ok := display.sourceLine(diagnostic, workingDirectory); ok {
		start := sourceColumn(line, diagnostic.Column-1)
		end := len([]rune(expandTabs(line)))
		if diagnostic.EndLine == diagnostic.Line {
			end = sourceColumn(line, diagnostic.EndColumn-1)
		}
		gutter := fmt.Sprint(diagnostic.Line)
		fmt.Fprintf(&text, "\n  %s | %s\n  %s | %s%s\n",
			display.style("90", gutter), readableText(expandTabs(line)),
			strings.Repeat(" ", len(gutter)), strings.Repeat(" ", start),
			display.style(severityColor, strings.Repeat("~", max(1, end-start))))
	}
	text.WriteByte('\n')
	if _, err := io.WriteString(display.output, text.String()); err != nil {
		return fmt.Errorf("write LuaLS diagnostic: %w", err)
	}
	return nil
}

func (display *humanDisplay) sourceLine(diagnostic Diagnostic, workingDirectory string) (string, bool) {
	path := filepath.FromSlash(diagnostic.File)
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	if path != display.sourcePath {
		display.sourcePath = path
		display.sourceLines = nil
		// A missing or changed source file must not hide the diagnostic itself.
		if source, err := os.ReadFile(path); err == nil {
			display.sourceLines = strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
		}
	}
	if diagnostic.Line < 1 || diagnostic.Line > len(display.sourceLines) {
		return "", false
	}
	return display.sourceLines[diagnostic.Line-1], true
}

func expandTabs(line string) string {
	return strings.ReplaceAll(line, "\t", "    ")
}

// LuaLS columns count UTF-16 code units. Convert to displayed characters,
// expanding tabs consistently with the source excerpt and clamping at its end.
func sourceColumn(line string, character int) int {
	units, column := 0, 0
	for _, r := range line {
		if units >= character {
			break
		}
		units++
		if r > 0xffff {
			units++
		}
		if r == '\t' {
			column += 4
		} else {
			column++
		}
	}
	return column
}

// Keep multiline text readable while preventing embedded terminal controls.
func readableText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		if (r < ' ' && r != '\n' && r != '\t') || (r >= 0x7f && r <= 0x9f) {
			return '\uFFFD'
		}
		return r
	}, value)
}
