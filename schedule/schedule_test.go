package schedule

import (
	"errors"
	"testing"
	"time"

	"github.com/wpalmer/ecscron/taskrunner"
)

func TestNextTime(t *testing.T) {
	t.Run("Should Return Time if Before", func(t *testing.T) {
		testIn := time.Date(2000, 1, 2, 3, 4, 5, 6, time.UTC)
		result := NextTime(testIn).Next(testIn.Add(time.Duration(-1)))

		if !result.Equal(testIn) {
			t.Fatalf("The input time was not returned when after the after time")
		}
	})

	t.Run("Should Return Zero if After", func(t *testing.T) {
		testIn := time.Date(2000, 1, 2, 3, 4, 5, 6, time.UTC)
		result := NextTime(testIn).Next(testIn.Add(time.Nanosecond))

		if !result.IsZero() {
			t.Fatalf("Zero time was not returned when time before the after time")
		}
	})
}

func TestNextFunc(t *testing.T) {
	t.Run("Should Pass-Through on Next", func(t *testing.T) {
		testIn := time.Date(2000, 1, 2, 3, 4, 5, 6, time.UTC)
		testOut := time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC)
		result := NextFunc(func(after time.Time) time.Time {
			if !after.Equal(testIn) {
				t.Fatalf("The test input was not passed through to the function when" +
					"calling .Next(...)")
			}

			return testOut
		}).Next(testIn)

		if !result.Equal(testOut) {
			t.Fatalf("The function output was not passed through to the test when" +
				"calling .Next(...)")
		}
	})
}

func TestNextList(t *testing.T) {
	list := NextList{}

	t.Run("First Add should return on Next", func(t *testing.T) {
		testAdd := time.Date(2001, 1, 0, 0, 0, 0, 0, time.UTC)
		testAfter := time.Date(2000, 1, 0, 0, 0, 0, 0, time.UTC)

		list.Add(NextTime(testAdd))
		result := list.Next(testAfter)

		if !result.Equal(testAdd) {
			t.Fatalf("Next did not return the first Nexter result when it was " +
				"the only Nexter in the list")
		}
	})

	t.Run("Multiple Adds should return the Earliest on Next", func(t *testing.T) {
		testAdd1 := time.Date(1999, 2, 0, 0, 0, 0, 0, time.UTC)
		testAdd2 := time.Date(2001, 2, 0, 0, 0, 0, 2, time.UTC)
		testAdd3 := time.Date(2001, 2, 0, 0, 0, 0, 1, time.UTC)
		testAdd4 := time.Date(2001, 2, 0, 0, 0, 0, 3, time.UTC)
		testAfter := time.Date(2001, 2, 0, 0, 0, 0, 0, time.UTC)

		list.Add(NextTime(testAdd1))
		list.Add(NextTime(testAdd2))
		list.Add(NextTime(testAdd3))
		list.Add(NextTime(testAdd4))
		result := list.Next(testAfter)

		if !result.Equal(testAdd3) {
			t.Fatalf("Next did not return the earliest Nexter result when multiple " +
				"were added to the list")
		}

		result = list.Next(testAdd3)

		if !result.Equal(testAdd2) {
			t.Fatalf("Next did not return the 2nd-earliest Nexter result when " +
				"using the earliest as the after date")
		}
	})

	t.Run("Clearing should prevent previous adds after matching", func(t *testing.T) {
		testAdd1 := time.Date(1999, 2, 0, 0, 0, 0, 1, time.UTC)
		testAdd2 := time.Date(2001, 2, 0, 0, 0, 0, 2, time.UTC)
		testAdd3 := time.Date(2001, 2, 0, 0, 0, 0, 3, time.UTC)
		testAdd4 := time.Date(2001, 2, 0, 0, 0, 0, 4, time.UTC)
		testAfter := time.Date(2001, 2, 0, 0, 0, 0, 2, time.UTC)

		list.Add(NextTime(testAdd1))
		list.Add(NextTime(testAdd2))
		list.Clear()
		list.Add(NextTime(testAdd3))
		list.Add(NextTime(testAdd4))
		result := list.Next(testAfter)

		if !result.Equal(testAdd3) {
			t.Fatalf("Next did not return the earliest Nexter result when multiple " +
				"were added to the list after Clear")
		}
	})

	t.Run("Earliest possible should prevent further checks", func(t *testing.T) {
		shortCircuit := true
		testAdd1 := time.Date(2001, 2, 0, 0, 0, 0, 0, time.UTC)
		list.Add(NextTime(testAdd1))

		list.Add(NextFunc(func(after time.Time) time.Time {
			shortCircuit = false
			return time.Date(2001, 2, 0, 0, 0, 0, 1, time.UTC)
		}))

		testAfter := testAdd1.Add(time.Duration(-1))
		result := list.Next(testAfter)

		if !result.Equal(testAdd1) {
			t.Fatalf("Next did not return an earliest possible: %v", result)
		}

		if !shortCircuit {
			t.Fatalf("Next did not stop processing after earliest possible")
		}
	})
}

