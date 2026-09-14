// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"strings"
	"testing"
)

const passingTest = `[vl_unittests] {"event":"test","mod":"example","name":"test","status":"pass","duration":0.001}` + "\n"
const passingSummary = `[vl_unittests] {"event":"summary","total":1,"passed":1,"failed":0}` + "\n"
const passingLogs = passingTest + passingSummary

func TestParseResults(t *testing.T) {
	for _, tc := range []struct {
		name string
		logs string
		want string
	}{
		{"pass", passingLogs, ""},
		{"older Luanti decimal counts", passingTest + strings.NewReplacer(`:1`, `:1.0`, `:0`, `:0.0`).Replace(passingSummary), ""},
		{"fractional counts", passingTest + strings.Replace(passingSummary, `"total":1`, `"total":1.5`, 1), "counts do not match"},
		{"luanti prefixes", "2026-09-13 12:00:00: ACTION[Main]: " + passingTest + "2026-09-13 12:00:00: ACTION[Server]: " + passingSummary, ""},
		{"no trailing newline", strings.TrimSuffix(passingLogs, "\n"), ""},
		{"ordinary logs", "some output\n" + passingLogs + "shutdown\n", ""},
		{"embedded marker ignored", "a traceback mentions " + passingSummary + passingLogs, ""},
		{"readiness is insufficient", "Server for gameid=\"voxelibre\" listening on [::]:30000.\n", "missing unit-test summary"},
		{"missing summary", passingTest, "missing unit-test summary"},
		{"bad json", passingLogs + resultPrefix + "{", "malformed"},
		{"truncated summary", passingTest + resultPrefix + `{"event":"summary"`, "malformed"},
		{"duplicate summary", passingLogs + passingSummary, "after final summary"},
		{"duplicate test", passingTest + passingLogs, "duplicate unit-test"},
		{"test after summary", passingLogs + passingTest, "after final summary"},
		{"missing results", passingSummary, "counts do not match"},
		{"wrong counts", strings.Replace(passingLogs, `"passed":1`, `"passed":2`, 1), "counts do not match"},
		{"missing count", strings.Replace(passingLogs, `,"failed":0`, "", 1), "counts do not match"},
		{"empty suite", resultPrefix + `{"event":"summary","total":0,"passed":0,"failed":0}`, "empty"},
		{"null", resultPrefix + `null`, "unknown unit-test event"},
		{"unknown event", resultPrefix + `{"event":"something"}`, "unknown unit-test event"},
		{"invalid status", strings.Replace(passingLogs, `"pass"`, `"skip"`, 1), "invalid unit-test result"},
		{"missing duration", strings.Replace(passingLogs, `,"duration":0.001`, "", 1), "invalid unit-test result"},
		{"negative duration", strings.Replace(passingLogs, "0.001", "-1", 1), "invalid unit-test result"},
		{"failure without traceback", strings.Replace(passingLogs, `"pass"`, `"fail"`, 1), "invalid unit-test result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseResults([]byte(tc.logs))
			if tc.want == "" && err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestParseFailureTraceback(t *testing.T) {
	logs := strings.Replace(passingTest, `"status":"pass"`, `"status":"fail","traceback":"assertion failed\nstack traceback:\n\ttests.lua:42"`, 1) +
		strings.NewReplacer(`"passed":1`, `"passed":0`, `"failed":0`, `"failed":1`).Replace(passingSummary)
	results, err := parseResults([]byte(logs))
	if err != nil {
		t.Fatal(err)
	}
	if results.Failed != 1 || !strings.Contains(results.Tests[0].Traceback, "\n\ttests.lua:42") {
		t.Fatalf("results = %#v", results)
	}
}
