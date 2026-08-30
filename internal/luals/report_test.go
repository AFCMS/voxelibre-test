// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package luals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReportReadsLuaLSDiagnostics(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "check.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	diagnostics, err := ParseReport(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("diagnostics = %d, want 4", len(diagnostics))
	}

	counts := map[Severity]int{}
	var encodedPathDiagnostic *Diagnostic
	for index := range diagnostics {
		diagnostic := &diagnostics[index]
		counts[diagnostic.Severity]++
		if strings.Contains(diagnostic.URI, "%2B") && diagnostic.Code == "undefined-field" {
			encodedPathDiagnostic = diagnostic
		}
	}
	for _, severity := range []Severity{SeverityError, SeverityWarning, SeverityInformation, SeverityHint} {
		if counts[severity] != 1 {
			t.Fatalf("severity %s count = %d, want 1", severity, counts[severity])
		}
	}
	if encodedPathDiagnostic == nil || encodedPathDiagnostic.Code != "undefined-field" ||
		encodedPathDiagnostic.Range.Start != (Position{Line: 278, Character: 25}) {
		t.Fatalf("encoded-path diagnostic = %#v", encodedPathDiagnostic)
	}
}

func TestParseReportAcceptsEmptyObject(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "empty.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	diagnostics, err := ParseReport(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
}

func TestParseReportRejectsMalformedFixture(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "malformed.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := ParseReport(file); err == nil || !strings.Contains(err.Error(), "range end precedes") {
		t.Fatalf("error = %v, want invalid range", err)
	}
}

func TestParseReportRejectsInvalidStructures(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "null report", json: `null`, want: "expected a JSON object"},
		{name: "missing severity", json: `{"file:///path/to/voxelibre/a.lua":[{"code":"x","message":"m","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]}`, want: "severity is required"},
		{name: "unknown severity", json: `{"file:///path/to/voxelibre/a.lua":[{"code":"x","message":"m","severity":5,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]}`, want: "unsupported severity"},
		{name: "trailing value", json: `{} {}`, want: "trailing JSON value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseReport(strings.NewReader(test.json))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseCheckLevel(t *testing.T) {
	for input, want := range map[string]CheckLevel{
		"error":       CheckLevelError,
		" Warning ":   CheckLevelWarning,
		"INFORMATION": CheckLevelInformation,
		"hint":        CheckLevelHint,
	} {
		level, err := ParseCheckLevel(input)
		if err != nil {
			t.Fatalf("ParseCheckLevel(%q): %v", input, err)
		}
		if level != want {
			t.Fatalf("ParseCheckLevel(%q) = %q, want %q", input, level, want)
		}
	}
	if _, err := ParseCheckLevel("notice"); err == nil {
		t.Fatal("expected invalid check level error")
	}
	if CheckLevelInformation.LuaLSArgument() != "Information" {
		t.Fatalf("LuaLS argument = %q", CheckLevelInformation.LuaLSArgument())
	}
}
