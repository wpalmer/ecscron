package ecstaskrunner

import (
	"crypto/md5"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/wpalmer/ecscron/taskrunner"
)

type MinimalECSAPI interface {
	ListTasks(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	RunTask(*ecs.RunTaskInput) (*ecs.RunTaskOutput, error)
}

type EcsTaskRunner struct {
	service MinimalECSAPI
	cluster string
}

func NewEcsTaskRunner(service MinimalECSAPI, cluster string) *EcsTaskRunner {
	return &EcsTaskRunner{service: service, cluster: cluster}
}

func (r *EcsTaskRunner) RunTask(task string) (*taskrunner.TaskStatus, error) {
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
			Ran:   false,
			Error: nil,
			Warnings: []error{
				fmt.Errorf("Skipping Task '%s', which is still running on cluster '%s'",
					task, r.cluster),
			},
			Output: nil,
		}, nil
	}

	runInput := &ecs.RunTaskInput{}
	if r.cluster != "" {
		runInput.SetCluster(r.cluster)
	}
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
					task, r.cluster, failure.GoString()))
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
