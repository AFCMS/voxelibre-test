// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

const resultPrefix = "[vl_unittests] "

// core.log prefixes messages with a timestamp, level and thread. Match only
// protocol messages at the start of a log message, not text within a traceback.
var resultLine = regexp.MustCompile(`^(?:[^\r\n]*: ACTION\[[^\]]+\]: )?` + regexp.QuoteMeta(resultPrefix) + `(.*)$`)

type testResult struct {
	Event     string   `json:"event"`
	Mod       string   `json:"mod"`
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Duration  *float64 `json:"duration"`
	Traceback string   `json:"traceback"`
	// Older Luanti versions encode integral Lua numbers as JSON decimals.
	Total  *float64 `json:"total"`
	Passed *float64 `json:"passed"`
	Failed *float64 `json:"failed"`
}

type suiteResults struct {
	Tests  []testResult
	Failed int
}

// resultParser validates the same record stream used for live presentation.
type resultParser struct {
	results     suiteResults
	seen        map[[2]string]bool
	summarySeen bool
}

func (parser *resultParser) consume(line string) (*testResult, error) {
	match := resultLine.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
	if match == nil {
		return nil, nil
	}
	var record testResult
	if err := json.Unmarshal([]byte(match[1]), &record); err != nil {
		return nil, fmt.Errorf("malformed unit-test record: %w", err)
	}
	if parser.summarySeen {
		return nil, fmt.Errorf("unexpected unit-test record after final summary")
	}
	switch record.Event {
	case "test":
		if strings.TrimSpace(record.Mod) == "" || strings.TrimSpace(record.Name) == "" ||
			(record.Status != "pass" && record.Status != "fail") ||
			record.Duration == nil || *record.Duration < 0 || math.IsInf(*record.Duration, 0) || math.IsNaN(*record.Duration) ||
			(record.Status == "fail" && record.Traceback == "") {
			return nil, fmt.Errorf("invalid unit-test result for %q:%q", record.Mod, record.Name)
		}
		if parser.seen == nil {
			parser.seen = make(map[[2]string]bool)
		}
		key := [2]string{record.Mod, record.Name}
		if parser.seen[key] {
			return nil, fmt.Errorf("duplicate unit-test result for %s:%s", record.Mod, record.Name)
		}
		parser.seen[key] = true
		parser.results.Tests = append(parser.results.Tests, record)
		if record.Status == "fail" {
			parser.results.Failed++
		}
	case "summary":
		if record.Total == nil || record.Passed == nil || record.Failed == nil ||
			*record.Total != float64(len(parser.results.Tests)) || *record.Failed != float64(parser.results.Failed) ||
			*record.Passed != float64(len(parser.results.Tests)-parser.results.Failed) {
			return nil, fmt.Errorf("unit-test summary counts do not match test results")
		}
		parser.summarySeen = true
	default:
		return nil, fmt.Errorf("unknown unit-test event %q", record.Event)
	}
	return &record, nil
}

func (parser *resultParser) finish() error {
	if !parser.summarySeen {
		return fmt.Errorf("missing unit-test summary: checkout must provide enabled vl_unittests emitting [vl_unittests] JSON records")
	}
	if len(parser.results.Tests) == 0 {
		return fmt.Errorf("unit-test suite is empty")
	}
	return nil
}

func parseResults(logs []byte) (suiteResults, error) {
	var parser resultParser
	for _, line := range strings.Split(string(logs), "\n") {
		if _, err := parser.consume(line); err != nil {
			return parser.results, err
		}
	}
	return parser.results, parser.finish()
}
