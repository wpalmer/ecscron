package tweak

import (
	"testing"

	"github.com/wpalmer/ecscron/taskrunner"
)

func TestTweakTaskRunner(t *testing.T) {
	t.Run("RunTask Should tweaked name for inner runner", func(t *testing.T) {
		var passedTask string
		runner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			passedTask = task
			return &taskrunner.TaskStatus{
				Ran:      true,
				Error:    nil,
				Warnings: []error{},
				Output:   "testOutput",
			}, nil
		})

		tweak := NewTweakTaskRunner(runner, func(task string) string {
			return "Tweaked"
		})
		_, _ = tweak.RunTask("test")

		if passedTask != "Tweaked" {
			t.Fatalf("The tweaked task was not passed to the inner runner")
		}
	})
}
