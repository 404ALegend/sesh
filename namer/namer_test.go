package namer

import (
	"fmt"
	"testing"

	"github.com/joshmedeski/sesh/v2/git"
	"github.com/joshmedeski/sesh/v2/home"
	"github.com/joshmedeski/sesh/v2/model"
	"github.com/joshmedeski/sesh/v2/pathwrap"
	"github.com/stretchr/testify/assert"
)

func TestFromPath(t *testing.T) {
	t.Run("when path does not contain a symlink", func(t *testing.T) {
		mockPathwrap := new(pathwrap.MockPath)
		mockGit := new(git.MockGit)
		mockHome := new(home.MockHome)
		config := model.Config{DirLength: 1}
		n := NewNamer(mockPathwrap, mockGit, mockHome, config, "tmux")

		t.Run("name for git repo", func(t *testing.T) {
			path := "/Users/josh/config/dotfiles/.config/neovim"
			list := `worktree /Users/josh/config/dotfiles
HEAD abc123
branch refs/heads/main
`
			mockPathwrap.On("EvalSymlinks", path).Return(path, nil)
			mockGit.On("WorktreeList", path).Return(true, list, nil)
			mockPathwrap.On("Base", "/Users/josh/config/dotfiles").Return("dotfiles")
			name, _ := n.Name(path)
			assert.Equal(t, "dotfiles/_config/neovim", name)
		})

		t.Run("returns base on non-git dir", func(t *testing.T) {
			path := "/Users/josh/.config/neovim"
			mockPathwrap.On("EvalSymlinks", path).Return(path, nil)
			mockGit.On("WorktreeList", path).Return(false, "", fmt.Errorf("not a git repository (or any of the parent"))
			mockPathwrap.On("Base", path).Return("neovim")
			name, _ := n.Name(path)
			assert.Equal(t, "neovim", name)
		})
	})

	t.Run("when path contains a symlink", func(t *testing.T) {
		t.Run("name for symlinked file in symlinked git repo", func(t *testing.T) {
			mockPathwrap := new(pathwrap.MockPath)
			mockGit := new(git.MockGit)
			mockHome := new(home.MockHome)
			config := model.Config{DirLength: 1}
			n := NewNamer(mockPathwrap, mockGit, mockHome, config, "tmux")
			resolved := "/Users/josh/dotfiles/.config/neovim"
			list := `worktree /Users/josh/dotfiles
HEAD abc123
branch refs/heads/main
`
			mockPathwrap.On("EvalSymlinks", "/Users/josh/d/.c/neovim").Return(resolved, nil)
			mockGit.On("WorktreeList", resolved).Return(true, list, nil)
			mockPathwrap.On("Base", "/Users/josh/dotfiles").Return("dotfiles")
			name, _ := n.Name("/Users/josh/d/.c/neovim")
			assert.Equal(t, "dotfiles/_config/neovim", name)
		})

		t.Run("name for git bare repo", func(t *testing.T) {
			mockPathwrap := new(pathwrap.MockPath)
			mockGit := new(git.MockGit)
			mockHome := new(home.MockHome)
			config := model.Config{DirLength: 1}
			n := NewNamer(mockPathwrap, mockGit, mockHome, config, "tmux")
			resolved := "/Users/josh/projects/sesh/main"
			list := `worktree /Users/josh/projects/sesh/.bare
bare

worktree /Users/josh/projects/sesh/main
HEAD ba04ca494
branch refs/heads/main
`
			mockPathwrap.On("EvalSymlinks", "/Users/josh/p/sesh/main").Return(resolved, nil)
			mockGit.On("WorktreeList", resolved).Return(true, list, nil)
			mockPathwrap.On("Base", "/Users/josh/projects/sesh").Return("sesh")
			name, _ := n.Name("/Users/josh/p/sesh/main")
			assert.Equal(t, "sesh/main", name)
		})

		t.Run("returns base on non-git dir", func(t *testing.T) {
			mockPathwrap := new(pathwrap.MockPath)
			mockGit := new(git.MockGit)
			mockHome := new(home.MockHome)
			config := model.Config{DirLength: 1}
			n := NewNamer(mockPathwrap, mockGit, mockHome, config, "tmux")
			resolved := "/Users/josh/.config/neovim"
			mockPathwrap.On("EvalSymlinks", "/Users/josh/c/neovim").Return(resolved, nil)
			mockGit.On("WorktreeList", resolved).Return(false, "", fmt.Errorf("not a git repository"))
			mockPathwrap.On("Base", resolved).Return("neovim")
			name, _ := n.Name("/Users/josh/c/neovim")
			assert.Equal(t, "neovim", name)
		})
	})
}

func TestDirectoryNamesByBackend(t *testing.T) {
	for _, tc := range []struct {
		backend string
		sep     string
	}{
		{"tuios", "%2F"},
		{"tmux", "/"},
		{"herdr", "/"},
	} {
		t.Run(tc.backend, func(t *testing.T) {
			pathwrap := new(pathwrap.MockPath)
			git := new(git.MockGit)
			config := model.Config{DirLength: 2, Multiplexer: "auto"}
			n := NewNamer(pathwrap, git, nil, config, tc.backend)
			for _, parent := range []string{"work", "personal"} {
				path := "/projects/" + parent + "/api"
				pathwrap.On("EvalSymlinks", path).Return(path, nil)
				git.On("WorktreeList", path).Return(false, "", nil)
				want := parent + tc.sep + "api"
				name, err := n.Name(path)
				assert.NoError(t, err)
				assert.Equal(t, want, name)
				root, err := n.RootName(path)
				assert.NoError(t, err)
				assert.Equal(t, want, root)
			}
		})
	}
}
