package suppression

import (
	"github.com/wpalmer/ecscron/taskrunner"
)

type SuppressionTaskRunner struct {
	runner taskrunner.TaskRunner
	tasks  map[string]error
}

func NewSuppressionTaskRunner(runner taskrunner.TaskRunner) *SuppressionTaskRunner {
	return &SuppressionTaskRunner{
		runner: runner,
		tasks:  make(map[string]error),
	}
}

func (r SuppressionTaskRunner) Suppress(task string, reason error) {
	r.tasks[task] = reason
}

func (r SuppressionTaskRunner) RunTask(task string) (*taskrunner.TaskStatus, error) {
	var reason error
	var ok bool
	if reason, ok = r.tasks[task]; !ok {
		return r.runner.RunTask(task)
	}

	warnings := []error{}
	if reason != nil {
		warnings = []error{reason}
	}

	return &taskrunner.TaskStatus{
		Ran:      false,
		Error:    nil,
		Warnings: warnings,
		Output:   nil,
	}, nil
}
