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

func TestCondition(t *testing.T) {
	var (
		failed    = mockReporter(JobStatusFailed)
		succeeded = mockReporter(JobStatusSucceeded)
		canceled  = mockReporter(JobStatusCanceled)
		skipped   = mockReporter(JobStatusSkipped)
	)

	t.Run("Always", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    bool
		}{
			{
				Name:      "nil => true",
				Reporters: nil,
				Expect:    true,
			},
			{
				Name:      "empty => true",
				Reporters: []Reporter{},
				Expect:    true,
			},
			{
				Name: "all terminated => true",
				Reporters: []Reporter{
					succeeded, failed, canceled, skipped,
				},
				Expect: true,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, Always(c.Reporters))
			})
		}
	})

	t.Run("Succeeded", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    bool
		}{
			{
				Name:      "nil => true",
				Reporters: nil,
				Expect:    true,
			},
			{
				Name:      "empty => true",
				Reporters: []Reporter{},
				Expect:    true,
			},
			{
				Name: "all succeeded => true",
				Reporters: []Reporter{
					succeeded, succeeded, skipped,
				},
				Expect: true,
			},
			{
				Name: "any failed => false",
				Reporters: []Reporter{
					succeeded, failed, skipped,
				},
				Expect: false,
			},
			{
				Name: "any canceled => false",
				Reporters: []Reporter{
					succeeded, canceled, skipped,
				},
				Expect: false,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, Succeeded(c.Reporters))
			})
		}
	})

	t.Run("Failed", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    bool
		}{
			{
				Name:      "nil => false",
				Reporters: nil,
				Expect:    false,
			},
			{
				Name:      "empty => false",
				Reporters: []Reporter{},
				Expect:    false,
			},
			{
				Name: "all succeeded => false",
				Reporters: []Reporter{
					succeeded, succeeded,
				},
				Expect: false,
			},
			{
				Name: "any failed => true",
				Reporters: []Reporter{
					succeeded, failed,
				},
				Expect: true,
			},
			{
				Name: "any canceled => false",
				Reporters: []Reporter{
					succeeded, canceled,
				},
				Expect: false,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, Failed(c.Reporters))
			})
		}
	})

	t.Run("SucceededOrFailed", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    bool
		}{
			{
				Name:      "nil => true",
				Reporters: nil,
				Expect:    true,
			},
			{
				Name:      "empty => true",
				Reporters: []Reporter{},
				Expect:    true,
			},
			{
				Name: "succeeded or failed => true",
				Reporters: []Reporter{
					succeeded, failed,
				},
				Expect: true,
			},
			{
				Name: "any canceled => false",
				Reporters: []Reporter{
					succeeded, canceled,
				},
				Expect: false,
			},
		} {
			c := c
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.Expect, SucceededOrFailed(c.Reporters))
			})
		}
	})

	t.Run("Never", func(t *testing.T) {
		for _, c := range []struct {
			Name      string
			Reporters []Reporter
			Expect    bool
		}{
			{
				Name:      "nil => false",
				Reporters: nil,
				Expect:    false,
			},
			{
				Name:      "empty => false",
				Reporters: []Reporter{},
				Expect:    false,
			},
			{
				Name: "succeeded => false",
				Reporters: []Reporter{
					succeeded,
				},
				Expect: false,
			},
			{
				Name: "failed => false",
				Reporters: []Reporter{
					failed,
				},
				Expect: false,
			},
			{
				Name: "succeeded or failed => false",
				Reporters: []Reporter{
					succeeded, failed,
				},
				Expect: false,
			},
			{
				Name: "any canceled => false",
				Reporters: []Reporter{
					succeeded, canceled,
				},
				Expect: false,
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
