//go:build !windows

package main

func pipeJobReady() bool { return false }
