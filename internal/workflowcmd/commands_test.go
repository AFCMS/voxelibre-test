// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package workflowcmd

import (
	"bytes"
	"testing"
)

func TestWriteAnnotationFormatsAndEscapesProperties(t *testing.T) {
	var output bytes.Buffer
	err := WriteAnnotation(&output, Annotation{
		Level:     LevelWarning,
		Title:     "LuaLS Warning: undefined,field",
		Message:   "failed 100%\r\nretry",
		File:      "VoxeLibre/mods/a,b.lua",
		Line:      2,
		EndLine:   3,
		Column:    4,
		EndColumn: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "::warning file=VoxeLibre/mods/a%2Cb.lua,line=2,endLine=3,col=4,endColumn=5,title=LuaLS Warning%3A undefined%2Cfield::failed 100%25%0D%0Aretry\n"
	if output.String() != want {
		t.Fatalf("annotation = %q, want %q", output.String(), want)
	}
}

func TestWriteAnnotationRejectsUnknownLevel(t *testing.T) {
	if err := WriteAnnotation(&bytes.Buffer{}, Annotation{Level: "debug"}); err == nil {
		t.Fatal("expected unsupported level error")
	}
}

func TestGroupClosesOnItsOwnLine(t *testing.T) {
	var output bytes.Buffer
	group, err := StartGroup(&output, "LuaLS 100%\nlogs")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := group.Write([]byte("diagnosis complete")); err != nil {
		t.Fatal(err)
	}
	if err := group.End(); err != nil {
		t.Fatal(err)
	}
	want := "::group::LuaLS 100%25%0Alogs\ndiagnosis complete\n::endgroup::\n"
	if output.String() != want {
		t.Fatalf("group = %q, want %q", output.String(), want)
	}
}
