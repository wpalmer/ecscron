package ecstaskrunner

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/wpalmer/ecscron/taskrunner"
)

type listTasksFunc func(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)

func (f listTasksFunc) ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
	return f(input)
}

type runTaskFunc func(input *ecs.RunTaskInput) (*ecs.RunTaskOutput, error)

func (f runTaskFunc) RunTask(input *ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
	return f(input)
}

func TestEcsSkipRunningTaskRunner(t *testing.T) {
	t.Run("ListTask errors should be returned as errors", func(t *testing.T) {
		service := listTasksFunc(func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
			return nil, errors.New("intentional error")
		})

		innerRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			t.Fatalf("inner taskrunner was called when listing tasks")
			return &taskrunner.TaskStatus{
				Ran:      false,
				Error:    nil,
				Warnings: []error{},
				Output:   "unexpected",
			}, nil
		})

		runner := NewEcsSkipRunningTaskRunner(service, "clustername", innerRunner)
		_, err := runner.RunTask("taskname")

		if err == nil {
			t.Fatalf("An error from ECS ListTask was not passed-through")
		}
	})

	t.Run("Already-running tasks should not re-run", func(t *testing.T) {
		service := listTasksFunc(func(i *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
			startedBy := *i.StartedBy
			if startedBy != "c48ff9aade4a76b8a3ea9767be30800b" {
				t.Fatalf("StartedBy was not md5sum of 'taskname' (%s)", startedBy)
			}

			taskArn := "arn:test"
			return &ecs.ListTasksOutput{
				NextToken: nil,
				TaskArns:  []*string{&taskArn},
			}, nil
		})

		innerRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			t.Fatalf("inner taskrunner was called when listing tasks")
			return &taskrunner.TaskStatus{
				Ran:      false,
				Error:    nil,
				Warnings: []error{},
				Output:   "unexpected",
			}, nil
		})
		runner := NewEcsSkipRunningTaskRunner(service, "clustername", innerRunner)
		result, err := runner.RunTask("taskname")

		if err != nil {
			t.Fatalf("Already-running task resulted in error")
		}

		if result.Ran {
			t.Fatalf("Already-running task ran")
		}

		if !result.Running {
			t.Fatalf("Already-running task not marked as running")
		}

		if len(result.Warnings) == 0 {
			t.Fatalf("No reason given when Already-running task was skipped")
		}
	})

	t.Run("RunTask without results should call inner runner", func(t *testing.T) {
		service := listTasksFunc(func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{
				NextToken: nil,
				TaskArns:  []*string{},
			}, nil
		})

		didRun := false
		innerRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			didRun = true
			return &taskrunner.TaskStatus{
				Ran:      true,
				Error:    nil,
				Warnings: []error{},
				Output:   "unexpected",
			}, nil
		})

		runner := NewEcsSkipRunningTaskRunner(service, "clustername", innerRunner)
		result, err := runner.RunTask("taskname")

		if !didRun {
			t.Fatalf("empty ListTasks result did not cause inner runner to run")
		}

		if err != nil {
			t.Fatalf("RunTask Success resulted in error")
		}

		if !result.Ran {
			t.Fatalf("RunTask Success reported that the task did not run")
		}
	})
}

func TestEcsTaskRunner(t *testing.T) {
	t.Run("RunTask errors should be returned as errors", func(t *testing.T) {
		service := runTaskFunc(func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
			return nil, errors.New("intentional error")
		})

		runner := NewEcsTaskRunner(service, "clustername")
		status, err := runner.RunTask("taskname")

		if err != nil {
			t.Fatalf("An error from ECS RunTask was passed-through")
		}

		if status.Ran {
			t.Fatalf("Runner reported that task Ran after an error from ECS RunTask")
		}

		if status.Error == nil {
			t.Fatalf("Runner did not return a Task Error after an error from ECS RunTask")
		}
	})

	t.Run("RunTask failures should be noted as warnings", func(t *testing.T) {
		service := runTaskFunc(func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
			taskArn := "arn:test"
			reason := "intentional\nfailure"
			return &ecs.RunTaskOutput{
				Failures: []*ecs.Failure{
					&ecs.Failure{
						Arn:    &taskArn,
						Reason: &reason,
					},
				},
				Tasks: []*ecs.Task{},
			}, nil
		})

		runner := NewEcsTaskRunner(service, "clustername")
		result, err := runner.RunTask("taskname")

		if err != nil {
			t.Fatalf("RunTask Failure (not error) resulted in error")
		}

		if result.Ran {
			t.Fatalf("RunTask Failure reported that the task ran")
		}

		if len(result.Warnings) < 1 {
			t.Fatalf("RunTask Failure did not return a warning")
		}

		for _, warning := range result.Warnings {
			if strings.Contains(fmt.Sprintf("%s", warning), "\n") {
				t.Fatalf("RunTask Failure contained a newline")
			}
		}
	})

	t.Run("RunTask without fail should be noted as success", func(t *testing.T) {
		service := runTaskFunc(func(i *ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
			startedBy := *i.StartedBy
			if startedBy != "c48ff9aade4a76b8a3ea9767be30800b" {
				t.Fatalf("StartedBy was not md5sum of 'taskname' (%s)", startedBy)
			}

			return &ecs.RunTaskOutput{
				Failures: []*ecs.Failure{},
				Tasks:    []*ecs.Task{},
			}, nil
		})

		runner := NewEcsTaskRunner(service, "clustername")
		result, err := runner.RunTask("taskname")

		if err != nil {
			t.Fatalf("RunTask Success resulted in error")
		}

		if !result.Ran {
			t.Fatalf("RunTask Success reported that the task did not run")
		}
	})
}
