package daemon

import (
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler sets up signal handlers for graceful shutdown and rebuild
func setupSignalHandler(d *Daemon) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		for {
			sig := <-sigCh
			d.logger.Info("received signal", "signal", sig.String())

			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				d.Stop()
				return
			case syscall.SIGUSR1:
				// Trigger rebuild
				go func() {
					if err := d.Rebuild(); err != nil {
						d.logger.Error("manual rebuild failed", "error", err)
					}
				}()
			}
		}
	}()
}
