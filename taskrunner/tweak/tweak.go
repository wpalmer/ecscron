package tweak

import (
	"github.com/wpalmer/ecscron/taskrunner"
)

type Translator func(string) string

type TweakTaskRunner struct {
	runner     taskrunner.TaskRunner
	translator Translator
}

func NewTweakTaskRunner(runner taskrunner.TaskRunner, translator Translator) *TweakTaskRunner {
	return &TweakTaskRunner{
		runner:     runner,
		translator: translator,
	}
}

func (r TweakTaskRunner) RunTask(task string) (*taskrunner.TaskStatus, error) {
	return r.runner.RunTask(r.translator(task))
}
