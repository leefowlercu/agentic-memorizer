package servicemanager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePlist(t *testing.T) {
	plist, err := generatePlist()
	if err != nil {
		t.Fatalf("generatePlist() error = %v", err)
	}

	// Verify plist contains expected elements
	expectedElements := []string{
		`<key>Label</key>`,
		`<string>com.leefowlercu.memorizer</string>`,
		`<key>ProgramArguments</key>`,
		`<string>daemon</string>`,
		`<string>start</string>`,
		`<key>RunAtLoad</key>`,
		`<true/>`,
		`<key>KeepAlive</key>`,
		`<key>SuccessfulExit</key>`,
		`<false/>`,
		`<key>ThrottleInterval</key>`,
		`<integer>10</integer>`,
	}

	for _, elem := range expectedElements {
		if !strings.Contains(plist, elem) {
			t.Errorf("generatePlist() missing expected element: %s", elem)
		}
	}

	// Verify it's valid XML structure
	if !strings.HasPrefix(plist, `<?xml version="1.0"`) {
		t.Error("generatePlist() missing XML declaration")
	}

	if !strings.Contains(plist, `<!DOCTYPE plist`) {
		t.Error("generatePlist() missing DOCTYPE")
	}
}

func TestGetPlistPath(t *testing.T) {
	path, err := getPlistPath()
	if err != nil {
		t.Fatalf("getPlistPath() error = %v", err)
	}

	if path == "" {
		t.Error("getPlistPath() returned empty string")
	}

	// Should be in Library/LaunchAgents
	if !strings.Contains(path, "Library/LaunchAgents") {
		t.Errorf("getPlistPath() = %v, want path containing Library/LaunchAgents", path)
	}

	// Should end with correct filename
	if !strings.HasSuffix(path, launchdPlistName) {
		t.Errorf("getPlistPath() = %v, want suffix %s", path, launchdPlistName)
	}
}

func TestLaunchdManager_Install(t *testing.T) {
	// Create temp directory to simulate LaunchAgents
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { os.Setenv("HOME", originalHome) }()

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	err := manager.Install(ctx)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Verify plist file was created
	plistPath := filepath.Join(tmpDir, "Library", "LaunchAgents", launchdPlistName)
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		t.Error("Install() did not create plist file")
	}

	// Verify launchctl load was called
	if len(mock.commands) != 1 {
		t.Errorf("Install() called %d commands, want 1", len(mock.commands))
	}

	if len(mock.commands) > 0 {
		cmd := mock.commands[0]
		if cmd.name != "launchctl" {
			t.Errorf("Install() called %s, want launchctl", cmd.name)
		}
		if len(cmd.args) < 3 || cmd.args[0] != "load" || cmd.args[1] != "-w" {
			t.Errorf("Install() args = %v, want [load -w <path>]", cmd.args)
		}
	}
}

func TestLaunchdManager_Install_LaunchctlError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	// Make launchctl fail
	plistPath := filepath.Join(tmpDir, "Library", "LaunchAgents", launchdPlistName)
	mock.errors["launchctl load -w "+plistPath] = errors.New("launchctl failed")

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	err := manager.Install(ctx)
	if err == nil {
		t.Error("Install() should return error when launchctl fails")
	}

	if !strings.Contains(err.Error(), "launchctl") {
		t.Errorf("Install() error = %v, want error mentioning launchctl", err)
	}
}

func TestLaunchdManager_Uninstall(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the plist file first
	launchAgentsDir := filepath.Join(tmpDir, "Library", "LaunchAgents")
	os.MkdirAll(launchAgentsDir, 0755)
	plistPath := filepath.Join(launchAgentsDir, launchdPlistName)
	os.WriteFile(plistPath, []byte("test"), 0644)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	err := manager.Uninstall(ctx)
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	// Verify plist file was removed
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Error("Uninstall() did not remove plist file")
	}

	// Verify launchctl unload was called
	if len(mock.commands) != 1 {
		t.Errorf("Uninstall() called %d commands, want 1", len(mock.commands))
	}

	if len(mock.commands) > 0 {
		cmd := mock.commands[0]
		if cmd.name != "launchctl" || cmd.args[0] != "unload" {
			t.Errorf("Uninstall() command = %s %v, want launchctl unload", cmd.name, cmd.args)
		}
	}
}

func TestLaunchdManager_StartDaemon(t *testing.T) {
	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	err := manager.StartDaemon(ctx)
	if err != nil {
		t.Fatalf("StartDaemon() error = %v", err)
	}

	// Verify launchctl start was called
	if len(mock.commands) != 1 {
		t.Errorf("StartDaemon() called %d commands, want 1", len(mock.commands))
	}

	if len(mock.commands) > 0 {
		cmd := mock.commands[0]
		if cmd.name != "launchctl" {
			t.Errorf("StartDaemon() called %s, want launchctl", cmd.name)
		}
		expectedArgs := []string{"start", launchdServiceLabel}
		if len(cmd.args) != len(expectedArgs) {
			t.Errorf("StartDaemon() args = %v, want %v", cmd.args, expectedArgs)
		}
		for i, arg := range expectedArgs {
			if i < len(cmd.args) && cmd.args[i] != arg {
				t.Errorf("StartDaemon() args[%d] = %s, want %s", i, cmd.args[i], arg)
			}
		}
	}
}

