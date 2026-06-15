package workspace

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerAllowsDifferentWritePaths(t *testing.T) {
	s := NewScheduler()
	firstRelease := make(chan struct{})
	secondRelease := make(chan struct{})
	started := make(chan string, 2)
	done := make(chan string, 2)

	go lockedTask(s, "first", LockWrite, []string{"/repo/a.txt"}, started, firstRelease, done)
	waitLockStarted(t, started, "first")
	go lockedTask(s, "second", LockWrite, []string{"/repo/b.txt"}, started, secondRelease, done)
	waitLockStarted(t, started, "second")

	close(firstRelease)
	close(secondRelease)
	waitLockDone(t, done)
	waitLockDone(t, done)
}

func TestSchedulerSerializesSameAndNestedPaths(t *testing.T) {
	for _, tc := range []struct {
		name       string
		firstPath  string
		secondPath string
	}{
		{name: "same", firstPath: "/repo/a.txt", secondPath: "/repo/a.txt"},
		{name: "nested", firstPath: "/repo/dir", secondPath: "/repo/dir/a.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewScheduler()
			firstRelease := make(chan struct{})
			secondRelease := make(chan struct{})
			firstReleased := atomic.Bool{}
			earlyStart := atomic.Bool{}
			attempted := make(chan string, 1)
			started := make(chan string, 2)
			done := make(chan string, 2)

			go lockedTask(s, "first", LockWrite, []string{tc.firstPath}, started, firstRelease, done)
			waitLockStarted(t, started, "first")
			go guardedLockedTask(s, "second", LockRead, []string{tc.secondPath}, attempted, started, secondRelease, done, &firstReleased, &earlyStart)

			waitLockAttempted(t, attempted, "second")
			firstReleased.Store(true)
			close(firstRelease)
			waitLockStarted(t, started, "second")
			assertNoEarlyLockStart(t, &earlyStart, "second")
			close(secondRelease)
			waitLockDone(t, done)
			waitLockDone(t, done)
		})
	}
}

func TestSchedulerExclusiveLockBlocksPathLocks(t *testing.T) {
	s := NewScheduler()
	exclusiveRelease := make(chan struct{})
	readRelease := make(chan struct{})
	exclusiveReleased := atomic.Bool{}
	earlyStart := atomic.Bool{}
	attempted := make(chan string, 1)
	started := make(chan string, 2)
	done := make(chan string, 2)

	go lockedTask(s, "exclusive", LockExclusive, nil, started, exclusiveRelease, done)
	waitLockStarted(t, started, "exclusive")
	go guardedLockedTask(s, "read", LockRead, []string{"/repo/a.txt"}, attempted, started, readRelease, done, &exclusiveReleased, &earlyStart)

	waitLockAttempted(t, attempted, "read")
	exclusiveReleased.Store(true)
	close(exclusiveRelease)
	waitLockStarted(t, started, "read")
	assertNoEarlyLockStart(t, &earlyStart, "read")
	close(readRelease)
	waitLockDone(t, done)
	waitLockDone(t, done)
}

func TestSchedulerPendingWriteBlocksLaterConflictingRead(t *testing.T) {
	s := NewScheduler()
	firstReadRelease := make(chan struct{})
	writeRelease := make(chan struct{})
	nextReadRelease := make(chan struct{})
	firstReadReleased := atomic.Bool{}
	writeReleased := atomic.Bool{}
	writeEarlyStart := atomic.Bool{}
	nextReadEarlyStart := atomic.Bool{}
	attempted := make(chan string, 2)
	started := make(chan string, 3)
	done := make(chan string, 3)

	go lockedTask(s, "first-read", LockRead, []string{"/repo/a.txt"}, started, firstReadRelease, done)
	waitLockStarted(t, started, "first-read")
	go guardedLockedTask(s, "write", LockWrite, []string{"/repo/a.txt"}, attempted, started, writeRelease, done, &firstReadReleased, &writeEarlyStart)
	waitLockAttempted(t, attempted, "write")
	waitSchedulerWaiting(t, s, 1)
	go guardedLockedTask(s, "next-read", LockRead, []string{"/repo/a.txt"}, attempted, started, nextReadRelease, done, &writeReleased, &nextReadEarlyStart)
	waitLockAttempted(t, attempted, "next-read")
	waitSchedulerWaiting(t, s, 2)

	firstReadReleased.Store(true)
	close(firstReadRelease)
	waitLockStarted(t, started, "write")
	assertNoEarlyLockStart(t, &writeEarlyStart, "write")
	writeReleased.Store(true)
	close(writeRelease)
	waitLockStarted(t, started, "next-read")
	assertNoEarlyLockStart(t, &nextReadEarlyStart, "next-read")

	close(nextReadRelease)
	waitLockDone(t, done)
	waitLockDone(t, done)
	waitLockDone(t, done)
}

