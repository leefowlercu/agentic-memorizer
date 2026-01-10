package config

import (
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	// reloadMu prevents concurrent reload attempts
	reloadMu sync.Mutex

	// signalChan receives SIGHUP signals
	signalChan chan os.Signal

	// stopChan signals the handler goroutine to stop
	stopChan chan struct{}
)

// SetupSignalHandler starts a goroutine that listens for SIGHUP
// and triggers config reload. Concurrent SIGHUP signals are ignored.
func SetupSignalHandler() {
	signalChan = make(chan os.Signal, 1)
	stopChan = make(chan struct{})

	signal.Notify(signalChan, syscall.SIGHUP)

	go func() {
		for {
			select {
			case <-signalChan:
				// Try to acquire mutex; if already locked, ignore this signal
				if reloadMu.TryLock() {
					slog.Info("received SIGHUP; reloading config")
					_ = Reload() // Error logged internally; retained previous config on failure
					reloadMu.Unlock()
				} else {
					slog.Debug("SIGHUP received during reload; ignoring")
				}
			case <-stopChan:
				signal.Stop(signalChan)
				return
			}
		}
	}()
}

// StopSignalHandler stops the signal handler goroutine.
func StopSignalHandler() {
	if stopChan != nil {
		close(stopChan)
	}
}
