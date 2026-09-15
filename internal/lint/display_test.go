// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package lint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/container"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luals"
)

func TestHumanDisplaySourceAndMultilineMessage(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "init.lua"), []byte("-- example\r\n\tlocal café = missing\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	display, err := NewHumanDisplay(&output, "never")
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := Diagnostic{
		File: "init.lua", Line: 2, EndLine: 2, Column: 15, EndColumn: 22,
		Severity: luals.SeverityError, Code: "undefined-global",
		Message: "Unknown global (100%).\nDeclare it first.",
	}
	if err := display.WriteDiagnostic(diagnostic, directory); err != nil {
		t.Fatal(err)
	}
	want := "init.lua:2:15 - error [undefined-global]: Unknown global (100%).\n  Declare it first.\n\n" +
		"  2 |     local café = missing\n" +
		"    |                  ~~~~~~~\n\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestHumanDisplaySeveritiesAndColors(t *testing.T) {
	for _, mode := range []string{"auto", "always", "never"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			t.Setenv("TERM", "dumb")
			var output bytes.Buffer
			display, err := NewHumanDisplay(&output, mode)
			if err != nil {
				t.Fatal(err)
			}
			for _, severity := range []luals.Severity{luals.SeverityError, luals.SeverityWarning, luals.SeverityInformation, luals.SeverityHint} {
				diagnostic := Diagnostic{File: "missing.lua", Line: 1, Column: 2, Severity: severity, Code: "code", Message: "message"}
				if err := display.WriteDiagnostic(diagnostic, t.TempDir()); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), strings.ToLower(severity.String())) {
					t.Fatalf("missing severity in %q", output.String())
				}
			}
			if strings.Contains(output.String(), "\x1b[") != (mode == "always") {
				t.Fatalf("unexpected color for %s: %q", mode, output.String())
			}
			if strings.Contains(output.String(), "::") {
				t.Fatalf("workflow commands in human output: %q", output.String())
			}
			if mode == "always" {
				for _, label := range []string{"\x1b[31merror\x1b[0m", "\x1b[33mwarning\x1b[0m", "\x1b[36minformation\x1b[0m", "\x1b[36mhint\x1b[0m"} {
					if !strings.Contains(output.String(), label) {
						t.Fatalf("missing colored label %q", label)
					}
				}
			}
		})
	}
	if _, err := NewHumanDisplay(io.Discard, "invalid"); err == nil {
		t.Fatal("accepted invalid color mode")
	}
}

func TestHumanDisplayAutoColorForRedirectedOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	display, err := NewHumanDisplay(file, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if display.(*humanDisplay).color {
		t.Fatal("auto enabled color for a regular file")
	}
}

func TestHumanDisplaySourceRanges(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "a.lua"), []byte("abc\ndef"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                             string
		line, column, endLine, endColumn int
		marker                           string
	}{
		{"multiline", 1, 2, 2, 2, "  |  ~~"},
		{"empty range", 1, 2, 1, 2, "  |  ~"},
		{"past line end", 1, 100, 1, 200, "  |    ~"},
		{"missing line", 4, 1, 4, 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			display, _ := NewHumanDisplay(&output, "never")
			err := display.WriteDiagnostic(Diagnostic{
				File: "a.lua", Line: tc.line, Column: tc.column, EndLine: tc.endLine, EndColumn: tc.endColumn,
				Severity: luals.SeverityWarning, Code: "code", Message: "message",
			}, directory)
			if err != nil {
				t.Fatal(err)
			}
			if tc.marker == "" {
				if strings.Contains(output.String(), " | ") {
					t.Fatalf("unexpected excerpt: %q", output.String())
				}
			} else if !strings.Contains(output.String(), tc.marker+"\n") {
				t.Fatalf("missing marker %q: %q", tc.marker, output.String())
			}
		})
	}
	if got := sourceColumn("\té𝄞x", 4); got != 6 {
		t.Fatalf("UTF-16 column = %d, want 6", got)
	}
}

func TestDisplayLogsAndSummary(t *testing.T) {
	for _, format := range []string{"human", "github"} {
		for _, logs := range []string{"", "done", "done\n"} {
			t.Run(format+"/"+logs, func(t *testing.T) {
				var output bytes.Buffer
				var display Display = NewGitHubDisplay(&output)
				want := "::group::LuaLS logs\n"
				if format == "human" {
					display, _ = NewHumanDisplay(&output, "never")
					want = "LuaLS logs\n"
				}
				if logs != "" {
					want += strings.TrimSuffix(logs, "\n") + "\n"
				}
				if format == "github" {
					want += "::endgroup::\n"
				} else {
					want += "\n"
				}
				want += "LuaLS: 1 errors, 2 warnings, 3 information, 4 hints\n"
				if err := display.WriteLogs([]byte(logs)); err != nil {
					t.Fatal(err)
				}
				if err := display.WriteSummary(Counts{1, 2, 3, 4}); err != nil {
					t.Fatal(err)
				}
				if output.String() != want {
					t.Fatalf("output = %q, want %q", output.String(), want)
				}
			})
		}
	}
}

type failingDisplayWriter struct{ err error }

func (writer failingDisplayWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestDisplayWriteFailures(t *testing.T) {
	failure := errors.New("output failed")
	for _, format := range []string{"human", "github"} {
		t.Run(format, func(t *testing.T) {
			writer := failingDisplayWriter{failure}
			var display Display = NewGitHubDisplay(writer)
			if format == "human" {
				display, _ = NewHumanDisplay(writer, "never")
			}
			for _, err := range []error{
				display.WriteLogs([]byte("logs")),
				display.WriteDiagnostic(Diagnostic{Severity: luals.SeverityError}, t.TempDir()),
				display.WriteSummary(Counts{}),
			} {
				if !errors.Is(err, failure) {
					t.Fatalf("error = %v, want output failure", err)
				}
			}
			runtime := &fakeRuntime{report: `{}`}
			runner := NewRunner(runtime, "tools", container.PullNever, "/clone", luals.CheckLevelWarning, display)
			if err := runner.Run(context.Background()); !errors.Is(err, failure) || !runtime.removed {
				t.Fatalf("run error = %v, removed = %v", err, runtime.removed)
			}
		})
	}
}

func TestHumanRunnerReportsErrorsAndCleansUp(t *testing.T) {
	runtime := &fakeRuntime{waitCode: 1, report: `{"file:///path/to/voxelibre/a.lua":[{
"code":"syntax-error","message":"Unexpected token.","severity":1,
"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}
}]}`}
	var output bytes.Buffer
	display, _ := NewHumanDisplay(&output, "never")
	runner := NewRunner(runtime, "tools", container.PullNever, "/clone", luals.CheckLevelWarning, display)
	if err := runner.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "reported 1 error diagnostics") {
		t.Fatalf("error = %v", err)
	}
	if !runtime.removed || !strings.Contains(output.String(), "a.lua:1:1 - error [syntax-error]") || !strings.Contains(output.String(), "LuaLS: 1 errors") {
		t.Fatalf("removed = %v, output = %q", runtime.removed, output.String())
	}
}
