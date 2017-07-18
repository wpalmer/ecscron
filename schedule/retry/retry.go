package retry

import (
	"fmt"
	"time"

	"github.com/wpalmer/ecscron/schedule"
	"github.com/wpalmer/ecscron/taskrunner"
	"github.com/wpalmer/ecscron/taskrunner/suppression"
)

type retryTaskStatus struct {
	attempts int64
	ok       bool
}

type RetrySchedule struct {
	schedule   schedule.Schedule
	maxRetries int64
	tasks      map[string]*retryTaskStatus
}

func NewRetrySchedule(schedule schedule.Schedule, numRetries int64) *RetrySchedule {
	return &RetrySchedule{
		schedule:   schedule,
		maxRetries: numRetries,
		tasks:      make(map[string]*retryTaskStatus),
	}
}

func (r *RetrySchedule) Next(from time.Time) time.Time {
	// If any tasks need a retry, schedule them for the next whole-minute
	for _, status := range r.tasks {
		if !status.ok && r.maxRetries < int64(0) || status.attempts <= r.maxRetries {
			return time.Date(from.Year(), from.Month(), from.Day(), from.Hour(),
				from.Minute(), 0, 0, from.Location()).Add(time.Minute)
		}
	}

	return r.schedule.Next(from)
}

func (r *RetrySchedule) Tick(runner taskrunner.TaskRunner, at time.Time) (map[string]*taskrunner.TaskStatus, error) {
	// wrap the runner in a SuppressionTaskRunner so, if we retry something that is also schedued, we don't run it twice
	suppressor := suppression.NewSuppressionTaskRunner(runner)

	runstatus := make(map[string]*taskrunner.TaskStatus)

	for task, status := range r.tasks {
		if !status.ok && r.maxRetries < 0 || status.attempts < r.maxRetries {
			suppressor.Suppress(task, fmt.Errorf("Skipping scheduled run of %s because it was already retried this tick", task))
			r.tasks[task].attempts += 1

			newstatus, err := runner.RunTask(task)
			if err != nil {
				return nil, err
			}

			runstatus[task] = newstatus
			r.tasks[task].ok = newstatus.Ran
		}
	}

	scheduledStatus, err := r.schedule.Tick(suppressor, at)
	for task, newstatus := range scheduledStatus {
		// don't overwrite status that we've already determined by retrying
		if _, ok := runstatus[task]; !ok {
			r.tasks[task] = &retryTaskStatus{
				attempts: 1,
				ok:       newstatus.Ran,
			}
			runstatus[task] = newstatus
		}
	}

	return runstatus, err
}
