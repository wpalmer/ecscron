package ecstaskrunner

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/wpalmer/ecscron/taskrunner"
)

type RunTasker interface {
	RunTask(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error)
}

type ListTaskser interface {
	ListTasks(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
}

type MinimalECSAPI interface {
	ListTaskser
	RunTasker
}

type EcsTaskRunner struct {
	service RunTasker
	cluster string
}

type EcsSkipRunningTaskRunner struct {
	service ListTaskser
	cluster string
	runner  taskrunner.TaskRunner
}

func NewEcsTaskRunner(service RunTasker, cluster string) *EcsTaskRunner {
	return &EcsTaskRunner{service: service, cluster: cluster}
}

func (r *EcsTaskRunner) RunTask(task string) (*taskrunner.TaskStatus, error) {
	runInput := &ecs.RunTaskInput{}
	if r.cluster != "" {
		runInput.SetCluster(r.cluster)
	}

	startedBy := fmt.Sprintf("%x", md5.Sum([]byte(task)))
	runInput.SetStartedBy(startedBy)
	runInput.SetTaskDefinition(task)
	runResult, err := r.service.RunTask(runInput)
	if err != nil {
		return &taskrunner.TaskStatus{
			Ran:      false,
			Error:    err,
			Warnings: []error{},
			Output:   runResult,
		}, nil
	}

	if len(runResult.Failures) > 0 {
		warnings := []error{}
		for _, failure := range runResult.Failures {
			warnings = append(warnings,
				fmt.Errorf("Failure during RunTask '%s' on cluster '%s': %s",
					task, r.cluster, strings.Replace(failure.GoString(), "\n", " ", -1)))
		}
		return &taskrunner.TaskStatus{
			Ran:      false,
			Error:    nil,
			Warnings: warnings,
			Output:   runResult,
		}, nil
	}

	return &taskrunner.TaskStatus{
		Ran:      true,
		Error:    nil,
		Warnings: []error{},
		Output:   runResult,
	}, nil
}

func NewEcsSkipRunningTaskRunner(service ListTaskser, cluster string, runner taskrunner.TaskRunner) *EcsSkipRunningTaskRunner {
	return &EcsSkipRunningTaskRunner{service: service, cluster: cluster, runner: runner}
}

func (r *EcsSkipRunningTaskRunner) RunTask(task string) (*taskrunner.TaskStatus, error) {
	listInput := &ecs.ListTasksInput{}
	if r.cluster != "" {
		listInput.SetCluster(r.cluster)
	}

	startedBy := fmt.Sprintf("%x", md5.Sum([]byte(task)))
	listInput.SetStartedBy(startedBy)
	listInput.SetMaxResults(1)
	listResult, err := r.service.ListTasks(listInput)
	if err != nil {
		return nil,
			fmt.Errorf("Failed to ListTasks looking for '%s' on cluster '%s': %s",
				task, r.cluster, err)
	}

	if len(listResult.TaskArns) > 0 {
		return &taskrunner.TaskStatus{
			Ran:     false,
			Running: true,
			Error:   nil,
			Warnings: []error{
				fmt.Errorf("Skipping Task '%s', which is still running on cluster '%s'",
					task, r.cluster),
			},
			Output: nil,
		}, nil
	}

	return r.runner.RunTask(task)
}
