// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package luals

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Severity int

const (
	SeverityError       Severity = 1
	SeverityWarning     Severity = 2
	SeverityInformation Severity = 3
	SeverityHint        Severity = 4
)

func (severity Severity) String() string {
	switch severity {
	case SeverityError:
		return "Error"
	case SeverityWarning:
		return "Warning"
	case SeverityInformation:
		return "Information"
	case SeverityHint:
		return "Hint"
	default:
		return fmt.Sprintf("Severity(%d)", severity)
	}
}

type CheckLevel string

const (
	CheckLevelError       CheckLevel = "error"
	CheckLevelWarning     CheckLevel = "warning"
	CheckLevelInformation CheckLevel = "information"
	CheckLevelHint        CheckLevel = "hint"
)

func ParseCheckLevel(value string) (CheckLevel, error) {
	level := CheckLevel(strings.ToLower(strings.TrimSpace(value)))
	switch level {
	case CheckLevelError, CheckLevelWarning, CheckLevelInformation, CheckLevelHint:
		return level, nil
	default:
		return "", fmt.Errorf(
			"unsupported LuaLS check level %q: expected error, warning, information, or hint",
			value,
		)
	}
}

func (level CheckLevel) LuaLSArgument() string {
	switch level {
	case CheckLevelError:
		return SeverityError.String()
	case CheckLevelWarning:
		return SeverityWarning.String()
	case CheckLevelInformation:
		return SeverityInformation.String()
	case CheckLevelHint:
		return SeverityHint.String()
	default:
		return string(level)
	}
}

type Position struct {
	Line      int
	Character int
}

type Range struct {
	Start Position
	End   Position
}

type Diagnostic struct {
	URI      string
	Code     string
	Message  string
	Range    Range
	Severity Severity
	Source   string
}

type rawPosition struct {
	Line      *int `json:"line"`
	Character *int `json:"character"`
}

type rawRange struct {
	Start *rawPosition `json:"start"`
	End   *rawPosition `json:"end"`
}

type rawDiagnostic struct {
	Code     *string   `json:"code"`
	Message  *string   `json:"message"`
	Range    *rawRange `json:"range"`
	Severity *Severity `json:"severity"`
	Source   string    `json:"source"`
}

func ParseReport(input io.Reader) ([]Diagnostic, error) {
	decoder := json.NewDecoder(input)
	var report map[string][]rawDiagnostic
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("decode LuaLS report: %w", err)
	}
	if report == nil {
		return nil, errors.New("decode LuaLS report: expected a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("decode LuaLS report: unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("decode LuaLS report trailer: %w", err)
	}

	diagnostics := make([]Diagnostic, 0)
	for uri, entries := range report {
		if strings.TrimSpace(uri) == "" {
			return nil, errors.New("validate LuaLS report: diagnostic URI must not be empty")
		}
		for index, entry := range entries {
			diagnostic, err := validateDiagnostic(uri, index, entry)
			if err != nil {
				return nil, err
			}
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	return diagnostics, nil
}

func validateDiagnostic(uri string, index int, raw rawDiagnostic) (Diagnostic, error) {
	prefix := fmt.Sprintf("validate LuaLS diagnostic %q[%d]", uri, index)
	if raw.Code == nil || strings.TrimSpace(*raw.Code) == "" {
		return Diagnostic{}, fmt.Errorf("%s: code must not be empty", prefix)
	}
	if raw.Message == nil || strings.TrimSpace(*raw.Message) == "" {
		return Diagnostic{}, fmt.Errorf("%s: message must not be empty", prefix)
	}
	if raw.Severity == nil {
		return Diagnostic{}, fmt.Errorf("%s: severity is required", prefix)
	}
	switch *raw.Severity {
	case SeverityError, SeverityWarning, SeverityInformation, SeverityHint:
	default:
		return Diagnostic{}, fmt.Errorf("%s: unsupported severity %d", prefix, *raw.Severity)
	}
	if raw.Range == nil || raw.Range.Start == nil || raw.Range.End == nil {
		return Diagnostic{}, fmt.Errorf("%s: range start and end are required", prefix)
	}
	start, err := validatePosition(prefix+" start", *raw.Range.Start)
	if err != nil {
		return Diagnostic{}, err
	}
	end, err := validatePosition(prefix+" end", *raw.Range.End)
	if err != nil {
		return Diagnostic{}, err
	}
	if end.Line < start.Line || (end.Line == start.Line && end.Character < start.Character) {
		return Diagnostic{}, fmt.Errorf("%s: range end precedes its start", prefix)
	}
	return Diagnostic{
		URI:      uri,
		Code:     *raw.Code,
		Message:  *raw.Message,
		Range:    Range{Start: start, End: end},
		Severity: *raw.Severity,
		Source:   raw.Source,
	}, nil
}

func validatePosition(prefix string, raw rawPosition) (Position, error) {
	if raw.Line == nil || raw.Character == nil {
		return Position{}, fmt.Errorf("%s: line and character are required", prefix)
	}
	if *raw.Line < 0 || *raw.Character < 0 {
		return Position{}, fmt.Errorf("%s: line and character must not be negative", prefix)
	}
	return Position{Line: *raw.Line, Character: *raw.Character}, nil
}
