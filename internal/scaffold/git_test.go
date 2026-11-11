package scaffold

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateInitializesIndependentGitRepository(t *testing.T) {
	useTestGitConfig(t, "")
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY"} {
		redirect := t.TempDir()
		t.Setenv(key, redirect)
		t.Cleanup(func() {
			entries, err := os.ReadDir(redirect)
			if err != nil {
				t.Error(err)
			} else if len(entries) != 0 {
				t.Errorf("Git wrote outside the project through %s: %v", key, entries)
			}
		})
	}

	result, err := CreateNamed(NamedOptions{
		Publisher: "acme", Kind: "theme", ShortName: "git-example",
		Version: "1.0.0", OutputDirectory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(result.Directory, ".git")
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(head)); got != "ref: refs/heads/main" {
		t.Errorf("HEAD = %q, want refs/heads/main", got)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "refs", "heads", "main")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected main to have no initial commit, got %v", err)
	}
	for _, dir := range []string{"objects", "refs"} {
		info, err := os.Stat(filepath.Join(gitDir, dir))
		if err != nil {
			t.Error(err)
		} else if !info.IsDir() {
			t.Errorf(".git/%s must be a directory", dir)
		}
	}
}

func TestCreateCleansUpIfGitInitFails(t *testing.T) {
	useTestGitConfig(t, "[malformed\n")
	parent := t.TempDir()
	if _, err := Create("theme", filepath.Join(parent, "theme-example"), "", nil); err == nil || !strings.Contains(err.Error(), "初始化 Git 失败") {
		t.Fatalf("expected Git initialization error, got %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("failed create left files behind: %v", entries)
	}
}

func TestCreatePreservesExistingProject(t *testing.T) {
	dest := t.TempDir()
	path := filepath.Join(dest, "README.md")
	const original = "existing project\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create("theme", dest, "", nil); err == nil || !strings.Contains(err.Error(), "项目目录已存在") {
		t.Fatalf("expected an existing project error, got %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("existing README changed to %q", data)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected no Git initialization in the existing project, got %v", err)
	}
}

func useTestGitConfig(t *testing.T, content string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("Git unavailable: %v", err)
	}
	config := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(config, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}
