package notion

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToDoScheduler_parseSchedule(t *testing.T) {
	cases := []struct {
		In  string
		Out *scheduleEvent
	}{
		{In: "every Monday", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Monday}},
		{In: "every Tuesday", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Tuesday}},
		{In: "every Wednesday", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Wednesday}},
		{In: "every Thursday", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Thursday}},
		{In: "every Friday", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Friday}},
		{In: "every Saturday", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Saturday}},
		{In: "every Sunday", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Sunday}},
		{In: "every Sunday at 1:00 pm", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Sunday, Hour: 13}},
		{In: "every Sunday at 1:45 pm", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Sunday, Hour: 13, Minute: 45}},
		{In: "every Sunday at 1:00 am", Out: &scheduleEvent{Interval: intervalWeekly, Weekday: time.Sunday, Hour: 1}},
		{In: "25th of every month", Out: &scheduleEvent{Interval: intervalMonthly, Day: 25}},
		{In: "1st of every month", Out: &scheduleEvent{Interval: intervalMonthly, Day: 1}},
		{In: "2nd of every month", Out: &scheduleEvent{Interval: intervalMonthly, Day: 2}},
		{In: "3rd of every month", Out: &scheduleEvent{Interval: intervalMonthly, Day: 3}},
		{In: "13th of every month", Out: &scheduleEvent{Interval: intervalMonthly, Day: 13}},
		{In: "32nd of every month"},
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			s := &ToDoScheduler{}

			e, err := s.parseSchedule(tc.In)
			if tc.Out != nil {
				require.NoError(t, err)
				assert.Equal(t, tc.Out, e)
			} else {
				require.Error(t, err)
			}
		})
	}
}
