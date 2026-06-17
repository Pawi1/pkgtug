package gitops

import (
	"fmt"
	"os/exec"
	"strings"
)

// Fetch runs git fetch on an existing local clone.
// For tag-tracked packages it fetches tags; for branch-tracked it fetches the specific branch.
func Fetch(localClone, versionType, branchName string) error {
	var args []string
	switch versionType {
	case "tag":
		args = []string{"fetch", "--tags", "--prune", "--prune-tags"}
	case "branch":
		args = []string{"fetch", "origin", branchName}
	default:
		return fmt.Errorf("unknown version type %q", versionType)
	}
	return run(localClone, args...)
}

// LatestTag returns the most recent tag matching the glob pattern (by version sort).
func LatestTag(localClone, pattern string) (string, error) {
	out, err := runOutput(localClone, "tag", "--list", pattern, "--sort=-version:refname")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, l := range lines {
		if l != "" {
			return l, nil
		}
	}
	return "", fmt.Errorf("no tag matching %q in %s", pattern, localClone)
}

// BranchSHA returns the short SHA of the tip of the given branch on origin.
func BranchSHA(localClone, branchName string) (string, error) {
	ref := "refs/remotes/origin/" + branchName
	out, err := runOutput(localClone, "rev-parse", "--short=8", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func run(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func runOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
