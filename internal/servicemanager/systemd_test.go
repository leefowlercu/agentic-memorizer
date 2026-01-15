package servicemanager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateUnitFile(t *testing.T) {
	unit, err := generateUnitFile()
	if err != nil {
		t.Fatalf("generateUnitFile() error = %v", err)
	}

	// Verify unit file contains expected sections
	expectedSections := []string{
		"[Unit]",
		"[Service]",
		"[Install]",
	}

	for _, section := range expectedSections {
		if !strings.Contains(unit, section) {
			t.Errorf("generateUnitFile() missing section: %s", section)
		}
	}

	// Verify unit file contains expected directives
	expectedDirectives := []string{
		"Description=",
		"After=network.target",
		"Type=simple",
		"ExecStart=",
		"daemon start",
		"Restart=on-failure",
		"RestartSec=5",
		"StartLimitBurst=5",
		"StartLimitIntervalSec=60",
		"WantedBy=default.target",
	}

	for _, directive := range expectedDirectives {
		if !strings.Contains(unit, directive) {
			t.Errorf("generateUnitFile() missing directive: %s", directive)
		}
	}
}

func TestGetUnitPath(t *testing.T) {
	path, err := getUnitPath()
	if err != nil {
		t.Fatalf("getUnitPath() error = %v", err)
	}

	if path == "" {
		t.Error("getUnitPath() returned empty string")
	}

	// Should be in .config/systemd/user
	if !strings.Contains(path, ".config/systemd/user") {
		t.Errorf("getUnitPath() = %v, want path containing .config/systemd/user", path)
	}

	// Should end with correct filename
	if !strings.HasSuffix(path, systemdServiceName) {
		t.Errorf("getUnitPath() = %v, want suffix %s", path, systemdServiceName)
	}
}

func TestSystemdManager_Install(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newSystemdManager(mock)
	ctx := context.Background()

	err := manager.Install(ctx)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Verify unit file was created
	unitPath := filepath.Join(tmpDir, ".config", "systemd", "user", systemdServiceName)
	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		t.Error("Install() did not create unit file")
	}

	// Verify systemctl commands were called in order
	expectedCommands := []struct {
		name string
		args []string
	}{
		{"systemctl", []string{"--user", "daemon-reload"}},
		{"systemctl", []string{"--user", "enable", systemdServiceName}},
	}

	if len(mock.commands) != len(expectedCommands) {
		t.Errorf("Install() called %d commands, want %d", len(mock.commands), len(expectedCommands))
	}

	for i, expected := range expectedCommands {
		if i >= len(mock.commands) {
			break
		}
		cmd := mock.commands[i]
		if cmd.name != expected.name {
			t.Errorf("Install() command[%d] = %s, want %s", i, cmd.name, expected.name)
		}
		for j, arg := range expected.args {
			if j >= len(cmd.args) || cmd.args[j] != arg {
				t.Errorf("Install() command[%d].args = %v, want %v", i, cmd.args, expected.args)
				break
			}
		}
	}
}

func TestSystemdManager_Install_EnableError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors: map[string]error{
			"systemctl --user enable " + systemdServiceName: errors.New("enable failed"),
		},
	}

	manager := newSystemdManager(mock)
	ctx := context.Background()

	err := manager.Install(ctx)
	if err == nil {
		t.Error("Install() should return error when enable fails")
	}

	if !strings.Contains(err.Error(), "enable") {
		t.Errorf("Install() error = %v, want error mentioning enable", err)
	}
}

func TestSystemdManager_Uninstall(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the unit file first
	unitDir := filepath.Join(tmpDir, ".config", "systemd", "user")
	os.MkdirAll(unitDir, 0755)
	unitPath := filepath.Join(unitDir, systemdServiceName)
	os.WriteFile(unitPath, []byte("test"), 0644)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newSystemdManager(mock)
	ctx := context.Background()

	err := manager.Uninstall(ctx)
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	// Verify unit file was removed
	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Error("Uninstall() did not remove unit file")
	}

	// Verify systemctl commands were called
	// Should call: stop, disable, daemon-reload
	if len(mock.commands) != 3 {
		t.Errorf("Uninstall() called %d commands, want 3", len(mock.commands))
	}
}

func TestSystemdManager_StartDaemon(t *testing.T) {
	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newSystemdManager(mock)
	ctx := context.Background()

	err := manager.StartDaemon(ctx)
	if err != nil {
		t.Fatalf("StartDaemon() error = %v", err)
	}

	if len(mock.commands) != 1 {
		t.Errorf("StartDaemon() called %d commands, want 1", len(mock.commands))
	}

	if len(mock.commands) > 0 {
		cmd := mock.commands[0]
		if cmd.name != "systemctl" {
			t.Errorf("StartDaemon() called %s, want systemctl", cmd.name)
		}
		expectedArgs := []string{"--user", "start", systemdServiceName}
		for i, arg := range expectedArgs {
			if i >= len(cmd.args) || cmd.args[i] != arg {
				t.Errorf("StartDaemon() args = %v, want %v", cmd.args, expectedArgs)
				break
			}
		}
	}
}

