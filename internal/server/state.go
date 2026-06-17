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
