package doctor

import "strings"

type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

type Check struct {
	Name     string
	Status   Status
	Summary  string
	NextStep string
}

type Report struct {
	ProjectSlug string
	RepoRoot    string
	Checks      []Check
}

func (report *Report) Add(check Check) {
	report.Checks = append(report.Checks, check)
}

func (report Report) Counts() (pass int, warn int, fail int) {
	for _, check := range report.Checks {
		switch check.Status {
		case StatusPass:
			pass++
		case StatusWarn:
			warn++
		case StatusFail:
			fail++
		}
	}
	return pass, warn, fail
}

func (report Report) HasFailures() bool {
	return report.FailureCount() > 0
}

func (report Report) FailureCount() int {
	_, _, fail := report.Counts()
	return fail
}

func NormalizeStatus(value string) Status {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(StatusPass):
		return StatusPass
	case string(StatusFail):
		return StatusFail
	default:
		return StatusWarn
	}
}
