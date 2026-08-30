// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package lint

import (
	"bytes"
	"strings"
	"testing"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luals"
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/workflowcmd"
)

func TestConvertDiagnosticsMapsPathsSeveritiesAndRanges(t *testing.T) {
	diagnostics := []luals.Diagnostic{
		{
			URI:      "file:///path/to/voxelibre/mods/z.lua",
			Code:     "hint-code",
			Message:  "Hint",
			Severity: luals.SeverityHint,
			Range: luals.Range{
				Start: luals.Position{Line: 4, Character: 1},
				End:   luals.Position{Line: 4, Character: 2},
			},
		},
		{
			URI:      "file:///path/to/voxelibre/mods/a%2Bb.lua",
			Code:     "bad,field",
			Message:  "Bad 100%\nfield",
			Severity: luals.SeverityError,
			Range: luals.Range{
				Start: luals.Position{Line: 1, Character: 2},
				End:   luals.Position{Line: 2, Character: 4},
			},
		},
		{
			URI:      "file:///path/to/voxelibre/mods/info.lua",
			Code:     "info-code",
			Message:  "Information",
			Severity: luals.SeverityInformation,
			Range: luals.Range{
				Start: luals.Position{Line: 3, Character: 0},
				End:   luals.Position{Line: 3, Character: 1},
			},
		},
		{
			URI:      "file:///path/to/voxelibre/mods/warn.lua",
			Code:     "warn-code",
			Message:  "Warning",
			Severity: luals.SeverityWarning,
			Range: luals.Range{
				Start: luals.Position{Line: 0, Character: 0},
				End:   luals.Position{Line: 0, Character: 1},
			},
		},
	}

	annotations, counts, err := ConvertDiagnostics(
		diagnostics,
		"/workspace/VoxeLibre",
		"/workspace",
		luals.CheckLevelHint,
	)
	if err != nil {
		t.Fatal(err)
	}
	if counts != (Counts{Errors: 1, Warnings: 1, Information: 1, Hints: 1}) {
		t.Fatalf("counts = %#v", counts)
	}
	if len(annotations) != 4 {
		t.Fatalf("annotations = %d, want 4", len(annotations))
	}
	if annotations[0].File != "VoxeLibre/mods/a+b.lua" || annotations[0].Line != 2 ||
		annotations[0].EndLine != 3 || annotations[0].Column != 3 || annotations[0].EndColumn != 5 ||
		annotations[0].Level != workflowcmd.LevelError {
		t.Fatalf("first annotation = %#v", annotations[0])
	}

	var output bytes.Buffer
	for _, annotation := range annotations {
		if err := workflowcmd.WriteAnnotation(&output, annotation); err != nil {
			t.Fatal(err)
		}
	}
	wantFirst := "::error file=VoxeLibre/mods/a+b.lua,line=2,endLine=3,col=3,endColumn=5,title=LuaLS Error%3A bad%2Cfield::Bad 100%25%0Afield\n"
	if !strings.HasPrefix(output.String(), wantFirst) {
		t.Fatalf("output = %q, want prefix %q", output.String(), wantFirst)
	}
	if strings.Count(output.String(), "::notice ") != 2 || strings.Count(output.String(), "::warning ") != 1 {
		t.Fatalf("output severities = %q", output.String())
	}
}

func TestConvertDiagnosticsUsesAbsolutePathOutsideWorkingDirectory(t *testing.T) {
	diagnostic := luals.Diagnostic{
		URI:      "file:///path/to/voxelibre/init.lua",
		Code:     "code",
		Message:  "message",
		Severity: luals.SeverityWarning,
		Range:    luals.Range{Start: luals.Position{}, End: luals.Position{}},
	}
	annotations, _, err := ConvertDiagnostics(
		[]luals.Diagnostic{diagnostic},
		"/outside/VoxeLibre",
		"/workspace",
		luals.CheckLevelWarning,
	)
	if err != nil {
		t.Fatal(err)
	}
	if annotations[0].File != "/outside/VoxeLibre/init.lua" {
		t.Fatalf("file = %q", annotations[0].File)
	}
}

func TestConvertDiagnosticsRejectsUnsupportedPaths(t *testing.T) {
	for _, uri := range []string{
		"https://example.com/a.lua",
		"file:///path/to/other/a.lua",
	} {
		diagnostic := luals.Diagnostic{
			URI: uri, Code: "code", Message: "message", Severity: luals.SeverityWarning,
			Range: luals.Range{Start: luals.Position{}, End: luals.Position{}},
		}
		if _, _, err := ConvertDiagnostics(
			[]luals.Diagnostic{diagnostic},
			"/workspace/VoxeLibre",
			"/workspace",
			luals.CheckLevelWarning,
		); err == nil {
			t.Fatalf("expected URI %q to fail", uri)
		}
	}
}

func TestConvertDiagnosticsAppliesMinimumSeverity(t *testing.T) {
	diagnostics := []luals.Diagnostic{
		{
			URI: "file:///path/to/voxelibre/error.lua", Code: "error", Message: "error",
			Severity: luals.SeverityError,
			Range:    luals.Range{Start: luals.Position{}, End: luals.Position{}},
		},
		{
			URI: "file:///path/to/voxelibre/warning.lua", Code: "warning", Message: "warning",
			Severity: luals.SeverityWarning,
			Range:    luals.Range{Start: luals.Position{}, End: luals.Position{}},
		},
	}
	annotations, counts, err := ConvertDiagnostics(
		diagnostics,
		"/workspace/VoxeLibre",
		"/workspace",
		luals.CheckLevelError,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 1 || counts != (Counts{Errors: 1}) || annotations[0].File != "VoxeLibre/error.lua" {
		t.Fatalf("annotations/counts = %#v/%#v", annotations, counts)
	}
}
