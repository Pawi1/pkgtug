package server

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/pawi1/pkgtug/internal/config"
)

const jobTimeout = 10 * time.Minute

// Job represents a pending or active build task.
type Job struct {
	ID           string         `json:"job_id"`
	PackageName  string         `json:"package"`
	GitURL       string         `json:"git_url"`
	Version      string         `json:"version"`
	BuildCommand string         `json:"build_command"`
	Binaries     []config.Binary `json:"binaries"`
	Platform     string         `json:"platform"`
	ClaimedAt    time.Time      `json:"-"`
}

// jobRegistry maps job ID → job for result lookup.
type jobRegistry struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func newJobRegistry() *jobRegistry {
	return &jobRegistry{jobs: make(map[string]*Job)}
}

func (r *jobRegistry) add(j *Job) {
	r.mu.Lock()
	r.jobs[j.ID] = j
	r.mu.Unlock()
}

func (r *jobRegistry) get(id string) (*Job, bool) {
	r.mu.Lock()
	j, ok := r.jobs[id]
	r.mu.Unlock()
	return j, ok
}

func (r *jobRegistry) delete(id string) {
	r.mu.Lock()
	delete(r.jobs, id)
	r.mu.Unlock()
}

func randomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
