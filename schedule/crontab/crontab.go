package crontab

import (
	"bufio"
	"fmt"
	"io"
	"regexp"

	"github.com/gorhill/cronexpr"
	"github.com/wpalmer/ecscron/schedule"
)

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

type Crontab struct {
	schedule.BasicSchedule
	table map[string]*schedule.NextList
}

func NewCrontab() *Crontab {
	return &Crontab{
		*schedule.NewBasicSchedule(),
		make(map[string]*schedule.NextList),
	}
}

func (s *Crontab) Add(task string, nexter schedule.Nexter) {
	var list *schedule.NextList
	var ok bool

	list, ok = s.table[task]
	if !ok {
		list = &schedule.NextList{}
		s.table[task] = list
		s.Set(task, list)
	}

	list.Add(nexter)
}

func (s *Crontab) Clear(task string) {
	list, ok := s.table[task]
	if ok {
		list.Clear()
	}
}

func (s *Crontab) Parse(line string) (bool, error) {
	matches := cronExprMatcher.FindStringSubmatch(line)

	if len(matches) == 0 {
		return false, fmt.Errorf("Unknown crontab line format")
	}

	expr, err := cronexpr.Parse(matches[1])
	if expr == nil {
		return false, fmt.Errorf("Failed to parse cron expression: %s", err)
	}

	s.Add(matches[2], expr)
	return true, nil
}

func (s *Crontab) Load(r io.Reader) (bool, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if ignoredMatcher.MatchString(line) {
			continue
		}

		if ok, err := s.Parse(line); !ok {
			return false,
				fmt.Errorf("Failed to parse cron expression '%s': %s",
					line, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return true, nil
}
