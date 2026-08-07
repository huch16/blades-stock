//go:build !windows

package main

func acquireSingleInstanceLock() bool {
	return true
}

func releaseSingleInstanceLock() {}

func showAlreadyRunningMessage() {}