func TestSystemdManager_StopDaemon(t *testing.T) {
	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newSystemdManager(mock)
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
		expectedArgs := []string{"--user", "stop", systemdServiceName}
		for i, arg := range expectedArgs {
			if i >= len(cmd.args) || cmd.args[i] != arg {
				t.Errorf("StopDaemon() args = %v, want %v", cmd.args, expectedArgs)
				break
			}
		}
	}
}

func TestSystemdManager_Restart(t *testing.T) {
	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newSystemdManager(mock)
	ctx := context.Background()

	err := manager.Restart(ctx)
	if err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	// Should call systemctl restart (single command)
	if len(mock.commands) != 1 {
		t.Errorf("Restart() called %d commands, want 1", len(mock.commands))
	}

	if len(mock.commands) > 0 {
		cmd := mock.commands[0]
		expectedArgs := []string{"--user", "restart", systemdServiceName}
		for i, arg := range expectedArgs {
			if i >= len(cmd.args) || cmd.args[i] != arg {
				t.Errorf("Restart() args = %v, want %v", cmd.args, expectedArgs)
				break
			}
		}
	}
}

func TestSystemdManager_IsInstalled(t *testing.T) {
	tests := []struct {
		name       string
		createFile bool
		want       bool
		wantErr    bool
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
				unitDir := filepath.Join(tmpDir, ".config", "systemd", "user")
				os.MkdirAll(unitDir, 0755)
				unitPath := filepath.Join(unitDir, systemdServiceName)
				os.WriteFile(unitPath, []byte("test"), 0644)
			}

			mock := &mockExecutor{}
			manager := newSystemdManager(mock)

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

func TestParseSystemctlOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantState   ServiceState
		wantPID     int
		wantRunning bool
	}{
		{
			name: "running and enabled",
			output: `ActiveState=active
MainPID=12345
UnitFileState=enabled`,
			wantState:   ServiceStateEnabled,
			wantPID:     12345,
			wantRunning: true,
		},
		{
			name: "stopped and enabled",
			output: `ActiveState=inactive
MainPID=0
UnitFileState=enabled`,
			wantState:   ServiceStateEnabled,
			wantPID:     0,
			wantRunning: false,
		},
		{
			name: "stopped and disabled",
			output: `ActiveState=inactive
MainPID=0
UnitFileState=disabled`,
			wantState:   ServiceStateDisabled,
			wantPID:     0,
			wantRunning: false,
		},
		{
			name: "activating",
			output: `ActiveState=activating
MainPID=54321
UnitFileState=enabled`,
			wantState:   ServiceStateEnabled,
			wantPID:     54321,
			wantRunning: true,
		},
		{
			name: "enabled-runtime",
			output: `ActiveState=active
MainPID=99999
UnitFileState=enabled-runtime`,
			wantState:   ServiceStateEnabled,
			wantPID:     99999,
			wantRunning: true,
		},
		{
			name:        "empty output",
			output:      "",
			wantState:   ServiceStateDisabled,
			wantPID:     0,
			wantRunning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotPID, gotRunning := parseSystemctlOutput(tt.output)
			if gotState != tt.wantState {
				t.Errorf("parseSystemctlOutput() state = %v, want %v", gotState, tt.wantState)
			}
			if gotPID != tt.wantPID {
				t.Errorf("parseSystemctlOutput() PID = %v, want %v", gotPID, tt.wantPID)
			}
			if gotRunning != tt.wantRunning {
				t.Errorf("parseSystemctlOutput() running = %v, want %v", gotRunning, tt.wantRunning)
			}
		})
	}
}

func TestSystemdManager_Status_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}

	manager := newSystemdManager(mock)
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

func TestSystemdManager_Status_InstalledRunning(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the unit file
	unitDir := filepath.Join(tmpDir, ".config", "systemd", "user")
	os.MkdirAll(unitDir, 0755)
	unitPath := filepath.Join(unitDir, systemdServiceName)
	os.WriteFile(unitPath, []byte("test"), 0644)

	mock := &mockExecutor{
		outputs: map[string]string{
			"systemctl --user show " + systemdServiceName + " --property=ActiveState,MainPID,UnitFileState": `ActiveState=active
MainPID=12345
UnitFileState=enabled`,
		},
		errors: make(map[string]error),
	}

	manager := newSystemdManager(mock)
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

func TestSystemdManager_Status_InstalledStopped(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the unit file
	unitDir := filepath.Join(tmpDir, ".config", "systemd", "user")
	os.MkdirAll(unitDir, 0755)
	unitPath := filepath.Join(unitDir, systemdServiceName)
	os.WriteFile(unitPath, []byte("test"), 0644)

	mock := &mockExecutor{
		outputs: map[string]string{
			"systemctl --user show " + systemdServiceName + " --property=ActiveState,MainPID,UnitFileState": `ActiveState=inactive
MainPID=0
UnitFileState=enabled`,
		},
		errors: make(map[string]error),
	}

	manager := newSystemdManager(mock)
	ctx := context.Background()

	status, err := manager.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if status.ServiceState != ServiceStateEnabled {
		t.Errorf("Status().ServiceState = %v, want %v", status.ServiceState, ServiceStateEnabled)
	}

	if status.IsRunning {
		t.Error("Status().IsRunning = true, want false")
	}

	if status.PID != 0 {
		t.Errorf("Status().PID = %d, want 0", status.PID)
	}
}
