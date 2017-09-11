package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/wpalmer/ecscron/schedule"
	"github.com/wpalmer/ecscron/taskrunner"
)

func TestRetrySchedule(t *testing.T) {
	t.Run("Next should initially pass to inner schedule", func(t *testing.T) {
		innerSchedule := schedule.NewBasicSchedule()

		testAfter := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
		testNext := testAfter.Add(time.Minute)
		innerSchedule.Set("test", schedule.NextTime(testNext))

		schedule := NewRetrySchedule(schedule.Schedule(innerSchedule), 1)
		result := schedule.Next(testAfter)

		if !result.Equal(testNext) {
			t.Fatalf("When no failure, Next did not pass-through to innner schedule")
		}
	})

	t.Run("Tick should initially pass to inner schedule", func(t *testing.T) {
		innerSchedule := schedule.NewBasicSchedule()
		taskResult := &taskrunner.TaskStatus{
			Ran:      true,
			Error:    nil,
			Warnings: []error{},
			Output:   "testOutputInitial",
		}

		var passedTask string
		runner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			passedTask = task
			return taskResult, nil
		})

		testAfter := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
		innerSchedule.Set("test", schedule.NextTime(testAfter))

		outerSchedule := NewRetrySchedule(innerSchedule, 1)
		results, err := outerSchedule.Tick(runner, testAfter)

		if err != nil {
			t.Fatalf("When no failure, unexpected error was returned by Tick")
		}

		if passedTask != "test" {
			t.Fatalf("Passed schedule/taskrunner did not receive expected task")
		}

		if _, ok := results["test"]; !ok {
			t.Fatalf("Task name not defined in results after tick")
		}

		if results["test"] != taskResult {
			t.Fatalf("Task Result was not passed back in the results map")
		}
	})

	t.Run("Failure should result in retry", func(t *testing.T) {
		innerSchedule := schedule.NewBasicSchedule()

		failRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			return &taskrunner.TaskStatus{
				Ran:      false,
				Error:    nil,
				Warnings: []error{errors.New("intential failure to trigger retry")},
				Output:   "testOutputIntentionalFailure",
			}, nil
		})

		testAfter := time.Date(2006, 1, 2, 15, 4, 30, 0, time.UTC)
		testNext := testAfter.Add((time.Second * 30) + (time.Minute * 2))
		innerSchedule.Set("test", schedule.NextTime(testNext))

		outerSchedule := NewRetrySchedule(innerSchedule, -1)
		_, _ = outerSchedule.Tick(failRunner, testNext)

		result := outerSchedule.Next(testAfter)
		if !result.Equal(testAfter.Add(time.Second * 30)) {
			t.Fatalf("Next after failed tick did not round to next whole-minute")
		}

		result = outerSchedule.Next(testAfter.Add(time.Second * 30))
		if !result.Equal(testAfter.Add(time.Second*30 + time.Minute)) {
			t.Fatalf("Next after a whole-minute after a failed tick was not identity")
		}

		didRun := false
		trackRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			didRun = true
			return &taskrunner.TaskStatus{
				Ran:      true,
				Error:    nil,
				Warnings: []error{},
				Output:   "testOutputSuccess",
			}, nil
		})
		results, _ := outerSchedule.Tick(trackRunner, testAfter)
		if !didRun {
			t.Fatalf("Ticking after a failure did not result in a retry")
		}

		for _, oneResult := range results {
			if _, ok := oneResult.Info.(*RetryInfo); !ok {
				t.Fatalf("Result of a Retry did not include RetryInfo")
			}
		}

		didRun = false
		_, _ = outerSchedule.Tick(trackRunner, testAfter)
		if didRun {
			t.Fatalf("Ticking after a failure+success still resulted in a retry")
		}
	})

	t.Run("Already-running should not result in retry", func(t *testing.T) {
		innerSchedule := schedule.NewBasicSchedule()

		failRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			return &taskrunner.TaskStatus{
				Ran:      false,
				Running:  true,
				Error:    nil,
				Warnings: []error{errors.New("intential already-running failure")},
				Output:   "testOutputIntentionalAlreadyRunningFailure",
			}, nil
		})

		testAfter := time.Date(2006, 1, 2, 15, 4, 30, 0, time.UTC)
		testNext := testAfter.Add((time.Second * 30) + (time.Minute * 2))
		innerSchedule.Set("test", schedule.NextTime(testNext))

		outerSchedule := NewRetrySchedule(innerSchedule, -1)
		_, _ = outerSchedule.Tick(failRunner, testNext)

		result := outerSchedule.Next(testAfter)
		if result.Equal(testAfter.Add(time.Second * 30)) {
			t.Fatalf("Next after already-running tick did add retry to schedule")
		}
	})

	t.Run("Failure should retry only maxRetries times", func(t *testing.T) {
		innerSchedule := schedule.NewBasicSchedule()

		runs := 0
		failRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			runs += 1
			return &taskrunner.TaskStatus{
				Ran:      false,
				Error:    nil,
				Warnings: []error{errors.New("intential failure to trigger retry")},
				Output:   "testOutputIntentionalFailure",
			}, nil
		})

		testAfter := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
		testNext := testAfter.Add((time.Second * 30))
		innerSchedule.Set("test", schedule.NextTime(testNext))

		outerSchedule := NewRetrySchedule(innerSchedule, 2)
		_, _ = outerSchedule.Tick(failRunner, testNext)

		for i := 0; i < 3; i++ {
			_, _ = outerSchedule.Tick(failRunner, testAfter)
		}

		if runs != 2 {
			t.Fatalf("Ticking repeatedly on an always-failing task did not retry " +
				"exactly maxRetries times")
		}
	})

	t.Run("Actual errors should not pass-through", func(t *testing.T) {
		innerSchedule := schedule.NewBasicSchedule()

		failRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			return &taskrunner.TaskStatus{
				Ran:      false,
				Error:    nil,
				Warnings: []error{errors.New("intential failure to trigger retry")},
				Output:   "testOutputIntentionalFailure",
			}, nil
		})

		testAfter := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
		testNext := testAfter.Add((time.Second * 30))
		innerSchedule.Set("test", schedule.NextTime(testNext))

		outerSchedule := NewRetrySchedule(innerSchedule, -1)
		_, _ = outerSchedule.Tick(failRunner, testNext)

		errorRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			return &taskrunner.TaskStatus{
				Ran:      false,
				Error:    nil,
				Warnings: []error{errors.New("intential failure to trigger retry")},
				Output:   "testOutputIntentionalFailure",
			}, errors.New("intentionalError")
		})

		_, err := outerSchedule.Tick(errorRunner, testAfter)
		if err.Error() != errors.New("intentionalError").Error() {
			t.Fatalf("error in retry taskrunner did not pass-through")
		}
	})
}
