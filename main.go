package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/gorhill/cronexpr"
)

const (
	DEBUG_ERROR  = 0
	DEBUG_INFO   = 1
	DEBUG_DETAIL = 2
	DEBUG_STATUS = 5
)

type CronTabEntry struct {
	FailuresSinceLastSuccess int64
	Schedule                 []*cronexpr.Expression
}

type CronTab map[string]*CronTabEntry

var cronExprMatcher *regexp.Regexp
var ignoredMatcher *regexp.Regexp

func init() {
	ignoredMatcher = regexp.MustCompile("^\\s*(?:#.*)?$")
	cronExprMatcher = regexp.MustCompile("^\\s*" +
		"(" +
		"@\\S+" + // Predefined
		"|" +
		"[-0-9*/,]+\\s+" + // Seconds
		"[-0-9*/,]+\\s+" + // Minutes
		"[-0-9*/,]+\\s+" + // Hours
		"[-0-9*/,LW]+\\s+" + // Day of month
		"[-0-9A-Za-z*/,]+\\s+" + // Month
		"[-0-9A-Za-z*/,L#]+\\s+" + // Day of week
		"[-0-9*/,]+" + // Year
		"|" +
		"[-0-9*/,]+\\s+" + // Minutes
		"[-0-9*/,]+\\s+" + // Hours
		"[-0-9*/,LW]+\\s+" + // Day of month
		"[-0-9A-Za-z*/,]+\\s+" + // Month
		"[-0-9A-Za-z*/,L#]+\\s+" + // Day of week
		"[-0-9*/,]+" + // Year
		"|" +
		"[-0-9*/,]+\\s+" + // Minutes
		"[-0-9*/,]+\\s+" + // Hours
		"[-0-9*/,LW]+\\s+" + // Day of month
		"[-0-9A-Za-z*/,]+\\s+" + // Month
		"[-0-9A-Za-z*/,L#]+" + // Day of week
		")" +
		"\\s+" +
		"(\\S+)" +
		"(?:\\s+#.*)?" +
		"\\s*$")
}

func ParseLine(line string) (ok bool, task string, expression *cronexpr.Expression) {
	if ignoredMatcher.MatchString(line) {
		return false, "", nil
	}

	matches := cronExprMatcher.FindStringSubmatch(line)

	if len(matches) == 0 {
		log.Printf("Unknown crontab line format: %s", line)
		return false, "", nil
	}

	expr, err := cronexpr.Parse(matches[1])

	if expr == nil {
		log.Printf("Failed to parse cron expression '%s': %s", line, err)
		return false, "", nil
	}

	return true, matches[2], expr
}

