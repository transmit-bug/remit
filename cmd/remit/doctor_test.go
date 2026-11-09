package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// doctorLibrary is a library with nothing wrong with it, as the command line
// finds one.
func doctorLibrary(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "library")
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := lib.SaveNote(core.Note{Title: "Memory", Blocks: []core.Block{
		{Text: "spacing :: beats cramming"},
	}}); err != nil {
		t.Fatalf("note: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return dir
}

// A doctor command answers for the state of the library: 0 when there is
// nothing to act on, 1 when a finding is there whoever has to fix it, 2 when the
// check itself could not run.
func TestDoctorExitCodes(t *testing.T) {
	whole := doctorLibrary(t)
	damaged := doctorLibrary(t)
	if err := os.Remove(filepath.Join(damaged, "remit.json")); err != nil {
		t.Fatalf("remove remit.json: %v", err)
	}
	absent := filepath.Join(t.TempDir(), "no-library")

	cases := []struct {
		name string
		dir  string
		want int
	}{
		{"a whole library", whole, doctorOK},
		{"a library with a finding", damaged, doctorFindings},
		{"no library to check", absent, doctorError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code, handled := runDoctor(tc.dir, true, false, false, &stdout, &stderr)
			if !handled {
				t.Fatal("-doctor did nothing")
			}
			if code != tc.want {
				t.Errorf("exit %d, want %d (stdout %q, stderr %q)", code, tc.want, stdout.String(), stderr.String())
			}
			if tc.want == doctorError && stdout.Len() != 0 {
				t.Errorf("a check that could not run still printed a report: %q", stdout.String())
			}
			if tc.want == doctorOK && !strings.Contains(stdout.String(), "no findings") {
				t.Errorf("a whole library does not say so: %q", stdout.String())
			}
			if tc.want == doctorFindings {
				for _, want := range []string{"error", "remit.json", "config.missing"} {
					if !strings.Contains(stdout.String(), want) {
						t.Errorf("the report does not name %q: %q", want, stdout.String())
					}
				}
			}
		})
	}
}

// -doctor-json prints the report in the shape a script reads, and the exit code
// still answers for the library.
func TestDoctorJSONShape(t *testing.T) {
	dir := doctorLibrary(t)
	if err := os.Remove(filepath.Join(dir, "remit.json")); err != nil {
		t.Fatalf("remove remit.json: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code, handled := runDoctor(dir, false, true, false, &stdout, &stderr)
	if !handled {
		t.Fatal("-doctor-json did nothing")
	}
	if code != doctorFindings {
		t.Fatalf("exit %d, want %d (stderr %q)", code, doctorFindings, stderr.String())
	}

	var report struct {
		Findings []map[string]any `json:"findings"`
		Notes    *int             `json:"notes"`
		Cards    *int             `json:"cards"`
		Reviews  *int             `json:"reviews"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
	if report.Notes == nil || report.Cards == nil || report.Reviews == nil {
		t.Errorf("the report lost a count: %s", stdout.String())
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %v, want the one", report.Findings)
	}
	finding := report.Findings[0]
	for _, key := range []string{"code", "severity", "path", "detail", "fixable"} {
		if _, ok := finding[key]; !ok {
			t.Errorf("the finding has no %q: %v", key, finding)
		}
	}
	if len(finding) != 5 {
		t.Errorf("the finding carries fields no reader knows: %v", finding)
	}
	if finding["code"] != "config.missing" || finding["path"] != "remit.json" || finding["fixable"] != false {
		t.Errorf("finding = %v, want the missing remit.json", finding)
	}
}

// -doctor-fix is -doctor that repairs: a deleted index comes back, and the code
// answers for the library after the repair.
func TestDoctorFixImpliesTheCheckAndRebuildsTheIndex(t *testing.T) {
	dir := doctorLibrary(t)
	if err := os.RemoveAll(filepath.Join(dir, ".remit")); err != nil {
		t.Fatalf("remove index: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code, handled := runDoctor(dir, false, false, true, &stdout, &stderr)
	if !handled {
		t.Fatal("-doctor-fix did nothing")
	}
	if code != doctorOK {
		t.Fatalf("exit %d, want %d (stdout %q, stderr %q)", code, doctorOK, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".remit", "index.sqlite")); err != nil {
		t.Errorf("the index was not built again: %v", err)
	}
	if !strings.Contains(stdout.String(), "no findings") {
		t.Errorf("after the repair the report is %q, want none", stdout.String())
	}
}

func TestNoDoctorFlagDoesNothing(t *testing.T) {
	if _, handled := runDoctor(t.TempDir(), false, false, false, io.Discard, io.Discard); handled {
		t.Error("runDoctor acted with no doctor flag given")
	}
}
