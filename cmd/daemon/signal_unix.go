//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// notifyPairingSignal relays SIGUSR1 (the operator's "open the pairing
// window" signal) onto ch.
func notifyPairingSignal(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGUSR1)
}

// sendPairingSignal sends SIGUSR1 to pid, used by the `pair` subcommand.
func sendPairingSignal(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		return fmt.Errorf("send SIGUSR1 to pid %d: %w", pid, err)
	}
	return nil
}
