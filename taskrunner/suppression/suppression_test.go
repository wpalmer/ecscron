package suppression

import (
	"errors"
	"testing"

	"github.com/wpalmer/ecscron/taskrunner"
)

func TestTaskRunner(t *testing.T) {
	t.Run("Should Pass-Through on non-suppressed RunTask", func(t *testing.T) {
		var passedTask string
		taskResult := &taskrunner.TaskStatus{
			Ran:      true,
			Error:    nil,
			Warnings: []error{},
			Output:   "testOutput",
		}

		runner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			passedTask = task
			return taskResult, nil
		})
		suppressor := NewSuppressionTaskRunner(runner)
		result, _ := suppressor.RunTask("test")

		if passedTask != "test" {
			t.Fatalf("The function was not passed the expected task name")
		}

		if result != taskResult {
			t.Fatalf("The function output was not passed through to the test when" +
				"calling .RunTask(...)")
		}
	})

	t.Run("Should not pass-through on Suppressed RunTask", func(t *testing.T) {
		var didRun bool
		taskResult := &taskrunner.TaskStatus{
			Ran:      true,
			Error:    nil,
			Warnings: []error{},
			Output:   "testOutput",
		}

		runner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			didRun = true
			return taskResult, nil
		})
		suppressor := NewSuppressionTaskRunner(runner)
		reason := errors.New("testReason")
		suppressor.Suppress("test", reason)
		result, _ := suppressor.RunTask("test")

		if didRun {
			t.Fatalf("Suppressed task was passed to the inner runner")
		}

		if result.Ran {
			t.Fatalf("Suppressed task Status claims to have run")
		}

		if result.Error != nil {
			t.Fatalf("Suppressed task Status contains an error: %s", result.Error)
		}

		if len(result.Warnings) != 1 || result.Warnings[0] != reason {
			t.Fatalf("Suppressed task Status does not have the reason as the warning")
		}
	})

	t.Run("Should have empty Warnings on nil Reason", func(t *testing.T) {
		var didRun bool
		taskResult := &taskrunner.TaskStatus{
			Ran:      true,
			Error:    nil,
			Warnings: []error{},
			Output:   "testOutput",
		}

		runner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			didRun = true
			return taskResult, nil
		})
		suppressor := NewSuppressionTaskRunner(runner)
		suppressor.Suppress("test", nil)
		result, _ := suppressor.RunTask("test")

		if didRun {
			t.Fatalf("Suppressed task was passed to the inner runner")
		}

		if result.Ran {
			t.Fatalf("Suppressed task Status claims to have run")
		}

		if result.Error != nil {
			t.Fatalf("Suppressed task Status contains an error: %s", result.Error)
		}

		if len(result.Warnings) != 0 {
			t.Fatalf("nil reason did not result in empty warnings list")
		}
	})
}
