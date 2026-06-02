//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.cliplink.plist")
}

func isAutostartEnabled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func enableAutostart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path := plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := strings.ReplaceAll(plistTemplate, "CLIPLINK_BINARY_PATH", exe)
	return os.WriteFile(path, []byte(content), 0644)
}

func disableAutostart() error {
	return os.Remove(plistPath())
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.cliplink</string>
    <key>ProgramArguments</key>
    <array>
        <string>CLIPLINK_BINARY_PATH</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/cliplink.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/cliplink.log</string>
</dict>
</plist>
`

func hideDockIcon() {} // systray handles activation policy internally

func autostartErrorFmt(err error) string {
	return fmt.Sprintf("自启失败: %v", err)
}
