package ecstaskrunner

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
)

type ecstest struct {
	list func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	run  func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error)
}

func (ecs ecstest) ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
	return ecs.list(input)
}
func (ecs ecstest) RunTask(input *ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
	return ecs.run(input)
}

func TestEcsTaskRunner(t *testing.T) {
	t.Run("ListTask errors should be returned as errors", func(t *testing.T) {
		service := ecstest{
			func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
				return nil, errors.New("intentional error")
			},
			func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
				t.Fatalf("RunTask was called when listing tasks")
				return nil, errors.New("never reached")
			},
		}

		runner := NewEcsTaskRunner(service, "clustername")
		_, err := runner.RunTask("taskname")

		if err == nil {
			t.Fatalf("An error from ECS ListTask was not passed-through")
		}
	})

	t.Run("Already-running tasks should not re-run", func(t *testing.T) {
		service := ecstest{
			func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
				taskArn := "arn:test"
				return &ecs.ListTasksOutput{
					NextToken: nil,
					TaskArns:  []*string{&taskArn},
				}, nil
			},
			func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
				t.Fatalf("RunTask was called when listing tasks")
				return nil, errors.New("never reached")
			},
		}

		runner := NewEcsTaskRunner(service, "clustername")
		result, err := runner.RunTask("taskname")

		if err != nil {
			t.Fatalf("Already-running task resulted in error")
		}

		if result.Ran {
			t.Fatalf("Already-running task ran")
		}

		if len(result.Warnings) == 0 {
			t.Fatalf("No reason given when Already-running task was skipped")
		}
	})

	t.Run("RunTask errors should be returned as errors", func(t *testing.T) {
		service := ecstest{
			func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
				return &ecs.ListTasksOutput{
					NextToken: nil,
					TaskArns:  []*string{},
				}, nil
			},
			func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
				return nil, errors.New("intentional error")
			},
		}

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
		service := ecstest{
			func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
				return &ecs.ListTasksOutput{
					NextToken: nil,
					TaskArns:  []*string{},
				}, nil
			},
			func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
				taskArn := "arn:test"
				reason := "intentional failure"
				return &ecs.RunTaskOutput{
					Failures: []*ecs.Failure{
						&ecs.Failure{
							Arn:    &taskArn,
							Reason: &reason,
						},
					},
					Tasks: []*ecs.Task{},
				}, nil
			},
		}

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
	})

	t.Run("RunTask without fail should be noted as success", func(t *testing.T) {
		service := ecstest{
			func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
				return &ecs.ListTasksOutput{
					NextToken: nil,
					TaskArns:  []*string{},
				}, nil
			},
			func(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
				return &ecs.RunTaskOutput{
					Failures: []*ecs.Failure{},
					Tasks:    []*ecs.Task{},
				}, nil
			},
		}

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
