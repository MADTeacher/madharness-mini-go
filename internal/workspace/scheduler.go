// Package workspace координирует конкурентный доступ агентских сессий к workspace.
package workspace

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// LockMode описывает тип доступа к workspace.
type LockMode string

const (
	// LockRead разрешает параллельные чтения, но конфликтует с записью в тот же путь.
	LockRead LockMode = "read"
	// LockWrite конфликтует с чтением или записью в тот же путь, ancestor или descendant.
	LockWrite LockMode = "write"
	// LockExclusive конфликтует со всеми активными workspace-действиями.
	LockExclusive LockMode = "exclusive"
)

// Scheduler выдаёт locks для file/shell tools всех agent sessions одного run.
type Scheduler struct {
	mu      sync.Mutex
	cond    *sync.Cond
	nextID  int64
	active  []lockRequest
	waiting []lockRequest
}

type lockRequest struct {
	id    int64
	mode  LockMode
	paths []string
}

// NewScheduler создаёт пустой workspace scheduler.
func NewScheduler() *Scheduler {
	s := &Scheduler{}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Acquire ждёт, пока request перестанет конфликтовать с активными и ожидающими locks.
func (s *Scheduler) Acquire(mode LockMode, paths []string) func() {
	if s == nil {
		return func() {}
	}
	request := lockRequest{mode: mode, paths: normalizePaths(paths)}
	s.mu.Lock()
	s.nextID++
	request.id = s.nextID
	s.waiting = append(s.waiting, request)
	for !s.canGrant(request) {
		s.cond.Wait()
	}
	s.removeWaiting(request.id)
	s.active = append(s.active, request)
	s.mu.Unlock()
	return func() {
		s.release(request.id)
	}
}

func (s *Scheduler) release(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, item := range s.active {
		if item.id == id {
			s.active = append(s.active[:index], s.active[index+1:]...)
			s.cond.Broadcast()
			return
		}
	}
}

func (s *Scheduler) canGrant(request lockRequest) bool {
	for _, active := range s.active {
		if requestsConflict(active, request) {
			return false
		}
	}
	for _, waiting := range s.waiting {
		if waiting.id == request.id {
			return true
		}
		if requestsConflict(waiting, request) {
			return false
		}
	}
	return true
}

func (s *Scheduler) removeWaiting(id int64) {
	for index, item := range s.waiting {
		if item.id == id {
			s.waiting = append(s.waiting[:index], s.waiting[index+1:]...)
			return
		}
	}
}

func requestsConflict(left lockRequest, right lockRequest) bool {
	if left.mode == LockExclusive || right.mode == LockExclusive {
		return true
	}
	if left.mode == LockRead && right.mode == LockRead {
		return false
	}
	for _, leftPath := range left.paths {
		for _, rightPath := range right.paths {
			if pathsOverlap(leftPath, rightPath) {
				return true
			}
		}
	}
	return false
}

func pathsOverlap(left string, right string) bool {
	if left == right {
		return true
	}
	return strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func normalizePaths(paths []string) []string {
	seen := map[string]bool{}
	for _, path := range paths {
		cleaned := filepath.ToSlash(filepath.Clean(path))
		if cleaned == "." || cleaned == "" {
			cleaned = "/"
		}
		seen[cleaned] = true
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
