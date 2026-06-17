package server

import (
	"sync"
	"time"
)

// packageState tracks detected and built versions per package.
type packageState struct {
	mu             sync.Mutex
	currentVersion string            // latest version seen in git (tag or branch SHA)
	builtVersions  map[string]string // platform → last successfully built version
	activeJobs     map[string]*Job   // platform → currently claimed job
}

func newPackageState() *packageState {
	return &packageState{
		builtVersions: make(map[string]string),
		activeJobs:    make(map[string]*Job),
	}
}

// pendingPlatforms returns platforms that need a build job for the current version.
// A platform needs a job when its built version differs from currentVersion
// and there is no live (non-timed-out) active job for it.
func (ps *packageState) pendingPlatforms(jobTimeout time.Duration) []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.currentVersion == "" {
		return nil
	}
	var out []string
	// We only produce a job for platforms that already have an active (timed-out)
	// or no job — callers ask per platform, so we return a bool here indirectly.
	// This method is not used directly; see needsJob below.
	_ = out
	return nil
}

// needsJob reports whether the given platform should receive a new build job.
func (ps *packageState) needsJob(platform string, jobTimeout time.Duration) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.currentVersion == "" {
		return false
	}
	if ps.builtVersions[platform] == ps.currentVersion {
		return false
	}
	if j, ok := ps.activeJobs[platform]; ok {
		if time.Since(j.ClaimedAt) < jobTimeout {
			return false // job is live
		}
		// timed out — allow re-scheduling
		delete(ps.activeJobs, platform)
	}
	return true
}

func (ps *packageState) claimJob(platform string, j *Job) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.activeJobs[platform] = j
}

func (ps *packageState) completeJob(platform, version string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.builtVersions[platform] = version
	delete(ps.activeJobs, platform)
}

func (ps *packageState) failJob(platform string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.activeJobs, platform)
}

func (ps *packageState) setVersion(v string) (changed bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.currentVersion == v {
		return false
	}
	ps.currentVersion = v
	return true
}

func (ps *packageState) getVersion() string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.currentVersion
}
