package crontab

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCronTab(t *testing.T) {
	loaders := map[string]func(*Crontab, string) (bool, bool, error){
		"Parse": func(tab *Crontab, expr string) (bool, bool, error) {
			if strings.ContainsRune(expr, '\n') {
				return false, false, nil
			}

			ok, err := tab.Parse(expr)
			return true, ok, err
		},
		"Load": func(tab *Crontab, expr string) (bool, bool, error) {
			if expr == "" {
				return false, false, nil
			}
			ok, err := tab.Load(strings.NewReader(expr))
			return true, ok, err
		},
	}

	for label, loader := range loaders {
		t.Run(fmt.Sprintf("%s should fail on invalid cron expressions", label), func(t *testing.T) {
			tab := NewCrontab()

			relevant, ok, err := loader(tab, "")
			if relevant {
				if ok {
					t.Fatalf("Parsing an empty line succeeded")
				}

				if err == nil {
					t.Fatalf("Parsing an empty line did not return an error")
				}
			}

			relevant, ok, err = loader(tab, "not a valid cron expression")
			if relevant {
				if ok {
					t.Fatalf("Parsing an invalid line succeeded")
				}

				if err == nil {
					t.Fatalf("Parsing an invalid line did not return an error")
				}
			}

			relevant, ok, err = loader(tab, "* * * * x Example")
			if relevant {
				if ok {
					t.Fatalf("Parsing a barely-invalid line succeeded")
				}

				if err == nil {
					t.Fatalf("Parsing a barely-invalid line did not return an error")
				}
			}

			relevant, ok, err = loader(tab, "* * * * *")
			if relevant {
				if ok {
					t.Fatalf("Parsing without a task succeeded")
				}

				if err == nil {
					t.Fatalf("Parsing without a task did not return an error")
				}
			}
		})

		t.Run(fmt.Sprintf("%s valid expr should add to the schedule", label), func(t *testing.T) {
			for _, expr := range []string{
				"* * * * * Example",
				"#ignored line\n* * * * * Example\n",
			} {
				tab := NewCrontab()

				relevant, ok, err := loader(tab, expr)
				if relevant {
					if !ok {
						t.Fatalf("Parsing a valid line did not succeed: %s", err)
					}

					expected := time.Date(2006, 1, 2, 15, 4, 0, 0, time.UTC)
					testAfter := expected.Add(time.Duration(-1))
					next := tab.Next(testAfter)

					if !next.Equal(expected) {
						t.Fatalf("Parsing a valid line did not add to the schedule: %v -> %v", testAfter, next)
					}

					tab.Clear("Example")
					next = tab.Next(testAfter)
					if !next.IsZero() {
						t.Fatalf("Clearing did not reset the schedule")
					}
				}
			}
		})
	}
}
