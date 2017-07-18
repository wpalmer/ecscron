package taskrunner

import (
	"testing"
)

func TestTaskRunnerFunc(t *testing.T) {
	t.Run("Should Pass-Through on RunTask", func(t *testing.T) {
		var passedTask string
		taskResult := &TaskStatus{
			Ran:      true,
			Error:    nil,
			Warnings: []error{},
			Output:   "testOutput",
		}
		result, _ := TaskRunnerFunc(func(task string) (*TaskStatus, error) {
			passedTask = task
			return taskResult, nil
		}).RunTask("test")

		if passedTask != "test" {
			t.Fatalf("The function was not passed the expected task name")
		}

		if result != taskResult {
			t.Fatalf("The function output was not passed through to the test when" +
				"calling .RunTask(...)")
		}
	})
}
