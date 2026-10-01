/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"strings"
	"testing"
	"time"
)

// cometRecoveredPanic is CometBFT dropping a slow peer during blocksync.
const cometRecoveredPanic = `E[2026-10-01] Stopping peer for error ` +
	`err="recovered from panic: rejected msg on chID 0x40: unsolicited BlockResponse"`

func TestScanLog(t *testing.T) {
	tests := []struct {
		name      string
		logs      string
		failLine  string
		warnCount int
	}{
		{"clean", "starting node\nimported block 1\n", "", 0},
		{"go flag", "flag provided but not defined: -foo\nUsage: geth", "flag provided but not defined: -foo", 0},
		{"unknown flag", "Error: unknown flag: --bar", "Error: unknown flag: --bar", 0},
		{"unrecognized option", "error: unrecognized option '--zap'", "error: unrecognized option '--zap'", 0},
		{"invalid argument", "error: Found argument '--x'\nINVALID ARGUMENT here", "INVALID ARGUMENT here", 1},
		{"parse", "Fatal: failed to parse config.toml: line 3", "Fatal: failed to parse config.toml: line 3", 0},
		{"panic", "panic: runtime error: index out of range", "panic: runtime error: index out of range", 0},
		{"bare panic", "panic:", "panic:", 0},
		{"rust backtrace", "WARN rpc timed out\\n  22: std::panic::catch_unwind", "", 0},
		{"cometbft recovered peer panic", cometRecoveredPanic, "", 1},
		{"fatal error", "fatal error: concurrent map writes", "fatal error: concurrent map writes", 0},
		{"missing config", "stat /x: no such file or directory (config)", "stat /x: no such file or directory (config)", 0},
		{"missing other file", "open /data/peers.dat: no such file or directory", "", 0},
		{"warn only", "ERROR peer disconnected\nINFO ok\nlvl=error msg=timeout\nerrors=0", "", 2},
		{"fail beats warn", "ERROR peer\nerror parsing flags", "error parsing flags", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scanLog(tt.logs)
			if s.failLine != tt.failLine {
				t.Errorf("failLine = %q, want %q", s.failLine, tt.failLine)
			}
			if s.warnCount != tt.warnCount {
				t.Errorf("warnCount = %d, want %d", s.warnCount, tt.warnCount)
			}
		})
	}
}

func TestRunErrorSummary(t *testing.T) {
	const runcErr = "docker run: docker: Error response from daemon: failed to create task for container: " +
		"failed to create shim task: OCI runtime create failed: runc create failed: unable to start " +
		`container process: error during container init: exec: "dashd": executable file not found in $PATH`
	tests := []struct{ in, want string }{
		{runcErr, `container init: exec: "dashd": executable file not found in $PATH`},
		{"docker run: docker: Error response from daemon: Conflict. Name in use", "Conflict. Name in use"},
		{"docker run: unknown flag: --foo\nSee 'docker run --help'.", "docker run: unknown flag: --foo"},
	}
	for _, tt := range tests {
		if got := runErrorSummary(tt.in); got != tt.want {
			t.Errorf("runErrorSummary(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestVerdict(t *testing.T) {
	running := containerState{Running: true}
	tests := []struct {
		name   string
		st     containerState
		scan   logScan
		status string
		detail string
	}{
		{"pass", running, logScan{}, statusPass, "running after 1m30s"},
		{"warn", running, logScan{warnCount: 3, firstWarn: "ERROR x"}, statusWarn, "3 error line(s), first: ERROR x"},
		{"fail line while running", running, logScan{failLine: "panic: boom"}, statusFail, "panic: boom"},
		{"clean exit is a failure", containerState{ExitCode: 0}, logScan{}, statusFail, "exited with code 0"},
		{
			"exit with reason", containerState{ExitCode: 2}, logScan{failLine: "unknown flag: --x"},
			statusFail, "exited with code 2 after 1m30s; unknown flag: --x",
		},
		{"oom", containerState{ExitCode: 137, OOMKilled: true}, logScan{}, statusFail, "(OOM killed)"},
		{
			"exit names the last error line", containerState{ExitCode: 1},
			logScan{warnCount: 2, firstWarn: "Applied migrate add error field", lastWarn: "Error: wiring failed"},
			statusFail, "exited with code 1 after 1m30s; Error: wiring failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, detail := verdict(tt.st, 90*time.Second, tt.scan)
			if status != tt.status || !strings.Contains(detail, tt.detail) {
				t.Errorf("verdict = %s %q, want %s containing %q", status, detail, tt.status, tt.detail)
			}
		})
	}
}

func TestScanLogDiskFull(t *testing.T) {
	const (
		gethGuard = "ERROR[10-01|12:00:00.000] Low disk space. Gracefully shutting down Geth " +
			"to prevent database corruption. available=100.00MiB path=/data"
		enospc  = "panic: write /data/db/000001.log: no space left on device"
		badFlag = "flag provided but not defined: -x"
	)
	tests := []struct {
		name     string
		logs     string
		diskFull string
		failLine string
	}{
		{"geth guard", "INFO starting\n" + gethGuard + "\n", gethGuard, ""},
		{"enospc panic", enospc, enospc, ""},
		{"flag error kept", gethGuard + "\n" + badFlag, gethGuard, badFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scanLog(tt.logs)
			if s.diskFull != tt.diskFull || s.failLine != tt.failLine {
				t.Errorf("scan = %+v, want diskFull %q failLine %q", s, tt.diskFull, tt.failLine)
			}
			if s.warnCount != 0 {
				t.Errorf("disk full line counted as a warning: %+v", s)
			}
		})
	}
}

func TestVerdictDiskFull(t *testing.T) {
	exited := containerState{ExitCode: 0}
	status, detail := verdict(exited, 20*time.Second, logScan{diskFull: "Low disk space. Gracefully shutting down Geth"})
	if status != statusWarn || !strings.Contains(detail, "tmpfs too small") {
		t.Errorf("verdict = %s %q, want WARN tmpfs too small", status, detail)
	}
	status, _ = verdict(exited, 20*time.Second, logScan{diskFull: "Low disk space", failLine: "unknown flag: --x"})
	if status != statusFail {
		t.Errorf("a flag error next to a full disk must stay FAIL, got %s", status)
	}
}
