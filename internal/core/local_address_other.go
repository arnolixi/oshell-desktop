//go:build !darwin

package core

func primaryLocalInterfaces() map[string]bool { return nil }
