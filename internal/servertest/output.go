// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// logPresenter passes engine output through unchanged and replaces complete
// unit-test records with a readable view. Partial lines span log snapshots.
type logPresenter struct {
	output   io.Writer
	previous []byte
	pending  []byte
	parser   resultParser
	parseErr error
}

func (presenter *logPresenter) snapshot(logs []byte) error {
	if !bytes.HasPrefix(logs, presenter.previous) {
		return fmt.Errorf("container log history changed during polling")
	}
	presenter.pending = append(presenter.pending, logs[len(presenter.previous):]...)
	presenter.previous = append(presenter.previous[:0], logs...)
	for {
		index := bytes.IndexByte(presenter.pending, '\n')
		if index < 0 {
			return nil
		}
		line := string(presenter.pending[:index+1])
		presenter.pending = presenter.pending[index+1:]
		if err := presenter.line(line); err != nil {
			return err
		}
	}
}

func (presenter *logPresenter) flush() error {
	if len(presenter.pending) == 0 {
		return nil
	}
	line := string(presenter.pending)
	presenter.pending = nil
	return presenter.line(line)
}

func (presenter *logPresenter) results() (suiteResults, error) {
	if presenter.parseErr != nil {
		return presenter.parser.results, presenter.parseErr
	}
	return presenter.parser.results, presenter.parser.finish()
}

func (presenter *logPresenter) line(line string) error {
	record, err := presenter.parser.consume(line)
	if err != nil && presenter.parseErr == nil {
		presenter.parseErr = err
	}
	if record == nil {
		// Retain malformed records too, so protocol errors have visible context.
		_, err := io.WriteString(presenter.output, line)
		return err
	}
	if record.Event == "summary" {
		_, err := fmt.Fprintf(presenter.output, "  Tests: %.0f passed, %.0f failed\n", *record.Passed, *record.Failed)
		return err
	}
	if _, err := fmt.Fprintf(presenter.output, "  %s  %s / %s (%s)\n",
		strings.ToUpper(record.Status), singleLine(record.Mod), singleLine(record.Name), formatDuration(*record.Duration)); err != nil {
		return err
	}
	if record.Status == "fail" {
		for _, line := range strings.Split(readableTraceback(record.Traceback), "\n") {
			if _, err := fmt.Fprintf(presenter.output, "        %s\n", line); err != nil {
				return err
			}
		}
	}
	return nil
}

func singleLine(text string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`, "\t", `\t`).Replace(text)
}

func formatDuration(seconds float64) string {
	switch {
	case seconds < 0.001:
		return fmt.Sprintf("%.0f µs", seconds*1e6)
	case seconds < 1:
		return fmt.Sprintf("%.2f ms", seconds*1e3)
	default:
		return fmt.Sprintf("%.2f s", seconds)
	}
}

// Lua truncates long source paths with "...". Shorten only recognizable game
// paths; never invent the missing part of an unrelated source location.
var truncatedGamePath = regexp.MustCompile(`\.\.\.[^\s"'<>]*/mods/`)

func readableTraceback(trace string) string {
	trace = strings.ReplaceAll(trace, "/var/lib/minetest/games/voxelibre/", "")
	trace = truncatedGamePath.ReplaceAllString(trace, "mods/")
	trace = strings.ReplaceAll(trace, "\r\n", "\n")
	return strings.ReplaceAll(trace, "\t", "  ")
}
