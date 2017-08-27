package schedule

import (
	"bytes"
	"testing"
	"time"
)

func TestBasicDump(t *testing.T) {
	t.Run("Dump should return channel of tasks which have been Set", func(t *testing.T) {
		schedule := NewBasicSchedule()

		testAdd := time.Date(2006, 1, 2, 15, 4, 5, 999, time.UTC)
		schedule.Set("test", NextTime(testAdd))

		testAfter := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
		testUntil := time.Date(2006, 1, 2, 15, 4, 6, 0, time.UTC)

		dump := Dump(schedule, testAfter, testUntil)
		next := <-dump

		if !next.After.Equal(testAdd) {
			t.Fatalf("Dump did not return the added Nexter when it was " +
				"the only Nexter in the list")
		}

		if len(next.Tasks) != 1 {
			t.Fatalf("Dump did not return exactly one task when exactly one task was scheduled")
		}

		if next.Tasks[0] != "test" {
			t.Fatalf("Dump did not return the expected task")
		}
	})

	t.Run("Dump should continue up to the until time", func(t *testing.T) {
		schedule := NewBasicSchedule()
		schedule.Set("test", NextFunc(func(after time.Time) time.Time {
			return after.Truncate(time.Minute).Add(time.Minute).Truncate(time.Minute)
		}))

		testAfter := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
		testUntil := time.Date(2006, 1, 2, 15, 7, 0, 0, time.UTC)
		dump := Dump(schedule, testAfter, testUntil)

		for _, minute := range []int{1, 2, 3} {
			event, ok := <-dump
			if !ok {
				t.Fatalf("Dump ended early")
			}

			if !event.After.Equal(testAfter.Add(time.Duration(minute) * time.Minute).Truncate(time.Minute)) {
				t.Fatalf("Dump did not return the expected event: %v vs %v",
					event.After, testAfter.Add(time.Duration(minute)*time.Minute))
			}

			if len(event.Tasks) != 1 {
				t.Fatalf("Dump did not return exactly one task when exactly one task was scheduled")
			}

			if event.Tasks[0] != "test" {
				t.Fatalf("Dump did not return the expected task")
			}
		}

		if _, ok := <-dump; ok {
			t.Fatalf("Dump continued beyond the 'until' time")
		}
	})

	t.Run("Dump should return multiple tasks when more than one is scheduled", func(t *testing.T) {
		schedule := NewBasicSchedule()
		schedule.Set("testA", NextFunc(func(after time.Time) time.Time {
			return after.Truncate(time.Minute).Add(time.Minute).Truncate(time.Minute)
		}))

		schedule.Set("testB", NextFunc(func(after time.Time) time.Time {
			if after.Before(time.Date(2006, 1, 2, 15, 6, 0, 0, time.UTC)) {
				return time.Date(2006, 1, 2, 15, 6, 0, 0, time.UTC)
			}

			if after.After(time.Date(2006, 1, 2, 15, 7, 0, 0, time.UTC)) {
				return time.Time{}
			}

			return after.Truncate(time.Minute).Add(time.Minute).Truncate(time.Minute)
		}))

		testAfter := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
		testUntil := time.Date(2006, 1, 2, 15, 8, 0, 0, time.UTC)
		dump := Dump(schedule, testAfter, testUntil)

		expected := [][]string{
			[]string{"testA"},
			[]string{"testA", "testB"},
			[]string{"testA", "testB"},
			[]string{"testA"},
		}
		for offset, tasks := range expected {
			minute := offset + 1
			event, ok := <-dump
			if !ok {
				t.Fatalf("Dump ended early")
			}

			if !event.After.Equal(testAfter.Add(time.Duration(minute) * time.Minute).Truncate(time.Minute)) {
				t.Fatalf("Dump did not return the expected event: %v vs %v",
					event.After, testAfter.Add(time.Duration(minute)*time.Minute))
			}

			if len(event.Tasks) != len(tasks) {
				t.Fatalf("Dump did not return the expected number of tasks %v %v", offset, event)
			}

			for _, expectedTask := range tasks {
				found := false
				for _, returnedTask := range event.Tasks {
					if returnedTask == expectedTask {
						found = true
						break
					}
				}

				if !found {
					t.Fatalf("Dump did not return one of the expected tasks %v", offset)
				}
			}
		}

		if _, ok := <-dump; ok {
			t.Fatalf("Dump continued beyond the 'until' time")
		}
	})

	t.Run("Empty Schedule should return a channel which immediately closes", func(t *testing.T) {
		schedule := NewBasicSchedule()
		testAfter := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
		testUntil := time.Date(2006, 1, 2, 15, 8, 0, 0, time.UTC)
		dump := Dump(schedule, testAfter, testUntil)
		if _, ok := <-dump; ok {
			t.Fatalf("Dump of empty schedule returned an event")
		}
	})
}

