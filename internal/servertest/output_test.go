// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package servertest

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPresenterStreamsResultsWithoutDuplicatingEngineLogs(t *testing.T) {
	engineLog := "2026-09-14 14:12:59: ACTION[Main]: World seed = 123\n"
	failure := strings.Replace(passingTest, `"status":"pass"`, `"status":"fail","traceback":"...ib/minetest/games/voxelibre/mods/CORE/mcl_util/tests.lua:8: expected \"IVAAA\", got \"IV\"\nstack traceback:\n\t/var/lib/minetest/games/voxelibre/mods/CORE/vl_unittests/assertions.lua:15: in function 'fail'"`, 1)
	warning := "2026-09-14 14:13:00: WARNING[Main]: decoration registered late\n"
	summary := strings.NewReplacer(`"passed":1`, `"passed":0`, `"failed":0`, `"failed":1`).Replace(passingSummary)
	logs := engineLog + "2026-09-14 14:12:59: ACTION[Main]: " + failure + warning + strings.TrimSuffix(summary, "\n")
	var output bytes.Buffer
	presenter := &logPresenter{output: &output}
	// Each byte can be a poll boundary, even within UTF-8, JSON or a log prefix.
	for size := 1; size <= len(logs); size++ {
		if err := presenter.snapshot([]byte(logs[:size])); err != nil {
			t.Fatal(err)
		}
		if err := presenter.snapshot([]byte(logs[:size])); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(output.String(), "Tests:") {
		t.Fatal("unterminated summary was emitted early")
	}
	if err := presenter.flush(); err != nil {
		t.Fatal(err)
	}
	results, err := presenter.results()
	if err != nil || results.Failed != 1 {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	want := engineLog + "  FAIL  example / test (1.00 ms)\n" +
		"        mods/CORE/mcl_util/tests.lua:8: expected \"IVAAA\", got \"IV\"\n" +
		"        stack traceback:\n" +
		"          mods/CORE/vl_unittests/assertions.lua:15: in function 'fail'\n" +
		warning + "  Tests: 0 passed, 1 failed\n"
	if output.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", &output, want)
	}
}

func TestPresenterPreservesAllNonTestOutput(t *testing.T) {
	logs := "timestamp: [Main]: startup\r\n _   _\n| |_| |\n\nERROR: Lua crashed\n  traceback line\nshutdown without newline"
	var output bytes.Buffer
	presenter := &logPresenter{output: &output}
	if err := presenter.snapshot([]byte(logs)); err != nil {
		t.Fatal(err)
	}
	if err := presenter.flush(); err != nil {
		t.Fatal(err)
	}
	if output.String() != logs {
		t.Fatalf("engine output changed: %q", output.String())
	}
}

func TestPresenterKeepsMalformedRecordsAndFollowingLogs(t *testing.T) {
	logs := "[vl_unittests] {bad JSON}\nERROR: crash details\n"
	var output bytes.Buffer
	presenter := &logPresenter{output: &output}
	if err := presenter.snapshot([]byte(logs)); err != nil {
		t.Fatal(err)
	}
	_, err := presenter.results()
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("err=%v", err)
	}
	if output.String() != logs {
		t.Fatalf("lost diagnostic context: %q", output.String())
	}
}

func TestPresenterDoesNotReplayChangedLogHistory(t *testing.T) {
	var output bytes.Buffer
	presenter := &logPresenter{output: &output}
	if err := presenter.snapshot([]byte(passingLogs)); err != nil {
		t.Fatal(err)
	}
	before := output.String()
	if err := presenter.snapshot([]byte("changed history\n" + passingLogs)); err == nil {
		t.Fatal("expected changed-history error")
	}
	if output.String() != before {
		t.Fatal("replayed previous results")
	}
}

func TestPresenterRetainsLongTracebacks(t *testing.T) {
	trace := strings.Repeat("trace line\\n", 10000)
	logs := strings.Replace(passingTest, `"status":"pass"`, `"status":"fail","traceback":"`+trace+`"`, 1)
	var output bytes.Buffer
	presenter := &logPresenter{output: &output}
	if err := presenter.snapshot([]byte(logs)); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "trace line\n") != 10000 {
		t.Fatal("traceback truncated")
	}
}

func TestFormatDuration(t *testing.T) {
	for _, tc := range []struct {
		seconds float64
		want    string
	}{
		{0, "0 µs"}, {3.3000000000000003e-5, "33 µs"}, {0.012345, "12.35 ms"}, {2.5, "2.50 s"},
	} {
		if got := formatDuration(tc.seconds); got != tc.want {
			t.Fatalf("%g: got %q, want %q", tc.seconds, got, tc.want)
		}
	}
}

func TestReadableTracebackDoesNotGuessUnknownPaths(t *testing.T) {
	trace := "...unknown/location.lua:8: failed\n\t[C]: in function 'error'"
	want := "...unknown/location.lua:8: failed\n  [C]: in function 'error'"
	if got := readableTraceback(trace); got != want {
		t.Fatalf("got %q", got)
	}
}

type brokenOutput struct{}

func (brokenOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPresenterReportsOutputErrors(t *testing.T) {
	for _, log := range []string{passingLogs, "ordinary engine log\n"} {
		presenter := &logPresenter{output: brokenOutput{}}
		if err := presenter.snapshot([]byte(log)); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("err=%v", err)
		}
	}
}
