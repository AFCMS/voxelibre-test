// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package workflowcmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
	LevelNotice  Level = "notice"
)

type Annotation struct {
	Level     Level
	Title     string
	Message   string
	File      string
	Line      int
	EndLine   int
	Column    int
	EndColumn int
}

func WriteAnnotation(output io.Writer, annotation Annotation) error {
	switch annotation.Level {
	case LevelError, LevelWarning, LevelNotice:
	default:
		return fmt.Errorf("unsupported workflow annotation level %q", annotation.Level)
	}

	properties := make([]string, 0, 6)
	if annotation.File != "" {
		properties = append(properties, "file="+EscapeProperty(annotation.File))
	}
	if annotation.Line > 0 {
		properties = append(properties, fmt.Sprintf("line=%d", annotation.Line))
	}
	if annotation.EndLine > 0 {
		properties = append(properties, fmt.Sprintf("endLine=%d", annotation.EndLine))
	}
	if annotation.Column > 0 {
		properties = append(properties, fmt.Sprintf("col=%d", annotation.Column))
	}
	if annotation.EndColumn > 0 {
		properties = append(properties, fmt.Sprintf("endColumn=%d", annotation.EndColumn))
	}
	if annotation.Title != "" {
		properties = append(properties, "title="+EscapeProperty(annotation.Title))
	}

	command := "::" + string(annotation.Level)
	if len(properties) > 0 {
		command += " " + strings.Join(properties, ",")
	}
	if _, err := fmt.Fprintf(output, "%s::%s\n", command, EscapeData(annotation.Message)); err != nil {
		return fmt.Errorf("write workflow annotation: %w", err)
	}
	return nil
}

type Group struct {
	output              io.Writer
	wroteOutput         bool
	outputEndsInNewline bool
}

func StartGroup(output io.Writer, title string) (*Group, error) {
	if _, err := fmt.Fprintf(output, "::group::%s\n", EscapeData(title)); err != nil {
		return nil, fmt.Errorf("start workflow log group: %w", err)
	}
	return &Group{output: output}, nil
}

func (group *Group) Write(data []byte) (int, error) {
	written, err := group.output.Write(data)
	if written > 0 {
		group.wroteOutput = true
		group.outputEndsInNewline = data[written-1] == '\n'
	}
	return written, err
}

func (group *Group) End() error {
	var resultErr error
	if group.wroteOutput && !group.outputEndsInNewline {
		if _, err := fmt.Fprintln(group.output); err != nil {
			resultErr = fmt.Errorf("finish workflow log line: %w", err)
		}
	}
	if _, err := fmt.Fprintln(group.output, "::endgroup::"); err != nil {
		resultErr = errors.Join(resultErr, fmt.Errorf("end workflow log group: %w", err))
	}
	return resultErr
}

func EscapeData(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	)
	return replacer.Replace(value)
}

func EscapeProperty(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
		":", "%3A",
		",", "%2C",
	)
	return replacer.Replace(value)
}