func main() {
	var async string
	var prevTick time.Time
	var timezone string
	var cluster string
	var prefix string
	var suffix string
	var region string
	var filePath string
	var retry bool
	var retryCount int64
	var simulate bool
	var verbosity int
	first := true

	flag.StringVar(&timezone, "timezone", "UTC", "The TimeZone in which to evaluate cron expressions")
	flag.StringVar(&async, "async", "", "The \"last run\" of cron (to resume after interruption) in YYYY-MM-DD HH:mm:ss format")
	flag.BoolVar(&retry, "retry", false, "When true, any failed run-task will be attempted again in the next iteration (same as -retry-count=-1)")
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

	if retry && retryCount == int64(0) {
		retryCount = -1
	}

	if retryCount != int64(0) {
		retry = true
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}

	crontab := make(CronTab)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ok, task, expr := ParseLine(scanner.Text())
		if !ok {
			continue
		}

		task = fmt.Sprintf("%s%s%s", prefix, task, suffix)

		if _, ok := crontab[task]; !ok {
			crontab[task] = new(CronTabEntry)
		}
		crontab[task].Schedule = append(crontab[task].Schedule, expr)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	awsConfig := aws.NewConfig()
	if region != "" {
		awsConfig = awsConfig.WithRegion(region)
	}

	awsSession := session.Must(session.NewSession(awsConfig))
	ecsService := ecs.New(awsSession)
	for {
		tick := time.Now().In(location)
		tickBase := time.Date(tick.Year(), tick.Month(), tick.Day(), tick.Hour(), tick.Minute(), 0, 0, location)
		nextTick := tickBase.Add(time.Minute)

		if !first {
			for task := range crontab {
				doRun := false
				var exprNext time.Time

				if crontab[task].FailuresSinceLastSuccess > int64(0) && retry {
					if retryCount > 0 {
						if crontab[task].FailuresSinceLastSuccess <= retryCount {
							log.Printf("Retrying %s (this is attempt %d/%d)",
								task, crontab[task].FailuresSinceLastSuccess+int64(1), retryCount+int64(1))
							doRun = true
						} else {
							// Task has completely failed to run. Reset the counter so it can run at the next scheduled time.
							crontab[task].FailuresSinceLastSuccess = 0
						}
					} else {
						log.Printf("Retrying %s (this is attempt %d)", task, crontab[task].FailuresSinceLastSuccess+int64(1))
						doRun = true
					}
				}

				// if doRun has not been set by retry rules, check the schedule
				if !doRun {
					if verbosity >= DEBUG_STATUS {
						log.Printf("Checking schedule for %s", task)
					}

					for i, expr := range crontab[task].Schedule {
						exprNext = expr.Next(prevTick)
						if verbosity >= DEBUG_STATUS {
							log.Printf("Next run of %s should be at %s (vs now: %s) according to rule %d", task, exprNext, tick, i)
						}

						if exprNext.Before(tick) || exprNext.Equal(tick) {
							doRun = true
							break
						}
					}
				}

				if !doRun {
					continue
				}

				// Default to assuming failure. In the event of success, this will be overridden below
				crontab[task].FailuresSinceLastSuccess += 1

				if simulate || verbosity >= DEBUG_INFO {
					log.Printf("Running: %s", task)
				}
				if simulate {
					continue
				}

				listInput := &ecs.ListTasksInput{}
				if cluster != "" {
					listInput.SetCluster(cluster)
				}
				listInput.SetStartedBy(task)
				listInput.SetMaxResults(1)
				listResult, err := ecsService.ListTasks(listInput)
				if err != nil {
					log.Printf("Failed to ListTasks looking for '%s' on cluster '%s': %s", task, cluster, err)
					continue
				}

				if len(listResult.TaskArns) > 0 {
					log.Printf("Skipping Task '%s', which is still running on cluster '%s'", task, cluster)
					continue
				}

				runInput := &ecs.RunTaskInput{}
				if cluster != "" {
					runInput.SetCluster(cluster)
				}
				runInput.SetStartedBy(task)
				runInput.SetTaskDefinition(task)
				runResult, err := ecsService.RunTask(runInput)
				if err != nil {
					log.Printf("Failed to RunTask '%s' on cluster '%s': %s", task, cluster, err)
					continue
				}

				if len(runResult.Failures) > 0 {
					for _, failure := range runResult.Failures {
						log.Printf("Failure during RunTask '%s' on cluster '%s': %s", task, cluster, failure.GoString())
					}
					continue
				}

				if verbosity >= DEBUG_DETAIL {
					for _, scheduledTask := range runResult.Tasks {
						log.Printf("%s Scheduled to run on Container Instance %s using Task Definition %s\n",
							task, *scheduledTask.ContainerInstanceArn, *scheduledTask.TaskDefinitionArn)
					}
				}

				crontab[task].FailuresSinceLastSuccess = 0
			}
		}

		pause := nextTick.Sub(time.Now().In(location))
		if pause < time.Duration(0) {
			log.Printf("Cron tasks running slowly: needed to skip %d runs", 1+((pause*time.Duration(int64(-1)))/time.Minute))
		} else {
			if verbosity >= DEBUG_STATUS {
				log.Printf("Sleeping for %0.2f seconds", pause.Seconds())
			}
			time.Sleep(pause)
		}

		first = false
		prevTick = tick
	}
}
