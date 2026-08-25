package checks

// Status represents the outcome of a diagnostic check.
type Status int

const (
	OK Status = iota
	Warn
	Fail
	Skip
)

func (s Status) String() string {
	switch s {
	case OK:
		return "OK"
	case Warn:
		return "WARN"
	case Fail:
		return "FAIL"
	case Skip:
		return "SKIP"
	default:
		return "UNKNOWN"
	}
}

// Result holds the outcome of a single check.
type Result struct {
	Name    string
	Status  Status
	Message string
	Details []string // optional sub-lines rendered beneath the main line
}

// Check is a single diagnostic that can be run.
type Check interface {
	Name() string
	Run() Result
}
