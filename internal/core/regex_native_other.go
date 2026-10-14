//go:build !darwin

package core

import "time"

func compileNativePattern(string, time.Duration) (nativeMatcher, bool, error) { return nil, false, nil }