func TestSchedulerPendingExclusiveBlocksLaterRead(t *testing.T) {
	s := NewScheduler()
	firstReadRelease := make(chan struct{})
	exclusiveRelease := make(chan struct{})
	nextReadRelease := make(chan struct{})
	firstReadReleased := atomic.Bool{}
	exclusiveReleased := atomic.Bool{}
	exclusiveEarlyStart := atomic.Bool{}
	nextReadEarlyStart := atomic.Bool{}
	attempted := make(chan string, 2)
	started := make(chan string, 3)
	done := make(chan string, 3)

	go lockedTask(s, "first-read", LockRead, []string{"/repo/a.txt"}, started, firstReadRelease, done)
	waitLockStarted(t, started, "first-read")
	go guardedLockedTask(s, "exclusive", LockExclusive, nil, attempted, started, exclusiveRelease, done, &firstReadReleased, &exclusiveEarlyStart)
	waitLockAttempted(t, attempted, "exclusive")
	waitSchedulerWaiting(t, s, 1)
	go guardedLockedTask(s, "next-read", LockRead, []string{"/repo/b.txt"}, attempted, started, nextReadRelease, done, &exclusiveReleased, &nextReadEarlyStart)
	waitLockAttempted(t, attempted, "next-read")
	waitSchedulerWaiting(t, s, 2)

	firstReadReleased.Store(true)
	close(firstReadRelease)
	waitLockStarted(t, started, "exclusive")
	assertNoEarlyLockStart(t, &exclusiveEarlyStart, "exclusive")
	exclusiveReleased.Store(true)
	close(exclusiveRelease)
	waitLockStarted(t, started, "next-read")
	assertNoEarlyLockStart(t, &nextReadEarlyStart, "next-read")

	close(nextReadRelease)
	waitLockDone(t, done)
	waitLockDone(t, done)
	waitLockDone(t, done)
}

func TestSchedulerPendingWriteAllowsIndependentRead(t *testing.T) {
	s := NewScheduler()
	firstReadRelease := make(chan struct{})
	writeRelease := make(chan struct{})
	independentReadRelease := make(chan struct{})
	firstReadReleased := atomic.Bool{}
	writeEarlyStart := atomic.Bool{}
	attempted := make(chan string, 1)
	started := make(chan string, 3)
	done := make(chan string, 3)

	go lockedTask(s, "first-read", LockRead, []string{"/repo/a.txt"}, started, firstReadRelease, done)
	waitLockStarted(t, started, "first-read")
	go guardedLockedTask(s, "write", LockWrite, []string{"/repo/a.txt"}, attempted, started, writeRelease, done, &firstReadReleased, &writeEarlyStart)
	waitLockAttempted(t, attempted, "write")
	waitSchedulerWaiting(t, s, 1)
	go lockedTask(s, "independent-read", LockRead, []string{"/repo/b.txt"}, started, independentReadRelease, done)
	waitLockStarted(t, started, "independent-read")

	close(independentReadRelease)
	waitLockDone(t, done)
	firstReadReleased.Store(true)
	close(firstReadRelease)
	waitLockStarted(t, started, "write")
	assertNoEarlyLockStart(t, &writeEarlyStart, "write")
	close(writeRelease)
	waitLockDone(t, done)
	waitLockDone(t, done)
}

func lockedTask(
	s *Scheduler,
	name string,
	mode LockMode,
	paths []string,
	started chan<- string,
	release <-chan struct{},
	done chan<- string,
) {
	unlock := s.Acquire(mode, paths)
	started <- name
	<-release
	unlock()
	done <- name
}

func guardedLockedTask(
	s *Scheduler,
	name string,
	mode LockMode,
	paths []string,
	attempted chan<- string,
	started chan<- string,
	release <-chan struct{},
	done chan<- string,
	allowed *atomic.Bool,
	earlyStart *atomic.Bool,
) {
	attempted <- name
	unlock := s.Acquire(mode, paths)
	if !allowed.Load() {
		earlyStart.Store(true)
	}
	started <- name
	<-release
	unlock()
	done <- name
}

func waitLockAttempted(t *testing.T, attempted <-chan string, want string) {
	t.Helper()
	select {
	case name := <-attempted:
		if name != want {
			t.Fatalf("attempted %s, want %s", name, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s did not attempt to acquire lock", want)
	}
}

func waitSchedulerWaiting(t *testing.T, s *Scheduler, want int) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		got := len(s.waiting)
		s.mu.Unlock()
		if got == want {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("waiting locks count did not reach %d", want)
		}
	}
}

func waitLockStarted(t *testing.T, started <-chan string, want string) {
	t.Helper()
	select {
	case name := <-started:
		if name != want {
			t.Fatalf("started %s, want %s", name, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s did not start", want)
	}
}

func assertNoEarlyLockStart(t *testing.T, earlyStart *atomic.Bool, name string) {
	t.Helper()
	if earlyStart.Load() {
		t.Fatalf("%s lock started before the conflicting lock was released", name)
	}
}

func waitLockDone(t *testing.T, done <-chan string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lock holder did not finish")
	}
}
