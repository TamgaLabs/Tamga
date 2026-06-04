package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Cloner struct {
	workDir string
}

func NewCloner(workDir string) *Cloner {
	return &Cloner{workDir: workDir}
}

type CloneResult struct {
	Path    string
	Commit  string
	Message string
}

func (c *Cloner) Clone(ctx context.Context, url, branch string) (*CloneResult, error) {
	if err := os.MkdirAll(c.workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work dir: %w", err)
	}

	tmpDir, err := os.MkdirTemp(c.workDir, "repo-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	cloneArgs := []string{"clone", "--depth", "1"}
	if branch != "" {
		cloneArgs = append(cloneArgs, "--branch", branch)
	}
	cloneArgs = append(cloneArgs, url, tmpDir)

	cmd := exec.CommandContext(ctx, "git", cloneArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	return c.resolveResult(tmpDir)
}

func (c *Cloner) Pull(ctx context.Context, repoDir, branch string) (*CloneResult, error) {
	cmd := exec.CommandContext(ctx, "git", "checkout", branch)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git checkout failed: %w", err)
	}

	cmd = exec.CommandContext(ctx, "git", "pull", "origin", branch)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git pull failed: %w", err)
	}

	return c.resolveResult(repoDir)
}

func (c *Cloner) resolveResult(repoDir string) (*CloneResult, error) {
	commit, err := runGit(repoDir, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	msg, err := runGit(repoDir, "log", "--format=%s", "-1")
	if err != nil {
		return nil, err
	}

	return &CloneResult{
		Path:    repoDir,
		Commit:  commit,
		Message: msg,
	}, nil
}

func (c *Cloner) ListFiles(repoDir string) ([]string, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

func (c *Cloner) Cleanup(repoDir string) error {
	return os.RemoveAll(repoDir)
}

func runGit(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v failed: %w", args, err)
	}
	return trimNewline(string(out)), nil
}

func trimNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func DockerfilePath(repoDir string) string {
	p := filepath.Join(repoDir, "Dockerfile")
	if FileExists(p) {
		return p
	}
	p = filepath.Join(repoDir, "dockerfile")
	if FileExists(p) {
		return p
	}
	return ""
}
