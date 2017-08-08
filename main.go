package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/wpalmer/ecscron/schedule"
	"github.com/wpalmer/ecscron/schedule/crontab"
	"github.com/wpalmer/ecscron/schedule/retry"
	"github.com/wpalmer/ecscron/taskrunner"
	"github.com/wpalmer/ecscron/taskrunner/ecstaskrunner"
	"github.com/wpalmer/ecscron/taskrunner/tweak"
)

const (
	DEBUG_ERROR  = 0
	DEBUG_INFO   = 1
	DEBUG_DETAIL = 2
	DEBUG_STATUS = 5
)

type simulatedStatus struct {
	TaskName string
}

func main() {
	var async string
	var prevTick time.Time
	var nextTick time.Time
	var timezone string
	var cluster string
	var prefix string
	var suffix string
	var region string
	var filePath string
	var doRetry bool
	var retryCount int64
	var simulate bool
	var verbosity int
	first := true

	flag.StringVar(&timezone, "timezone", "UTC", "The TimeZone in which to evaluate cron expressions")
	flag.StringVar(&async, "async", "", "The \"last run\" of cron (to resume after interruption) in YYYY-MM-DD HH:mm:ss format")
	flag.BoolVar(&doRetry, "retry", false, "When true, any failed run-task will be attempted again in the next iteration (same as -retry-count=-1)")
	flag.Int64Var(&retryCount, "retry-count", 0, "The number of times to retry a failed run-task before giving up (-1 means forever)")
	flag.StringVar(&cluster, "cluster", "", "The ECS Cluster on which to run tasks")
	flag.StringVar(&region, "region", "", "The AWS Region in which the ECS Cluster resides")
	flag.StringVar(&filePath, "crontab", "/etc/ecscrontab", "The location of the crontab file to parse")
	flag.StringVar(&prefix, "prefix", "", "An optional prefix to add to all ECS Task names within the crontab")
	flag.StringVar(&suffix, "suffix", "", "An optional suffix to add to all ECS Task names within the crontab")
	flag.BoolVar(&simulate, "simulate", false, "When true, don't actually run anything, only print what would be run")
	flag.IntVar(&verbosity, "debug", 0, "Debug level 0 = errors/warnings, 1 = run info, 2 = detail, 5 = status")
	flag.Parse()

	location, err := time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("Failed to parse timzeone: %s", err)
	}

	if async != "" {
		prevTick, err = time.ParseInLocation("2006-05-04 15:02:01", async, location)
		if err != nil {
			log.Fatalf("Failed to parse time of last run: %s", err)
		}

		first = false
	}

	if doRetry && retryCount == int64(0) {
		retryCount = -1
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening crontab: %s", err)
	}

	var sched schedule.Schedule
	table := crontab.NewCrontab()
	if ok, err := table.Load(file); !ok {
		log.Fatalf("Error loading crontab: %s", err)
	}
	sched = table

	if retryCount != 0 {
		numAttempts := retryCount
		if numAttempts > 0 {
			numAttempts += 1
		}

		sched = retry.NewRetrySchedule(sched, numAttempts)
	}

	var runner taskrunner.TaskRunner
	if simulate {
		runner = taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
			log.Printf("[-simulate] Running: %s", task)
			return &taskrunner.TaskStatus{Ran: true, Output: &simulatedStatus{TaskName: task}}, nil
		})
	} else {
		awsConfig := aws.NewConfig()
		if region != "" {
			awsConfig = awsConfig.WithRegion(region)
		}

		awsSession := session.Must(session.NewSession(awsConfig))
		ecsService := ecs.New(awsSession)

		runner = ecstaskrunner.NewEcsTaskRunner(ecsService, cluster)
	}

	if prefix != "" || suffix != "" {
		runner = tweak.NewTweakTaskRunner(runner, func(task string) string {
			return fmt.Sprintf("%s%s%s", prefix, task, suffix)
		})
	}

	if first {
		prevTick = time.Now().In(location)
	}

	for {
		nextTick = sched.Next(prevTick)
		pause := nextTick.Sub(time.Now().In(location))
		if pause < time.Duration(0) {
			log.Printf("Cron tasks running slowly: %0.2f seconds late entering tick scheduled for %v",
				(pause * time.Duration(-1)).Seconds(), nextTick)
		} else {
			if verbosity >= DEBUG_STATUS {
				log.Printf("Sleeping for %0.2f seconds", pause.Seconds())
			}
			time.Sleep(pause)
		}

		prevTick = nextTick
		results, err := sched.Tick(runner, nextTick)
		if err != nil {
			log.Fatalf("Fatal error in tick: %s", err)
		}

		for task, result := range results {
			if result.Ran {
				if verbosity >= DEBUG_DETAIL {

					switch info := result.Info.(type) {
					default:
					case *retry.RetryInfo:
						if info.MaxRetries > 0 {
							log.Printf("Retrying %s (attempt %d of %d)\n",
								task, info.Attempt, info.MaxRetries)
						} else {
							log.Printf("Retrying %s (attempt %d)\n", task, info.Attempt)
						}
					}

					switch output := result.Output.(type) {
					default:
						log.Printf("%s Scheduled to Run via an unknown method", task)
					case *simulatedStatus:
						log.Printf("[-simulate] %s Would have been scheduled to run as %s", task, output.TaskName)
					case *ecs.RunTaskOutput:
						for _, scheduledTask := range output.Tasks {
							log.Printf("%s Scheduled to run on Container Instance %s using Task Definition %s\n",
								task, *scheduledTask.ContainerInstanceArn, *scheduledTask.TaskDefinitionArn)
						}
					}
				}
			} else {
				if result.Error != nil {
					log.Printf("Error when running task '%s': %s", task, result.Error)
				}

				for _, warning := range result.Warnings {
					log.Printf("Warning when running task '%s': %s", task, warning)
				}
			}
		}
	}
}
