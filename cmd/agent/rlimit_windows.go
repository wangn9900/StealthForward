//go:build windows
// +build windows

package main

func setRLimit() {
	// Windows does not support syscall.Setrlimit
}
