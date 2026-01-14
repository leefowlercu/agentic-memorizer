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

	// signalMu protects signalChan, stopChan, and doneChan
	signalMu sync.Mutex

	// signalChan receives SIGHUP signals
	signalChan chan os.Signal

	// stopChan signals the handler goroutine to stop
	stopChan chan struct{}

	// doneChan is closed when the handler goroutine exits
	doneChan chan struct{}
)

// SetupSignalHandler starts a goroutine that listens for SIGHUP
// and triggers config reload. Concurrent SIGHUP signals are ignored.
// Safe to call multiple times; subsequent calls stop previous handler.
func SetupSignalHandler() {
	signalMu.Lock()
	defer signalMu.Unlock()

	// Stop any existing handler and wait for it to finish
	if stopChan != nil {
		close(stopChan)
		// Wait for the goroutine to finish (unlock mutex temporarily)
		localDone := doneChan
		signalMu.Unlock()
		<-localDone
		signalMu.Lock()
	}

	signalChan = make(chan os.Signal, 1)
	stopChan = make(chan struct{})
	doneChan = make(chan struct{})

	// Capture local copies for the goroutine
	localSignalChan := signalChan
	localStopChan := stopChan
	localDoneChan := doneChan

	signal.Notify(localSignalChan, syscall.SIGHUP)

	go func() {
		defer close(localDoneChan)
		for {
			select {
			case <-localSignalChan:
				// Try to acquire mutex; if already locked, ignore this signal
				if reloadMu.TryLock() {
					slog.Info("received SIGHUP; reloading config")
					_ = Reload() // Error logged internally; retained previous config on failure
					reloadMu.Unlock()
				} else {
					slog.Debug("SIGHUP received during reload; ignoring")
				}
			case <-localStopChan:
				signal.Stop(localSignalChan)
				return
			}
		}
	}()
}

// StopSignalHandler stops the signal handler goroutine and waits for it to exit.
func StopSignalHandler() {
	signalMu.Lock()

	if stopChan == nil {
		signalMu.Unlock()
		return
	}

	close(stopChan)
	stopChan = nil
	localDone := doneChan
	doneChan = nil
	signalMu.Unlock()

	// Wait for goroutine to finish outside of lock
	<-localDone
}
