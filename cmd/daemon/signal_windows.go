//go:build windows

package main

import (
	"fmt"
	"os"
)

// homectl-daemon is Linux-only (per OBJECTIF.md); these stubs exist only so
// the module still builds on a Windows development machine. SIGUSR1 has no
// Windows equivalent, so the pairing window can never be opened this way
// here.
func notifyPairingSignal(ch chan<- os.Signal) {}

func sendPairingSignal(pid int) error {
	return fmt.Errorf("pairing signal is not supported on Windows; homectl-daemon is Linux-only")
}
