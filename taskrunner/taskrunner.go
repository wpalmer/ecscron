package taskrunner

type TaskRunner interface {
	RunTask(task string) (*TaskStatus, error)
}

type TaskStatus struct {
	Ran bool

	// Permanent or undefined / unknown error
	Error error

	// Possibly temporary, known errors (eg: failure to place task, task is still running)
	// These errors should probably result in the scheduler arranging for the task to try again
	Warnings []error

	Info   interface{}
	Output interface{}
}

type TaskRunnerFunc func(task string) (*TaskStatus, error)

func (r TaskRunnerFunc) RunTask(task string) (*TaskStatus, error) {
	return r(task)
}
