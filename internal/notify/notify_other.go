//go:build !linux

package notify

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf16"
)

// deliver sends one notification on non-Linux platforms. These shell
// paths cannot attach a click action — delivery only, by platform limit.
func deliver(title, body string, _ func()) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script).Run()
	case "windows":
		return powershellToast(title, body)
	default:
		return exec.Command("notify-send", "-a", appName, title, body).Run()
	}
}

// powershellToast shows a WinRT toast. The script is passed as base64
// UTF-16LE (-EncodedCommand) so quoting never mangles the copy.
func powershellToast(title, body string) error {
	script := strings.Join([]string{
		"[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null",
		"$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)",
		"$t.GetElementsByTagName('text').Item(0).AppendChild($t.CreateTextNode('" + title + "')) > $null",
		"$t.GetElementsByTagName('text').Item(1).AppendChild($t.CreateTextNode('" + body + "')) > $null",
		"[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('" + appName + "').Show([Windows.UI.Notifications.ToastNotification]::new($t))",
	}, "; ")
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePS(script)).Run()
}

// encodePS encodes a script the way -EncodedCommand expects: base64
// over UTF-16LE bytes.
func encodePS(s string) string {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return base64.StdEncoding.EncodeToString(b)
}
