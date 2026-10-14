//go:build !darwin

package core

func platformSyncConflictCheck(string) error                 { return nil }
func coordinateSyncWrite(_ string, write func() error) error { return write() }
