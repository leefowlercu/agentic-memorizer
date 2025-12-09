package servicemanager

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	platform := DetectPlatform()

	switch runtime.GOOS {
	case "linux":
		if platform != PlatformLinux {
			t.Errorf("Expected PlatformLinux on Linux, got %v", platform)
		}
	case "darwin":
		if platform != PlatformDarwin {
			t.Errorf("Expected PlatformDarwin on macOS, got %v", platform)
		}
	default:
		if platform != PlatformUnknown {
			t.Errorf("Expected PlatformUnknown on %s, got %v", runtime.GOOS, platform)
		}
	}
}

func TestIsPlatformSupported(t *testing.T) {
	supported := IsPlatformSupported()

	switch runtime.GOOS {
	case "linux", "darwin":
		if !supported {
			t.Errorf("Expected platform to be supported on %s", runtime.GOOS)
		}
	default:
		if supported {
			t.Errorf("Expected platform to be unsupported on %s", runtime.GOOS)
		}
	}
}

func TestPlatformString(t *testing.T) {
	tests := []struct {
		platform Platform
		expected string
	}{
		{PlatformLinux, "linux"},
		{PlatformDarwin, "darwin"},
		{PlatformUnknown, "unknown"},
	}

	for _, tt := range tests {
		if got := tt.platform.String(); got != tt.expected {
			t.Errorf("Platform.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestGetBinaryPath(t *testing.T) {
	// This test may vary by environment, so we just check that it doesn't error
	// and returns a non-empty path
	path, err := GetBinaryPath()

	// If the binary is found, path should not be empty
	// If not found, err should be non-nil
	if err == nil && path == "" {
		t.Error("GetBinaryPath() returned empty path without error")
	}

	// If path is returned, it should contain "memorizer"
	if err == nil && !strings.Contains(path, "memorizer") {
		t.Errorf("GetBinaryPath() returned unexpected path: %s", path)
	}
}

func TestGenerateUserUnit(t *testing.T) {
	cfg := SystemdConfig{
		BinaryPath: "/usr/local/bin/memorizer",
		User:       "testuser",
		Home:       "/home/testuser",
		LogFile:    "/home/testuser/.memorizer/daemon.log",
	}

	unit := GenerateUserUnit(cfg)

	// Check that key fields are present
	expectedStrings := []string{
		"Description=Agentic Memorizer Daemon",
		"Type=notify",
		"User=testuser",
		"Group=testuser",
		"WorkingDirectory=/home/testuser",
		"ExecStart=/usr/local/bin/memorizer daemon start",
		"NoNewPrivileges=true",
		"PrivateTmp=true",
		`Environment="HOME=/home/testuser"`,
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(unit, expected) {
			t.Errorf("GenerateUserUnit() missing expected string: %s", expected)
		}
	}
}

func TestGenerateSystemUnit(t *testing.T) {
	cfg := SystemdConfig{
		BinaryPath: "/usr/local/bin/memorizer",
		User:       "testuser",
		Home:       "/home/testuser",
		LogFile:    "/home/testuser/.memorizer/daemon.log",
	}

	unit := GenerateSystemUnit(cfg)

	// Check that key fields are present
	expectedStrings := []string{
		"Description=Agentic Memorizer Daemon",
		"Type=notify",
		"WorkingDirectory=/home/testuser",
		"ExecStart=/usr/local/bin/memorizer daemon start",
		"WantedBy=multi-user.target",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(unit, expected) {
			t.Errorf("GenerateSystemUnit() missing expected string: %s", expected)
		}
	}

	// System units should NOT have User/Group fields
	if strings.Contains(unit, "User=") {
		t.Error("GenerateSystemUnit() should not contain User= field")
	}
	if strings.Contains(unit, "Group=") {
		t.Error("GenerateSystemUnit() should not contain Group= field")
	}
}

func TestGetUserUnitPath(t *testing.T) {
	home := "/home/testuser"
	expected := filepath.Join(home, ".config", "systemd", "user", "memorizer.service")

	path, err := GetUserUnitPath(home)
	if err != nil {
		t.Fatalf("GetUserUnitPath() error = %v", err)
	}

	if path != expected {
		t.Errorf("GetUserUnitPath() = %v, want %v", path, expected)
	}
}

func TestGetSystemUnitPath(t *testing.T) {
	expected := "/etc/systemd/system/memorizer.service"
	path := GetSystemUnitPath()

	if path != expected {
		t.Errorf("GetSystemUnitPath() = %v, want %v", path, expected)
	}
}

func TestInstallUserUnit(t *testing.T) {
	// Use temp directory for testing
	tempDir := t.TempDir()
	unitContent := "test unit content"

	err := InstallUserUnit(unitContent, tempDir)
	if err != nil {
		t.Fatalf("InstallUserUnit() error = %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tempDir, ".config", "systemd", "user", "memorizer.service")
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read installed unit file: %v", err)
	}

	if string(content) != unitContent {
		t.Errorf("InstallUserUnit() wrote %v, want %v", string(content), unitContent)
	}

	// Verify permissions
	info, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("Failed to stat unit file: %v", err)
	}

	if info.Mode().Perm() != 0644 {
		t.Errorf("InstallUserUnit() created file with permissions %v, want 0644", info.Mode().Perm())
	}
}

func TestGetSystemdUserInstructions(t *testing.T) {
	unitPath := "/home/testuser/.config/systemd/user/memorizer.service"
	instructions := GetSystemdUserInstructions(unitPath)

	expectedStrings := []string{
		unitPath,
		"systemctl --user daemon-reload",
		"systemctl --user enable memorizer",
		"systemctl --user start memorizer",
		"systemctl --user status memorizer",
		"journalctl --user -u memorizer -f",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(instructions, expected) {
			t.Errorf("GetUserInstructions() missing expected string: %s", expected)
		}
	}
}

func TestGetSystemdSystemInstructions(t *testing.T) {
	unitContent := "test unit content"
	instructions := GetSystemdSystemInstructions(unitContent)

	expectedStrings := []string{
		"sudo",
		"/etc/systemd/system/memorizer.service",
		"systemctl daemon-reload",
		"systemctl enable memorizer",
		"systemctl start memorizer",
		unitContent,
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(instructions, expected) {
			t.Errorf("GetSystemInstructions() missing expected string: %s", expected)
		}
	}
}

func TestGeneratePlist(t *testing.T) {
	cfg := LaunchdConfig{
		BinaryPath: "/usr/local/bin/memorizer",
		User:       "testuser",
		Home:       "/Users/testuser",
		LogFile:    "/Users/testuser/.memorizer/daemon.log",
	}

	plist := GeneratePlist(cfg)

	expectedStrings := []string{
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>",
		"<!DOCTYPE plist",
		"<key>Label</key>",
		"<string>com.testuser.memorizer</string>",
		"<key>ProgramArguments</key>",
		"<string>/usr/local/bin/memorizer</string>",
		"<string>daemon</string>",
		"<string>start</string>",
		"<key>WorkingDirectory</key>",
		"<string>/Users/testuser</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<key>StandardOutPath</key>",
		"<string>/Users/testuser/.memorizer/daemon.log</string>",
		"<key>StandardErrorPath</key>",
		"<key>ThrottleInterval</key>",
		"<integer>30</integer>",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(plist, expected) {
			t.Errorf("GeneratePlist() missing expected string: %s", expected)
		}
	}
}

func TestGetUserAgentPath(t *testing.T) {
	home := "/Users/testuser"
	user := "testuser"
	expected := filepath.Join(home, "Library", "LaunchAgents", "com.testuser.memorizer.plist")

	path, err := GetUserAgentPath(home, user)
	if err != nil {
		t.Fatalf("GetUserAgentPath() error = %v", err)
	}

	if path != expected {
		t.Errorf("GetUserAgentPath() = %v, want %v", path, expected)
	}
}

func TestGetSystemDaemonPath(t *testing.T) {
	user := "testuser"
	expected := "/Library/LaunchDaemons/com.testuser.memorizer.plist"

	path := GetSystemDaemonPath(user)

	if path != expected {
		t.Errorf("GetSystemDaemonPath() = %v, want %v", path, expected)
	}
}

func TestInstallUserAgent(t *testing.T) {
	// Use temp directory for testing
	tempDir := t.TempDir()
	plistContent := "test plist content"
	plistPath := filepath.Join(tempDir, "Library", "LaunchAgents", "com.testuser.memorizer.plist")

	err := InstallUserAgent(plistContent, plistPath)
	if err != nil {
		t.Fatalf("InstallUserAgent() error = %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("Failed to read installed plist file: %v", err)
	}

	if string(content) != plistContent {
		t.Errorf("InstallUserAgent() wrote %v, want %v", string(content), plistContent)
	}

	// Verify permissions
	info, err := os.Stat(plistPath)
	if err != nil {
		t.Fatalf("Failed to stat plist file: %v", err)
	}

	if info.Mode().Perm() != 0644 {
		t.Errorf("InstallUserAgent() created file with permissions %v, want 0644", info.Mode().Perm())
	}
}

func TestGetLaunchdUserInstructions(t *testing.T) {
	plistPath := "/Users/testuser/Library/LaunchAgents/com.testuser.memorizer.plist"
	user := "testuser"
	instructions := GetLaunchdUserInstructions(plistPath, user)

	expectedStrings := []string{
		plistPath,
		"launchctl bootstrap gui/$(id -u)",
		"launchctl enable gui/$(id -u)/com.testuser.memorizer",
		"launchctl kickstart -k gui/$(id -u)/com.testuser.memorizer",
		"launchctl list | grep com.testuser.memorizer",
		"launchctl bootout gui/$(id -u)",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(instructions, expected) {
			t.Errorf("GetUserInstructions() missing expected string: %s", expected)
		}
	}
}

func TestGetLaunchdSystemInstructions(t *testing.T) {
	plistContent := "test plist content"
	plistPath := "/Library/LaunchDaemons/com.testuser.memorizer.plist"
	instructions := GetLaunchdSystemInstructions(plistContent, plistPath)

	expectedStrings := []string{
		"sudo",
		plistPath,
		plistContent,
		"chmod 644",
		"chown root:wheel",
		"launchctl bootstrap system",
		"launchctl enable system",
		"launchctl kickstart -k system",
		"launchctl bootout system",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(instructions, expected) {
			t.Errorf("GetSystemInstructions() missing expected string: %s", expected)
		}
	}
}
