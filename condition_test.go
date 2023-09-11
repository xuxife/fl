package pl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockReporter JobStatus

func (r mockReporter) String() string {
	return string(r)
}

func (r mockReporter) GetStatus() JobStatus {
	return JobStatus(r)
}

func (r mockReporter) GetCondition() Condition {
	return DefaultCondition
}

func (r mockReporter) GetRetry() RetryOption {
	return DefaultRetryOption
}

func (r mockReporter) GetWhen() WhenFunc {
	return DefaultWhenFunc
}

func TestCond(t *testing.T) {
	var (
		pending   = mockReporter(JobStatusPending)
		running   = mockReporter(JobStatusRunning)
		failed    = mockReporter(JobStatusFailed)
		succeeded = mockReporter(JobStatusSucceeded)
		canceled  = mockReporter(JobStatusCanceled)
	)

	t.Run("CondAlways", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    JobStatus
		}{
			{
				Name:      "nil => Running",
				Reporters: nil,
				Expect:    JobStatusRunning,
			},
			{
				Name:      "empty => Running",
				Reporters: []Reporter{},
				Expect:    JobStatusRunning,
			},
			{
				Name: "one Pending => Pending",
				Reporters: []Reporter{
					pending, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "one Running => Pending",
				Reporters: []Reporter{
					running, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "all terminated => Running",
				Reporters: []Reporter{
					succeeded, failed, canceled,
				},
				Expect: JobStatusRunning,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, Always(c.Reporters))
			})
		}
	})

	t.Run("CondSucceeded", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    JobStatus
		}{
			{
				Name:      "nil => Running",
				Reporters: nil,
				Expect:    JobStatusRunning,
			},
			{
				Name:      "empty => Running",
				Reporters: []Reporter{},
				Expect:    JobStatusRunning,
			},
			{
				Name: "one Pending => Pending",
				Reporters: []Reporter{
					pending, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "one Running => Pending",
				Reporters: []Reporter{
					running, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "all succeeded => Running",
				Reporters: []Reporter{
					succeeded, succeeded,
				},
				Expect: JobStatusRunning,
			},
			{
				Name: "any failed => Canceled",
				Reporters: []Reporter{
					succeeded, failed,
				},
				Expect: JobStatusCanceled,
			},
			{
				Name: "any canceled => Canceled",
				Reporters: []Reporter{
					succeeded, canceled,
				},
				Expect: JobStatusCanceled,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, Succeeded(c.Reporters))
			})
		}
	})

	t.Run("CondFailed", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    JobStatus
		}{
			{
				Name:      "nil => Canceled",
				Reporters: nil,
				Expect:    JobStatusCanceled,
			},
			{
				Name:      "empty => Canceled",
				Reporters: []Reporter{},
				Expect:    JobStatusCanceled,
			},
			{
				Name: "one Pending => Pending",
				Reporters: []Reporter{
					pending, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "one Running => Pending",
				Reporters: []Reporter{
					running, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "all succeeded => Canceled",
				Reporters: []Reporter{
					succeeded, succeeded,
				},
				Expect: JobStatusCanceled,
			},
			{
				Name: "any failed => Running",
				Reporters: []Reporter{
					succeeded, failed,
				},
				Expect: JobStatusRunning,
			},
			{
				Name: "any canceled => Canceled",
				Reporters: []Reporter{
					succeeded, canceled,
				},
				Expect: JobStatusCanceled,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, Failed(c.Reporters))
			})
		}
	})

	t.Run("CondSucceededOrFailed", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    JobStatus
		}{
			{
				Name:      "nil => Running",
				Reporters: nil,
				Expect:    JobStatusRunning,
			},
			{
				Name:      "empty => Running",
				Reporters: []Reporter{},
				Expect:    JobStatusRunning,
			},
			{
				Name: "one Pending => Pending",
				Reporters: []Reporter{
					pending, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "one Running => Pending",
				Reporters: []Reporter{
					running, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "succeeded or failed => Running",
				Reporters: []Reporter{
					succeeded, failed,
				},
				Expect: JobStatusRunning,
			},
			{
				Name: "any canceled => Canceled",
				Reporters: []Reporter{
					succeeded, canceled,
				},
				Expect: JobStatusCanceled,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, SucceededOrFailed(c.Reporters))
			})
		}
	})

	t.Run("CondNever", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    JobStatus
		}{
			{
				Name:      "nil => Canceled",
				Reporters: nil,
				Expect:    JobStatusCanceled,
			},
			{
				Name:      "empty => Canceled",
				Reporters: []Reporter{},
				Expect:    JobStatusCanceled,
			},
			{
				Name: "one Pending => Pending",
				Reporters: []Reporter{
					pending, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "one Running => Pending",
				Reporters: []Reporter{
					running, succeeded, failed, canceled,
				},
				Expect: JobStatusPending,
			},
			{
				Name: "succeeded => Canceled",
				Reporters: []Reporter{
					succeeded,
				},
				Expect: JobStatusCanceled,
			},
			{
				Name: "failed => Canceled",
				Reporters: []Reporter{
					failed,
				},
				Expect: JobStatusCanceled,
			},
			{
				Name: "succeeded or failed => Canceled",
				Reporters: []Reporter{
					succeeded, failed,
				},
				Expect: JobStatusCanceled,
			},
			{
				Name: "any canceled => Canceled",
				Reporters: []Reporter{
					succeeded, canceled,
				},
				Expect: JobStatusCanceled,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, Never(c.Reporters))
			})
		}
	})
}
