package daemon

import (
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler sets up signal handlers for graceful shutdown and rebuild
func setupSignalHandler(d *Daemon) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGHUP)

	go func() {
		for {
			sig := <-sigCh
			d.GetLogger().Info("received signal", "signal", sig.String())

			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				d.Stop()
				return
			case syscall.SIGUSR1:
				// Trigger rebuild
				go func() {
					if err := d.Rebuild(); err != nil {
						d.GetLogger().Error("manual rebuild failed", "error", err)
					}
				}()
			case syscall.SIGHUP:
				// Reload configuration
				go func() {
					if err := d.ReloadConfig(); err != nil {
						d.GetLogger().Error("config reload failed", "error", err)
					} else {
						d.GetLogger().Info("config reloaded successfully")
					}
				}()
			}
		}
	}()
}
