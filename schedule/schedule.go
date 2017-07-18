package schedule

import (
	"time"

	"github.com/wpalmer/ecscron/taskrunner"
)

type Nexter interface {
	// starting from "after", returns the time of the "next" event in the schedule,
	// which must be greater than "after".
	// A zero-value is used to indicate that no "next" event is known when
	// starting from the given time. Due to this, results are undefined when
	// "after" is a zero-value.
	Next(after time.Time) time.Time
}

type NextTime time.Time

func (t NextTime) Next(after time.Time) time.Time {
	if time.Time(t).After(after) {
		return time.Time(t)
	}

	return time.Time{}
}

type NextFunc func(after time.Time) time.Time

func (f NextFunc) Next(after time.Time) time.Time {
	return f(after)
}

type NextList []Nexter

func (list *NextList) Add(nexter Nexter) {
	*list = append(*list, nexter)
}

func (list *NextList) Clear() {
	*list = []Nexter{}
}

func (list *NextList) Next(after time.Time) time.Time {
	var earliest time.Time

	for _, nexter := range *list {
		next := nexter.Next(after)

		if !next.IsZero() && (earliest.IsZero() || next.Before(earliest)) {
			earliest = next

			if earliest.Equal(after.Add(time.Nanosecond)) {
				break
			}
		}
	}

	return earliest
}

// The Schedule controls when tasks are run. It is in charge of both determining
// when something needs to happen, and the actual passing of scheduled events to
// a TaskRunner.
// This allows a scheduler to independently decide not only when the "scheduled
// tasks" should be run, but also any retries, and anything to do with internal
// state manipulation.
type Schedule interface {
	// Schedules can be used as Nexters, Recursively
	Nexter

	// process all scheduled tasks which should run specifically "at" the given
	// time. In practice, the "at" parameter will be the result of the previous
	// call of Next(...)
	Tick(runner taskrunner.TaskRunner, at time.Time) (map[string]*taskrunner.TaskStatus, error)
}

type BasicSchedule struct {
	table map[string]Nexter
}

func NewBasicSchedule() *BasicSchedule {
	return &BasicSchedule{make(map[string]Nexter)}
}

func (s *BasicSchedule) Set(name string, nexter Nexter) {
	s.table[name] = nexter
}

func (s *BasicSchedule) Next(after time.Time) time.Time {
	var earliest time.Time

	for _, nexter := range s.table {
		next := nexter.Next(after)

		if !next.IsZero() && (earliest.IsZero() || next.Before(earliest)) {
			earliest = next

			if earliest.Equal(after.Add(time.Nanosecond)) {
				break
			}
		}
	}

	return earliest
}

func (s *BasicSchedule) Tick(runner taskrunner.TaskRunner, at time.Time) (map[string]*taskrunner.TaskStatus, error) {
	results := make(map[string]*taskrunner.TaskStatus)
	after := at.Add(time.Duration(-1))

	for name, entry := range s.table {
		next := entry.Next(after)
		if !next.IsZero() && next.Equal(at) {
			result, err := runner.RunTask(name)

			if err != nil {
				return results, err
			}

			results[name] = result
		}
	}

	return results, nil
}
