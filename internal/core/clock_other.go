//go:build !darwin

package core

import "time"

func platformClockText(now time.Time) string { return now.Local().Format("2006-01-02 15:04:05") }
