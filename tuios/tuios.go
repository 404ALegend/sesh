// Package tuios drives TUIOS as a sesh multiplexer backend.
package tuios

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/joshmedeski/sesh/v2/model"
	"github.com/joshmedeski/sesh/v2/mux"
	"github.com/joshmedeski/sesh/v2/oswrap"
	"github.com/joshmedeski/sesh/v2/shell"
)

const Name = "tuios"

type TUIOS struct {
	os    oswrap.Os
	shell shell.Shell
	bin   string
}

var _ mux.Mux = (*TUIOS)(nil)

func NewTUIOS(os oswrap.Os, shell shell.Shell, bin string) *TUIOS {
	if bin == "" {
		bin = Name
	}
	return &TUIOS{os: os, shell: shell, bin: bin}
}

func (t *TUIOS) Name() string { return Name }

func (t *TUIOS) IsAttached() bool {
	return t.os.Getenv("TUIOS_SESSION") != "" || t.os.Getenv("TUIOS_PANE_ID") != ""
}

type session struct {
	Name       string   `json:"name"`
	ID         string   `json:"id"`
	Created    int64    `json:"created"`
	LastActive int64    `json:"last_active"`
	Attached   bool     `json:"attached"`
	Windows    []window `json:"windows"`
}

type window struct {
	ID         string `json:"window_id"`
	Name       string `json:"display_name"`
	CustomName string `json:"custom_name"`
	Title      string `json:"title"`
	CWD        string `json:"cwd"`
	Index      int    `json:"index"`
	Focused    bool   `json:"focused"`
}

type windowsResponse struct {
	Windows []window `json:"windows"`
}

func (t *TUIOS) ListSessions() ([]*model.TmuxSession, error) {
	output, err := t.shell.CmdCapture(t.bin, "ls", "--json")
	var sessions []session
	if decodeErr := json.Unmarshal([]byte(output), &sessions); decodeErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("decode tuios sessions: %w", decodeErr)
	}

	result := make([]*model.TmuxSession, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, s.toModel())
	}
	return result, nil
}

func (s session) toModel() *model.TmuxSession {
	result := &model.TmuxSession{
		Name:     s.Name,
		ID:       s.ID,
		Attached: boolToInt(s.Attached),
		Windows:  len(s.Windows),
	}
	if s.Created != 0 {
		created := time.Unix(s.Created, 0)
		result.Created = &created
	}
	if s.LastActive != 0 {
		lastActive := time.Unix(s.LastActive, 0)
		result.Activity = &lastActive
	}
	for _, w := range s.Windows {
		if w.Focused || result.Path == "" {
			result.Path = w.CWD
		}
	}
	return result
}

func (t *TUIOS) NewSession(sessionName string, startDir string) (string, error) {
	_, err := t.shell.Cmd(t.bin, "new", sessionName, "--cwd", startDir, "--detach")
	return sessionName, err
}

func (t *TUIOS) RenameSession(target string, newName string) (string, error) {
	return t.shell.Cmd(t.bin, "rename-session", newName, target)
}

func (t *TUIOS) SwitchOrAttach(name string, opts model.ConnectOpts) (string, error) {
	if t.IsAttached() || opts.Switch {
		return t.SwitchClient(name)
	}
	return t.shell.CmdWithOutput(t.bin, "attach", name)
}

func (t *TUIOS) SwitchClient(target string) (string, error) {
	return t.shell.Cmd(t.bin, "switch-session", target)
}

func (t *TUIOS) SendKeys(target string, command string) (string, error) {
	windows, err := t.listWindows(target)
	if err != nil {
		return "", err
	}
	if len(windows) == 0 {
		return "", fmt.Errorf("tuios session %q has no windows", target)
	}

	windowID := windows[0].ID
	for _, w := range windows {
		if w.Focused {
			windowID = w.ID
			break
		}
	}
	return t.shell.Cmd(t.bin, "send-text", "--session", target, "--window", windowID, command+"\n")
}

func (t *TUIOS) CapturePane(targetSession string) (string, error) {
	return t.shell.Cmd(t.bin, "capture-pane", "--session", targetSession, "--ansi")
}

func (t *TUIOS) ListWindows(targetSession string) ([]*model.TmuxWindow, error) {
	windows, err := t.listWindows(targetSession)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TmuxWindow, 0, len(windows))
	for _, w := range windows {
		result = append(result, w.toModel())
	}
	return result, nil
}

func (t *TUIOS) ListAllWindowNames(_ string) (map[string][]string, error) {
	sessions, err := t.ListSessions()
	if err != nil {
		return nil, err
	}
	result := make(map[string][]string, len(sessions))
	for _, s := range sessions {
		windows, err := t.listWindows(s.Name)
		if err != nil {
			return nil, err
		}
		for _, w := range windows {
			result[s.Name] = append(result[s.Name], w.name())
		}
	}
	return result, nil
}

func (t *TUIOS) NewWindowInSession(opts model.TmuxWindowOpts) (string, error) {
	args := []string{"new-window"}
	if opts.Name != "" {
		args = append(args, opts.Name)
	}
	args = append(args, "--session", opts.TargetSession)
	if opts.StartDir != "" {
		args = append(args, "--cwd", opts.StartDir)
	}
	if opts.Background {
		args = append(args, "--no-focus")
	}
	args = append(args, "--print-id")
	if opts.Command != "" {
		args = append(args, "--", "/bin/sh", "-lc", opts.Command)
	}
	return t.shell.Cmd(t.bin, args...)
}

func (t *TUIOS) NextWindowInSession(targetSession string) (string, error) {
	return t.shell.Cmd(t.bin, "focus-window", "--session", targetSession, "--relative", "next")
}

func (t *TUIOS) SelectWindow(targetWindow string) (string, error) {
	sessionName, index, hasIndex := strings.Cut(targetWindow, ":")
	if hasIndex {
		windows, err := t.listWindows(sessionName)
		if err != nil {
			return "", err
		}
		for _, w := range windows {
			if fmt.Sprint(w.Index) == index {
				return t.shell.Cmd(t.bin, "focus-window", "--session", sessionName, w.ID)
			}
		}
		return "", fmt.Errorf("tuios window %q not found", targetWindow)
	}
	return t.shell.Cmd(t.bin, "focus-window", targetWindow)
}

func (t *TUIOS) listWindows(sessionName string) ([]window, error) {
	output, err := t.shell.Cmd(t.bin, "list-windows", "--session", sessionName, "--json")
	if err != nil {
		return nil, err
	}
	var response windowsResponse
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return nil, fmt.Errorf("decode tuios windows for %q: %w", sessionName, err)
	}
	return response.Windows, nil
}

func (w window) name() string {
	if w.Name != "" {
		return w.Name
	}
	if w.CustomName != "" {
		return w.CustomName
	}
	return w.Title
}

func (w window) toModel() *model.TmuxWindow {
	return &model.TmuxWindow{Name: w.name(), Path: w.CWD, Index: w.Index, Active: w.Focused}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
