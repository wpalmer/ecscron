package schedule

import (
  "encoding/json"
  "fmt"
  "io"
  "time"

  "github.com/wpalmer/ecscron/taskrunner"
)

type DumpEntry struct {
  After time.Time
  Tasks []string
}

func Dump(schedule Schedule, after time.Time, until time.Time) (chan DumpEntry) {
  var entry DumpEntry
  dump := make(chan DumpEntry)
  dumpRunner := taskrunner.TaskRunnerFunc(func(task string) (*taskrunner.TaskStatus, error) {
    entry.Tasks = append(entry.Tasks, task)
    return &taskrunner.TaskStatus{}, nil
  })

  go func(){
    defer func(){ close(dump); }()
    next := schedule.Next(after);

    for {
      if next.IsZero() || next.After(until) {
        break
      }

      entry = DumpEntry{ After: next }
      _, _ = schedule.Tick(dumpRunner, next)
      if len(entry.Tasks) > 0 {
        dump <- entry
      }

      next = schedule.Next(next)
    }
  }()

  return dump
}

func DumpJson(writer io.Writer, schedule Schedule, after time.Time, until time.Time) (int, error) {
  var written int
  var writtenPart int
  var err error
  var whenJson []byte
  var tasksJson []byte

  if written, err = writer.Write([]byte("[")); err != nil {
    return written, err
  }

  channel := Dump(schedule, after, until)

  glue := ""
  for entry := range channel {
    whenJson, err = json.Marshal(entry.After.UTC().Format("2006-01-02 15:04:05"))
    if err != nil {
      return written, err
    }

    tasksJson, err = json.Marshal(entry.Tasks)
    if err != nil {
      return written, err
    }

    writtenPart, err = fmt.Fprintf(writer, "%s{\"when\":%s,\"tasks\":%s}",
      glue,
      whenJson,
      tasksJson)

    written = written + writtenPart
    if err != nil {
      return written, err
    }

    glue = ","
  }

  writtenPart, err = writer.Write([]byte("]\n"))
  written = written + writtenPart
  if err != nil {
    return written, err
  }

  return written, nil
}