func TestJsonDump(t *testing.T) {
	t.Run("Multi-Task Schedule", func(t *testing.T) {
		schedule := NewBasicSchedule()
		schedule.Set("testA", NextFunc(func(after time.Time) time.Time {
			if after.After(time.Date(2006, 1, 2, 15, 3, 0, 0, time.UTC)) &&
				after.Before(time.Date(2006, 1, 2, 15, 7, 0, 0, time.UTC)) {
				return time.Date(2006, 1, 2, 15, 7, 0, 0, time.UTC)
			}

			return after.Truncate(time.Minute).Add(time.Minute).Truncate(time.Minute)
		}))

		schedule.Set("testB", NextFunc(func(after time.Time) time.Time {
			if after.Before(time.Date(2006, 1, 2, 15, 6, 0, 0, time.UTC)) {
				return time.Date(2006, 1, 2, 15, 6, 0, 0, time.UTC)
			}

			if after.After(time.Date(2006, 1, 2, 15, 7, 0, 0, time.UTC)) {
				return time.Time{}
			}

			return after.Truncate(time.Minute).Add(time.Minute).Truncate(time.Minute)
		}))

		testAfter := time.Date(2006, 1, 2, 15, 2, 0, 0, time.UTC)
		testUntil := time.Date(2006, 1, 2, 15, 8, 0, 0, time.UTC)

		buf := new(bytes.Buffer)
		i, err := DumpJson(buf, schedule, testAfter, testUntil)
		if err != nil {
			t.Fatalf("unexpected error while writing JSON: %v", err)
		}

		expected := "[{\"when\":\"2006-01-02 15:03:00\",\"tasks\":[\"testA\"]},"
		expected += "{\"when\":\"2006-01-02 15:06:00\",\"tasks\":[\"testB\"]},"
		expected += "{\"when\":\"2006-01-02 15:07:00\",\"tasks\":[\"testA\",\"testB\"]},"
		expected += "{\"when\":\"2006-01-02 15:08:00\",\"tasks\":[\"testA\"]}]"

		if buf.String() != expected {
			t.Fatalf("JSON did not match expected JSON")
		}

		if i != len(expected) {
			t.Fatalf("DumpJson reported inaccurate byte-count")
		}
	})

	t.Run("Empty Schedule should return an empty array", func(t *testing.T) {
		schedule := NewBasicSchedule()
		buf := new(bytes.Buffer)
		testAfter := time.Date(2006, 1, 2, 15, 2, 0, 0, time.UTC)
		testUntil := time.Date(2006, 1, 2, 15, 8, 0, 0, time.UTC)
		i, err := DumpJson(buf, schedule, testAfter, testUntil)
		if err != nil {
			t.Fatalf("unexpected error while writing JSON: %v", err)
		}

		if buf.String() != "[]" {
			t.Fatalf("JSON was not an empty array")
		}

		if i != len("[]") {
			t.Fatalf("DumpJson reported inaccurate byte-count")
		}
	})
}
