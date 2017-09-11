package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	var doPause bool
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

	var doDump bool
	var dumpFrom string
	var dumpFromTime time.Time
	var dumpUntil string
	var dumpUntilTime time.Time
	var dumpFormat string
	first := true

	flag.StringVar(&timezone, "timezone", "UTC", "The TimeZone in which to evaluate cron expressions")
	flag.StringVar(&async, "async", "", "The \"last run\" of cron (to resume after interruption) in YYYY-MM-DD HH:mm:ss format")
	flag.BoolVar(&doPause, "pause", false, "Start cron in a 'paused' state, awaiting SIGUSR1 to resume")
	flag.BoolVar(&doRetry, "retry", false, "When true, any failed run-task will be attempted again in the next iteration (same as -retry-count=-1)")
	flag.Int64Var(&retryCount, "retry-count", 0, "The number of times to retry a failed run-task before giving up (-1 means forever)")
	flag.StringVar(&cluster, "cluster", "", "The ECS Cluster on which to run tasks")
	flag.StringVar(&region, "region", "", "The AWS Region in which the ECS Cluster resides")
	flag.StringVar(&filePath, "crontab", "/etc/ecscrontab", "The location of the crontab file to parse")
	flag.StringVar(&prefix, "prefix", "", "An optional prefix to add to all ECS Task names within the crontab")
	flag.StringVar(&suffix, "suffix", "", "An optional suffix to add to all ECS Task names within the crontab")
	flag.BoolVar(&simulate, "simulate", false, "When true, don't actually run anything, only print what would be run")
	flag.IntVar(&verbosity, "debug", 0, "Debug level 0 = errors/warnings, 1 = run info, 2 = detail, 5 = status")

	flag.BoolVar(&doDump, "dump", false, "Rather than running the cron, output a summary of the schedule")
	flag.StringVar(&dumpFrom, "dump-from", "", "Output the schedule up starting from the specified time, in YYYY-MM-DD HH:mm:ss format")
	flag.StringVar(&dumpUntil, "dump-until", "", "Output the schedule up until the specified time, in YYYY-MM-DD HH:mm:ss format")
	flag.StringVar(&dumpFormat, "dump-format", "", "Output the schedule in the specified format. Currently the only supported format is 'json'")
	flag.Parse()

	location, err := time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("Failed to parse timzeone: %s", err)
	}

	if async != "" {
		prevTick, err = time.ParseInLocation("2006-01-02 15:04:05", async, location)
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

	if dumpFrom != "" {
		doDump = true
		dumpFromTime, err = time.ParseInLocation("2006-01-02 15:04:05", dumpFrom, location)
		if err != nil {
			log.Fatalf("Failed to parse time to dump from: %s", err)
		}
	} else {
		dumpFromTime = time.Now()
	}

	if dumpUntil != "" {
		doDump = true
		dumpUntilTime, err = time.ParseInLocation("2006-01-02 15:04:05", dumpUntil, location)
		if err != nil {
			log.Fatalf("Failed to parse time to dump until: %s", err)
		}
	} else {
		dumpUntilTime = dumpFromTime.Add(time.Hour * 24)
	}

	if dumpFormat == "" {
		dumpFormat = "json"
	} else {
		doDump = true
		if dumpFormat != "json" {
			log.Fatalf("Unknown dump format: %s", dumpFormat)
		}
	}

	if doDump {
		_, err := schedule.DumpJson(os.Stdout, sched, dumpFromTime.Add(-1), dumpUntilTime)
		if err != nil {
			log.Fatalf("Failed to dump schedule: %s", err)
		}

		fmt.Printf("\n")
		os.Exit(0)
	}

	if first {
		prevTick = time.Now().In(location)
	}

	signals := make(chan os.Signal, 1)
	if doPause {
		signals <- syscall.SIGUSR1
	}
	signal.Notify(signals, syscall.SIGUSR1)

	ticks := make(chan time.Time, 1)

	for {
		nextTick = sched.Next(prevTick)
		pause := nextTick.Sub(time.Now().In(location))
		if pause < time.Duration(0) {
			log.Printf("Cron tasks running slowly: %0.2f seconds late entering tick scheduled for %v",
				(pause * time.Duration(-1)).Seconds(), nextTick)
			ticks <- nextTick
		} else {
			if verbosity >= DEBUG_STATUS {
				log.Printf("Sleeping for %0.2f seconds", pause.Seconds())
			}
			go func() {
				time.Sleep(pause)
				ticks <- nextTick
			}()
		}

		ticked := false
		for ticked == false {
			select {
			case <-ticks:
				ticked = true
			case <-signals:
				if doPause {
					doPause = false
					log.Printf("Pausing, send SIGUSR1 to resume...")
				} else {
					log.Printf("Received SIGUSR1, pausing...")
				}

				<-signals
				log.Printf("Received SIGUSR1 while paused, resuming...")
			default:
			}
		}

		prevTick = nextTick
		results, err := sched.Tick(runner, nextTick)
		if err != nil {
			log.Fatalf("Fatal error in tick: %s", err)
		}

		for task, result := range results {
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
			}

			if result.Ran {
				if verbosity >= DEBUG_DETAIL {
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
