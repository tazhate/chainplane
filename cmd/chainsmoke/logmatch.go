/*
Copyright (c) 2026 tazhate <hate@tazhate.ru>
SPDX-License-Identifier: Apache-2.0

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	// failLogRe matches log lines that mean the operator handed the node a
	// flag or config it does not understand, or the node crashed.
	failLogRe = regexp.MustCompile(`(?i)(` + strings.Join([]string{
		`unknown flag`,
		`flag provided but not defined`,
		`unrecognized (option|argument)`,
		`invalid (argument|option|flag)`,
		`error parsing`,
		`failed to parse`,
		`panic:`,
		`fatal error:`,
		`no such file or directory.*config`,
	}, "|") + `)`)
	// warnLogRe matches the remaining error noise worth a look.
	warnLogRe = regexp.MustCompile(`(?i)\b(fatal|error)\b`)
)

// logScan summarises a container log against failLogRe and warnLogRe.
type logScan struct {
	failLine  string // first failLogRe line
	warnCount int    // warnLogRe lines that are not failLogRe lines
	firstWarn string
}

func scanLog(logs string) logScan {
	var s logScan
	for line := range strings.Lines(logs) {
		line = strings.TrimSpace(line)
		switch {
		case failLogRe.MatchString(line):
			if s.failLine == "" {
				s.failLine = line
			}
		case warnLogRe.MatchString(line):
			s.warnCount++
			if s.firstWarn == "" {
				s.firstWarn = line
			}
		}
	}
	return s
}

// containerState is the subset of `docker inspect` .State used for verdicts.
type containerState struct {
	Running   bool   `json:"Running"`
	ExitCode  int    `json:"ExitCode"`
	OOMKilled bool   `json:"OOMKilled"`
	Error     string `json:"Error"`
}

// verdict turns the container state after the run window and its log scan
// into a status and detail. Any exit before the window ends is a FAIL, a
// clean exit included: a node is supposed to keep running.
func verdict(st containerState, ranFor time.Duration, s logScan) (status, detail string) {
	if !st.Running {
		detail = fmt.Sprintf("exited with code %d after %s", st.ExitCode, ranFor.Round(time.Second))
		if st.OOMKilled {
			detail += " (OOM killed)"
		}
		if st.Error != "" {
			detail += ": " + st.Error
		}
		if s.failLine != "" {
			detail += "; " + s.failLine
		} else if s.firstWarn != "" {
			detail += "; " + s.firstWarn
		}
		return statusFail, detail
	}
	if s.failLine != "" {
		return statusFail, s.failLine
	}
	if s.warnCount > 0 {
		return statusWarn, fmt.Sprintf("%d error line(s), first: %s", s.warnCount, s.firstWarn)
	}
	return statusPass, fmt.Sprintf("running after %s", ranFor.Round(time.Second))
}