func TestBasicSchedule(t *testing.T) {
	t.Run("Next should return tasks which have been Set", func(t *testing.T) {
		schedule := NewBasicSchedule()

		testAdd := time.Date(2006, 1, 2, 15, 4, 5, 999, time.UTC)
		schedule.Set("test", NextTime(testAdd))

		testAfter := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
		result := schedule.Next(testAfter)

		if !result.Equal(testAdd) {
			t.Fatalf("Next did not return the added Nexter when it was " +
				"the only Nexter in the list")
		}
	})

	t.Run("Next should return Earliest Set Nexter", func(t *testing.T) {
		schedule := NewBasicSchedule()

		testAdd1 := time.Date(2006, 1, 2, 15, 4, 5, 999, time.UTC)
		testAdd2 := time.Date(2006, 1, 2, 15, 4, 5, 900, time.UTC)
		testAdd3 := time.Date(2006, 1, 2, 15, 4, 5, 950, time.UTC)
		schedule.Set("test1", NextTime(testAdd1))
		schedule.Set("test2", NextTime(testAdd2))
		schedule.Set("test3", NextTime(testAdd3))

		testAfter := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
		result := schedule.Next(testAfter)

		if !result.Equal(testAdd2) {
			t.Fatalf("Next did not return the earliest Nexter when multiple " +
				"Nexters were added")
		}
	})

	t.Run("Ealiest possibe should prevent further checks", func(t *testing.T) {
		schedule := NewBasicSchedule()

		testAdd1 := time.Date(2001, 2, 0, 0, 0, 0, 0, time.UTC)
		schedule.Set("test1", NextTime(testAdd1))

		schedule.Set("test2", NextFunc(func(after time.Time) time.Time {
			return time.Date(2001, 2, 0, 0, 0, 0, 1, time.UTC)
		}))

		testAfter := testAdd1.Add(time.Duration(-1))
		result := schedule.Next(testAfter)

		if !result.Equal(testAdd1) {
			t.Fatalf("Next did not return an ealiest possible")
		}
	})

	t.Run("Tick should pass matching tasks to TaskRunner", func(t *testing.T) {
		schedule := NewBasicSchedule()

		testAfter := time.Date(2001, 2, 0, 0, 0, 0, 0, time.UTC)
		schedule.Set("test", NextTime(testAfter))

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
		results, err := schedule.Tick(runner, testAfter)

		if err != nil {
			t.Fatalf("Tick gave an unexpected error: %s", err)
		}

		if passedTask != "test" {
			t.Fatalf("Passed taskrunner did not receive expected task")
		}

		if _, ok := results["test"]; !ok {
			t.Fatalf("Task name not defined in results after tick")
		}

		if results["test"] != taskResult {
			t.Fatalf("Task Result was not passed back in the results map")
		}
	})

	t.Run("Tick should abort in the event of an error", func(t *testing.T) {
		schedule := NewBasicSchedule()

		testAfter := time.Date(2001, 2, 0, 0, 0, 0, 0, time.UTC)
		schedule.Set("test1", NextTime(testAfter))
		schedule.Set("test2", NextTime(testAfter))

		runs := 0
		runner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			runs += 1
			return &taskrunner.TaskStatus{
					Ran:      true,
					Error:    nil,
					Warnings: []error{},
					Output:   nil,
				},
				errors.New("intentional")
		})
		_, err := schedule.Tick(runner, testAfter)

		if err == nil {
			t.Fatalf("intionally-failing Tick did not return an error")
		}

		if runs != 1 {
			t.Fatalf("Always-failing Tick with two tasks did not run exactly once")
		}
	})
}
