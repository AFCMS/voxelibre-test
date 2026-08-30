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
	"git.minetest.land/VoxeLibre/voxelibre-test/internal/workflowcmd"
)

const containerCheckoutPath = "/path/to/voxelibre"

type Counts struct {
	Errors      int
	Warnings    int
	Information int
	Hints       int
}

type convertedAnnotation struct {
	annotation workflowcmd.Annotation
	severity   luals.Severity
	code       string
}

func ConvertDiagnostics(
	diagnostics []luals.Diagnostic,
	cloneDirectory string,
	workingDirectory string,
	checkLevel luals.CheckLevel,
) ([]workflowcmd.Annotation, Counts, error) {
	maximumSeverity, err := maximumSeverity(checkLevel)
	if err != nil {
		return nil, Counts{}, err
	}
	converted := make([]convertedAnnotation, 0, len(diagnostics))
	var counts Counts
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity > maximumSeverity {
			continue
		}
		file, err := annotationFile(diagnostic.URI, cloneDirectory, workingDirectory)
		if err != nil {
			return nil, Counts{}, fmt.Errorf("convert LuaLS diagnostic %q: %w", diagnostic.Code, err)
		}
		level, err := annotationLevel(diagnostic.Severity)
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
		}
		converted = append(converted, convertedAnnotation{
			annotation: workflowcmd.Annotation{
				Level:     level,
				Title:     fmt.Sprintf("LuaLS %s: %s", diagnostic.Severity, diagnostic.Code),
				Message:   diagnostic.Message,
				File:      file,
				Line:      diagnostic.Range.Start.Line + 1,
				EndLine:   diagnostic.Range.End.Line + 1,
				Column:    diagnostic.Range.Start.Character + 1,
				EndColumn: diagnostic.Range.End.Character + 1,
			},
			severity: diagnostic.Severity,
			code:     diagnostic.Code,
		})
	}

	sort.Slice(converted, func(first, second int) bool {
		left := converted[first]
		right := converted[second]
		leftAnnotation := left.annotation
		rightAnnotation := right.annotation
		if leftAnnotation.File != rightAnnotation.File {
			return leftAnnotation.File < rightAnnotation.File
		}
		if leftAnnotation.Line != rightAnnotation.Line {
			return leftAnnotation.Line < rightAnnotation.Line
		}
		if leftAnnotation.Column != rightAnnotation.Column {
			return leftAnnotation.Column < rightAnnotation.Column
		}
		if leftAnnotation.EndLine != rightAnnotation.EndLine {
			return leftAnnotation.EndLine < rightAnnotation.EndLine
		}
		if leftAnnotation.EndColumn != rightAnnotation.EndColumn {
			return leftAnnotation.EndColumn < rightAnnotation.EndColumn
		}
		if left.severity != right.severity {
			return left.severity < right.severity
		}
		if left.code != right.code {
			return left.code < right.code
		}
		return leftAnnotation.Message < rightAnnotation.Message
	})
	annotations := make([]workflowcmd.Annotation, len(converted))
	for index, item := range converted {
		annotations[index] = item.annotation
	}
	return annotations, counts, nil
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

func annotationLevel(severity luals.Severity) (workflowcmd.Level, error) {
	switch severity {
	case luals.SeverityError:
		return workflowcmd.LevelError, nil
	case luals.SeverityWarning:
		return workflowcmd.LevelWarning, nil
	case luals.SeverityInformation, luals.SeverityHint:
		return workflowcmd.LevelNotice, nil
	default:
		return "", fmt.Errorf("unsupported severity %d", severity)
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
