// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package lint

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"git.minetest.land/VoxeLibre/voxelibre-test/internal/luals"
)

const containerCheckoutPath = "/path/to/voxelibre"

type Counts struct {
	Errors      int
	Warnings    int
	Information int
	Hints       int
}

// Diagnostic is a display-independent diagnostic with a host path and one-based
// positions. EndColumn is exclusive, as in the LuaLS report.
type Diagnostic struct {
	File      string
	Line      int
	EndLine   int
	Column    int
	EndColumn int
	Severity  luals.Severity
	Code      string
	Message   string
}

func ConvertDiagnostics(
	diagnostics []luals.Diagnostic,
	cloneDirectory string,
	workingDirectory string,
	checkLevel luals.CheckLevel,
) ([]Diagnostic, Counts, error) {
	maximumSeverity, err := maximumSeverity(checkLevel)
	if err != nil {
		return nil, Counts{}, err
	}
	converted := make([]Diagnostic, 0, len(diagnostics))
	var counts Counts
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity > maximumSeverity {
			continue
		}
		file, err := annotationFile(diagnostic.URI, cloneDirectory, workingDirectory)
		if err != nil {
			return nil, Counts{}, fmt.Errorf("convert LuaLS diagnostic %q: %w", diagnostic.Code, err)
		}
		switch diagnostic.Severity {
		case luals.SeverityError:
			counts.Errors++
		case luals.SeverityWarning:
			counts.Warnings++
		case luals.SeverityInformation:
			counts.Information++
		case luals.SeverityHint:
			counts.Hints++
		default:
			return nil, Counts{}, fmt.Errorf("unsupported severity %d", diagnostic.Severity)
		}
		converted = append(converted, Diagnostic{
			File:      file,
			Line:      diagnostic.Range.Start.Line + 1,
			EndLine:   diagnostic.Range.End.Line + 1,
			Column:    diagnostic.Range.Start.Character + 1,
			EndColumn: diagnostic.Range.End.Character + 1,
			Severity:  diagnostic.Severity,
			Code:      diagnostic.Code,
			Message:   diagnostic.Message,
		})
	}

	sort.Slice(converted, func(first, second int) bool {
		left := converted[first]
		right := converted[second]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.EndLine != right.EndLine {
			return left.EndLine < right.EndLine
		}
		if left.EndColumn != right.EndColumn {
			return left.EndColumn < right.EndColumn
		}
		if left.Severity != right.Severity {
			return left.Severity < right.Severity
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return left.Message < right.Message
	})
	return converted, counts, nil
}

func maximumSeverity(checkLevel luals.CheckLevel) (luals.Severity, error) {
	switch checkLevel {
	case luals.CheckLevelError:
		return luals.SeverityError, nil
	case luals.CheckLevelWarning:
		return luals.SeverityWarning, nil
	case luals.CheckLevelInformation:
		return luals.SeverityInformation, nil
	case luals.CheckLevelHint:
		return luals.SeverityHint, nil
	default:
		return 0, fmt.Errorf("unsupported LuaLS check level %q", checkLevel)
	}
}

func annotationFile(uri, cloneDirectory, workingDirectory string) (string, error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("parse file URI %q: %w", uri, err)
	}
	if parsedURI.Scheme != "file" || (parsedURI.Host != "" && parsedURI.Host != "localhost") {
		return "", fmt.Errorf("unsupported diagnostic URI %q", uri)
	}
	containerPath := filepath.Clean(parsedURI.Path)
	relativeToContainer, err := filepath.Rel(containerCheckoutPath, containerPath)
	if err != nil {
		return "", fmt.Errorf("resolve diagnostic path %q: %w", containerPath, err)
	}
	if relativeToContainer == ".." || strings.HasPrefix(relativeToContainer, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("diagnostic path %q is outside %s", containerPath, containerCheckoutPath)
	}

	hostPath := filepath.Join(cloneDirectory, relativeToContainer)
	relativeToWorkingDirectory, err := filepath.Rel(workingDirectory, hostPath)
	if err == nil && relativeToWorkingDirectory != ".." &&
		!strings.HasPrefix(relativeToWorkingDirectory, ".."+string(filepath.Separator)) {
		hostPath = relativeToWorkingDirectory
	}
	return filepath.ToSlash(hostPath), nil
}
