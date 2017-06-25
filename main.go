package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/gorhill/cronexpr"
)

type CronTabEntry struct {
	lastRun     time.Time
	expressions []*cronexpr.Expression
}

const (
	DEBUG_ERROR  = 0
	DEBUG_INFO   = 1
	DEBUG_STATUS = 2
)

type CronTab map[string][]*cronexpr.Expression

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
	var filePath string
	var simulate bool
	var verbosity int
	first := true

	flag.StringVar(&timezone, "timezone", "UTC", "The TimeZone in which to evaluate cron expressions")
	flag.StringVar(&async, "async", "", "The \"last run\" of cron (to resume after interruption) in YYYY-MM-DD HH:mm:ss format")
	flag.StringVar(&filePath, "crontab", "/etc/ecscrontab", "The location of the crontab file to parse")
	flag.BoolVar(&simulate, "simulate", false, "When true, don't actually run anything, only print what would be run")
	flag.IntVar(&verbosity, "debug", 0, "Debug level 0 = errors/warnings, 1 = run info, 2 = status")
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

		crontab[task] = append(crontab[task], expr)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	for {
		tick := time.Now().In(location)
		tickBase := time.Date(tick.Year(), tick.Month(), tick.Day(), tick.Hour(), tick.Minute(), 0, 0, location)
		nextTick := tickBase.Add(time.Minute)

		if !first {
			for task := range crontab {
				doRun := false
				for _, expr := range crontab[task] {
					if exprNext := expr.Next(prevTick); exprNext.Before(tick) || exprNext.Equal(tick) {
						doRun = true
						break
					}
				}

				if !doRun {
					continue
				}

				if simulate || verbosity >= DEBUG_INFO {
					log.Printf("Running: %s\n", task)
				}
				if simulate {
					continue
				}
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