func TestLaunchdManager_StopDaemon(t *testing.T) {
	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	err := manager.StopDaemon(ctx)
	if err != nil {
		t.Fatalf("StopDaemon() error = %v", err)
	}

	if len(mock.commands) != 1 {
		t.Errorf("StopDaemon() called %d commands, want 1", len(mock.commands))
	}

	if len(mock.commands) > 0 {
		cmd := mock.commands[0]
		if cmd.name != "launchctl" {
			t.Errorf("StopDaemon() called %s, want launchctl", cmd.name)
		}
		expectedArgs := []string{"stop", launchdServiceLabel}
		if len(cmd.args) != len(expectedArgs) {
			t.Errorf("StopDaemon() args = %v, want %v", cmd.args, expectedArgs)
		}
	}
}

func TestLaunchdManager_IsInstalled(t *testing.T) {
	tests := []struct {
		name        string
		createFile  bool
		want        bool
		wantErr     bool
	}{
		{
			name:       "not installed",
			createFile: false,
			want:       false,
			wantErr:    false,
		},
		{
			name:       "installed",
			createFile: true,
			want:       true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			if tt.createFile {
				launchAgentsDir := filepath.Join(tmpDir, "Library", "LaunchAgents")
				os.MkdirAll(launchAgentsDir, 0755)
				plistPath := filepath.Join(launchAgentsDir, launchdPlistName)
				os.WriteFile(plistPath, []byte("test"), 0644)
			}

			mock := &mockExecutor{}
			manager := newLaunchdManager(mock)

			got, err := manager.IsInstalled()
			if (err != nil) != tt.wantErr {
				t.Errorf("IsInstalled() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLaunchctlOutput(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantPID   int
		wantAlive bool
	}{
		{
			name:      "running with PID in dict format",
			output:    `"PID" = 12345;`,
			wantPID:   12345,
			wantAlive: true,
		},
		{
			name:      "running with tabular format",
			output:    "12345\t0\tcom.leefowlercu.memorizer",
			wantPID:   12345,
			wantAlive: true,
		},
		{
			name:      "not running tabular format",
			output:    "-\t0\tcom.leefowlercu.memorizer",
			wantPID:   0,
			wantAlive: false,
		},
		{
			name:      "empty output",
			output:    "",
			wantPID:   0,
			wantAlive: false,
		},
		{
			name: "full dict output",
			output: `{
	"PID" = 54321;
	"Label" = "com.leefowlercu.memorizer";
}`,
			wantPID:   54321,
			wantAlive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPID, gotAlive := parseLaunchctlOutput(tt.output)
			if gotPID != tt.wantPID {
				t.Errorf("parseLaunchctlOutput() PID = %v, want %v", gotPID, tt.wantPID)
			}
			if gotAlive != tt.wantAlive {
				t.Errorf("parseLaunchctlOutput() alive = %v, want %v", gotAlive, tt.wantAlive)
			}
		})
	}
}

func TestLaunchdManager_Status_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	status, err := manager.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if status.ServiceState != ServiceStateNotInstalled {
		t.Errorf("Status().ServiceState = %v, want %v", status.ServiceState, ServiceStateNotInstalled)
	}

	if status.IsRunning {
		t.Error("Status().IsRunning = true, want false")
	}
}

func TestLaunchdManager_Status_InstalledRunning(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the plist file
	launchAgentsDir := filepath.Join(tmpDir, "Library", "LaunchAgents")
	os.MkdirAll(launchAgentsDir, 0755)
	plistPath := filepath.Join(launchAgentsDir, launchdPlistName)
	os.WriteFile(plistPath, []byte("test"), 0644)

	mock := &mockExecutor{
		outputs: map[string]string{
			"launchctl list " + launchdServiceLabel: "12345\t0\tcom.leefowlercu.memorizer",
		},
		errors: make(map[string]error),
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	status, err := manager.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if status.ServiceState != ServiceStateEnabled {
		t.Errorf("Status().ServiceState = %v, want %v", status.ServiceState, ServiceStateEnabled)
	}

	if !status.IsRunning {
		t.Error("Status().IsRunning = false, want true")
	}

	if status.PID != 12345 {
		t.Errorf("Status().PID = %d, want 12345", status.PID)
	}
}

func TestLaunchdManager_Status_InstalledNotLoaded(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the plist file
	launchAgentsDir := filepath.Join(tmpDir, "Library", "LaunchAgents")
	os.MkdirAll(launchAgentsDir, 0755)
	plistPath := filepath.Join(launchAgentsDir, launchdPlistName)
	os.WriteFile(plistPath, []byte("test"), 0644)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors: map[string]error{
			"launchctl list " + launchdServiceLabel: errors.New("could not find service"),
		},
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	status, err := manager.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if status.ServiceState != ServiceStateDisabled {
		t.Errorf("Status().ServiceState = %v, want %v", status.ServiceState, ServiceStateDisabled)
	}

	if status.IsRunning {
		t.Error("Status().IsRunning = true, want false")
	}
}

func TestLaunchdManager_Restart(t *testing.T) {
	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newLaunchdManager(mock)
	ctx := context.Background()

	err := manager.Restart(ctx)
	if err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	// Should have called stop then start
	if len(mock.commands) != 2 {
		t.Errorf("Restart() called %d commands, want 2", len(mock.commands))
	}

	if len(mock.commands) >= 2 {
		if mock.commands[0].args[0] != "stop" {
			t.Errorf("Restart() first command = %v, want stop", mock.commands[0].args)
		}
		if mock.commands[1].args[0] != "start" {
			t.Errorf("Restart() second command = %v, want start", mock.commands[1].args)
		}
	}
}
