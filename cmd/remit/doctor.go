package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/transmit-bug/remit/internal/core/doctor"
)

// The exit codes a doctor command answers: 0 the library is whole, 1 it has
// findings, 2 the check itself could not run. -doctor-json answers the same
// codes, so a script reads the report and the status both.
const (
	doctorOK       = 0
	doctorFindings = 1
	doctorError    = 2
)

// runDoctor checks the library's integrity without opening a window, and
// reports whether a doctor flag asked for it. -doctor-fix implies -doctor: it
// repairs the derived index and stale temporary files, then reports what is
// left, so the exit code answers for the library as it stands afterwards.
func runDoctor(dir string, check, asJSON, fix bool, stdout, stderr io.Writer) (int, bool) {
	if !check && !asJSON && !fix {
		return doctorOK, false
	}
	var (
		report doctor.Report
		err    error
	)
	if fix {
		report, err = doctor.Fix(dir)
	} else {
		report, err = doctor.Check(dir)
	}
	if asJSON {
		if writeErr := writeDoctorJSON(stdout, report); writeErr != nil {
			fmt.Fprintf(stderr, "doctor: %v\n", writeErr)
			return doctorError, true
		}
	} else if err == nil || !emptyReport(report) {
		// A check that could not run found nothing to print; a repair that could
		// not finish still names what it saw.
		writeDoctorReport(stdout, dir, report)
	}
	if err != nil {
		// A repair that could not finish still reports what it found: the
		// reason goes beside the findings, not instead of them.
		fmt.Fprintln(stderr, err)
		return doctorError, true
	}
	if !report.Clean() {
		return doctorFindings, true
	}
	return doctorOK, true
}

// writeDoctorReport prints the check as a person reads it: one line per
// finding, then what the library holds and how much of it can be repaired.
func writeDoctorReport(w io.Writer, dir string, report doctor.Report) {
	fmt.Fprintf(w, "%s: %s\n", dir, doctorSummary(report))
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "  %-7s %s [%s] %s", finding.Severity, finding.Path, finding.Code, finding.Detail)
		if finding.Fixable {
			fmt.Fprint(w, " (fixable)")
		}
		fmt.Fprintln(w)
	}
	if fixable := countFixable(report.Findings); fixable > 0 {
		fmt.Fprintf(w, "%s repairable with -doctor-fix\n", count(fixable, "finding", "findings"))
	}
}

// writeDoctorJSON prints the report in the shape a script reads. The findings
// are always an array, so a consumer never has to tell null from empty.
func writeDoctorJSON(w io.Writer, report doctor.Report) error {
	if report.Findings == nil {
		report.Findings = []doctor.Finding{}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func doctorSummary(report doctor.Report) string {
	held := fmt.Sprintf("%s, %s, %s",
		count(report.Notes, "note", "notes"),
		count(report.Cards, "card", "cards"),
		count(report.Reviews, "log event", "log events"))
	if report.Clean() {
		return "no findings (" + held + ")"
	}
	severities := map[string]int{}
	for _, finding := range report.Findings {
		severities[finding.Severity]++
	}
	return fmt.Sprintf("%s (%s, %s, %s; %s)",
		count(len(report.Findings), "finding", "findings"),
		count(severities[doctor.SeverityError], "error", "errors"),
		count(severities[doctor.SeverityWarning], "warning", "warnings"),
		count(severities[doctor.SeverityInfo], "info", "info"),
		held)
}

// count names a quantity with the number that agrees with it.
func count(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// emptyReport says the check left nothing to read: no finding, no count.
func emptyReport(report doctor.Report) bool {
	return len(report.Findings) == 0 && report.Notes == 0 && report.Cards == 0 && report.Reviews == 0
}

func countFixable(findings []doctor.Finding) int {
	fixable := 0
	for _, finding := range findings {
		if finding.Fixable {
			fixable++
		}
	}
	return fixable
}
