//go:build !windows
// +build !windows

package main

import (
	"log"
	"syscall"
)

func setRLimit() {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Printf("Error getting rlimit: %v", err)
	}
	rLimit.Max = 65535
	rLimit.Cur = 65535
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Printf("Error setting rlimit: %v", err)
	} else {
		log.Printf("System RLimit (NOFILE) set to 65535")
	}
}
