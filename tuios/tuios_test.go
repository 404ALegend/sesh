package tuios

import (
	"testing"

	"github.com/joshmedeski/sesh/v2/model"
	"github.com/joshmedeski/sesh/v2/oswrap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testOS struct {
	*oswrap.RealOs
	env map[string]string
}

func (o testOS) Getenv(key string) string { return o.env[key] }

type testShell struct {
	cmd           func(string, ...string) (string, error)
	cmdCapture    func(string, ...string) (string, error)
	cmdWithOutput func(string, ...string) (string, error)
}

func (s testShell) Cmd(cmd string, args ...string) (string, error) {
	return s.cmd(cmd, args...)
}

func (s testShell) CmdInDir(string, string, ...string) (string, error) { return "", nil }

func (s testShell) CmdWithOutput(cmd string, args ...string) (string, error) {
	return s.cmdWithOutput(cmd, args...)
}

func (s testShell) CmdCapture(cmd string, args ...string) (string, error) {
	return s.cmdCapture(cmd, args...)
}

func (s testShell) ListCmd(string, ...string) ([]string, error) { return nil, nil }

func (s testShell) PrepareCmd(string, map[string]string) ([]string, error) { return nil, nil }

func (s testShell) ShellCmd(string, map[string]string) (string, error) { return "", nil }

func TestListSessionsUsesJSON(t *testing.T) {
	tuios := NewTUIOS(testOS{}, testShell{
		cmdCapture: func(cmd string, args ...string) (string, error) {
			assert.Equal(t, "tuios", cmd)
			assert.Equal(t, []string{"ls", "--json"}, args)
			return `[
				{"name":"work","id":"s1","created":10,"last_active":20,"attached":true,"windows":[
					{"window_id":"w1","cwd":"/repo","focused":true},
					{"window_id":"w2","cwd":"/other"}
				]}
			]`, nil
		},
	}, "")

	sessions, err := tuios.ListSessions()

	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "work", sessions[0].Name)
	assert.Equal(t, "s1", sessions[0].ID)
	assert.Equal(t, "/repo", sessions[0].Path)
	assert.Equal(t, 1, sessions[0].Attached)
	assert.Equal(t, 2, sessions[0].Windows)
	assert.Equal(t, int64(10), sessions[0].Created.Unix())
	assert.Equal(t, int64(20), sessions[0].Activity.Unix())
}

func TestListSessionsAcceptsSavedSessionsWhenDaemonIsStopped(t *testing.T) {
	tuios := NewTUIOS(testOS{}, testShell{
		cmdCapture: func(string, ...string) (string, error) {
			return `[{"name":"saved"}]`, assert.AnError
		},
	}, "")

	sessions, err := tuios.ListSessions()

	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "saved", sessions[0].Name)
}

func TestNewSessionStartsDaemonHeadlesslyAtRequestedDirectory(t *testing.T) {
	tuios := NewTUIOS(testOS{}, testShell{
		cmd: func(cmd string, args ...string) (string, error) {
			assert.Equal(t, "tuios", cmd)
			assert.Equal(t, []string{"new", "project space", "--cwd", "/repo with spaces", "--detach"}, args)
			return "", nil
		},
	}, "")

	name, err := tuios.NewSession("project space", "/repo with spaces")

	require.NoError(t, err)
	assert.Equal(t, "project space", name)
}

func TestSwitchOrAttachAttachesOutsideTUIOS(t *testing.T) {
	tuios := NewTUIOS(testOS{}, testShell{
		cmdWithOutput: func(cmd string, args ...string) (string, error) {
			assert.Equal(t, "tuios", cmd)
			assert.Equal(t, []string{"attach", "work"}, args)
			return "", nil
		},
	}, "")

	_, err := tuios.SwitchOrAttach("work", model.ConnectOpts{})

	require.NoError(t, err)
}

func TestSwitchOrAttachSwitchesInsideTUIOS(t *testing.T) {
	tuios := NewTUIOS(testOS{env: map[string]string{"TUIOS_SESSION": "current"}}, testShell{
		cmd: func(cmd string, args ...string) (string, error) {
			assert.Equal(t, "tuios", cmd)
			assert.Equal(t, []string{"switch-session", "work"}, args)
			return "", nil
		},
	}, "")

	_, err := tuios.SwitchOrAttach("work", model.ConnectOpts{})

	require.NoError(t, err)
}

func TestSendKeysTargetsTheFocusedWindow(t *testing.T) {
	tuios := NewTUIOS(testOS{}, testShell{
		cmd: func(cmd string, args ...string) (string, error) {
			switch args[0] {
			case "list-windows":
				assert.Equal(t, "tuios", cmd)
				assert.Equal(t, []string{"list-windows", "--session", "work", "--json"}, args)
				return `{"windows":[
					{"window_id":"first","display_name":"shell"},
					{"window_id":"focused","display_name":"editor","focused":true}
				]}`, nil
			default:
				assert.Equal(t, "tuios", cmd)
				assert.Equal(t, []string{"send-text", "--session", "work", "--window", "focused", "nvim\n"}, args)
				return "", nil
			}
		},
	}, "")

	_, err := tuios.SendKeys("work", "nvim")

	require.NoError(t, err)
}

func TestCapturePanePreservesANSI(t *testing.T) {
	tuios := NewTUIOS(testOS{}, testShell{
		cmd: func(cmd string, args ...string) (string, error) {
			assert.Equal(t, "tuios", cmd)
			assert.Equal(t, []string{"capture-pane", "--session", "work", "--ansi"}, args)
			return "\x1b[31merror", nil
		},
	}, "")

	output, err := tuios.CapturePane("work")

	require.NoError(t, err)
	assert.Equal(t, "\x1b[31merror", output)
}

func TestNewWindowPassesCommandAsSeparateArguments(t *testing.T) {
	tuios := NewTUIOS(testOS{}, testShell{
		cmd: func(cmd string, args ...string) (string, error) {
			assert.Equal(t, "tuios", cmd)
			assert.Equal(t, []string{"new-window", "build", "--session", "work", "--cwd", "/repo", "--no-focus", "--print-id", "--", "/bin/sh", "-lc", "go test ./..."}, args)
			return "window-id", nil
		},
	}, "")

	id, err := tuios.NewWindowInSession(model.TmuxWindowOpts{
		Name:          "build",
		TargetSession: "work",
		StartDir:      "/repo",
		Background:    true,
		Command:       "go test ./...",
	})

	require.NoError(t, err)
	assert.Equal(t, "window-id", id)
}
