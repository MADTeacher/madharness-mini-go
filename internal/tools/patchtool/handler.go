package patchtool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func applyPatch(ctx *tools.Context, args map[string]any) tools.Observation {
	patch := tools.StringArg(args, "patch", "")
	paths, pathMap, err := touchedPaths(ctx, patch)
	if err != nil {
		summary := err.Error()
		return tools.Fail("apply_patch", summary, failureData(summary))
	}
	release := ctx.LockWorkspaceWrite("apply_patch", paths...)
	defer release()
	parser := Parser{ctx: ctx, paths: pathMap}
	changes, err := parser.Prepare(patch)
	if err != nil {
		summary := err.Error()
		return tools.Fail("apply_patch", summary, failureData(summary))
	}
	if err := applyChanges(changes); err != nil {
		return tools.Fail("apply_patch", err.Error())
	}
	return tools.OK("apply_patch", "applied patch to "+itoa(len(uniqueChangePaths(changes)))+" file(s)", nil)
}

type fileSnapshot struct {
	exists  bool
	content []byte
	mode    os.FileMode
}

func applyChanges(changes []Change) error {
	if err := validateWriteTargets(changes); err != nil {
		return err
	}
	snapshots, err := snapshotChanges(changes)
	if err != nil {
		return err
	}
	createdDirs := []string{}
	for _, change := range changes {
		if change.Content == nil {
			if err := os.Remove(change.Path); err != nil {
				return failWithRollback(err, snapshots, createdDirs)
			}
			continue
		}
		created, err := mkdirAllTracked(filepath.Dir(change.Path))
		createdDirs = append(createdDirs, created...)
		if err != nil {
			return failWithRollback(err, snapshots, createdDirs)
		}
		if err := os.WriteFile(change.Path, []byte(*change.Content), 0o644); err != nil {
			return failWithRollback(err, snapshots, createdDirs)
		}
	}
	return nil
}

func validateWriteTargets(changes []Change) error {
	for _, change := range changes {
		if change.Content == nil {
			continue
		}
		if info, err := os.Stat(change.Path); err == nil {
			if info.IsDir() {
				return fmt.Errorf("not a file: %s", change.Path)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := validateParentDirectory(change.Path); err != nil {
			return err
		}
	}
	return nil
}

func validateParentDirectory(path string) error {
	parent := filepath.Dir(path)
	for {
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("not a directory: %s", parent)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return err
		}
		parent = next
	}
}

func snapshotChanges(changes []Change) (map[string]fileSnapshot, error) {
	snapshots := map[string]fileSnapshot{}
	for _, change := range changes {
		if _, ok := snapshots[change.Path]; ok {
			continue
		}
		info, err := os.Stat(change.Path)
		if errors.Is(err, os.ErrNotExist) {
			snapshots[change.Path] = fileSnapshot{}
			continue
		}
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(change.Path)
		if err != nil {
			return nil, err
		}
		snapshots[change.Path] = fileSnapshot{
			exists:  true,
			content: content,
			mode:    info.Mode().Perm(),
		}
	}
	return snapshots, nil
}

func failWithRollback(applyErr error, snapshots map[string]fileSnapshot, createdDirs []string) error {
	rollbackErr := errors.Join(rollbackSnapshots(snapshots), removeCreatedDirs(createdDirs))
	if rollbackErr != nil {
		return fmt.Errorf("%v; rollback failed: %w", applyErr, rollbackErr)
	}
	return applyErr
}

func rollbackSnapshots(snapshots map[string]fileSnapshot) error {
	paths := snapshotPaths(snapshots)
	sort.Slice(paths, func(i int, j int) bool {
		return pathDepth(paths[i]) > pathDepth(paths[j])
	})
	errs := []error{}
	for _, path := range paths {
		if snapshots[path].exists {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	sort.Slice(paths, func(i int, j int) bool {
		return pathDepth(paths[i]) < pathDepth(paths[j])
	})
	for _, path := range paths {
		snapshot := snapshots[path]
		if !snapshot.exists {
			continue
		}
		if err := restoreSnapshot(path, snapshot); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func restoreSnapshot(path string, snapshot fileSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, snapshot.content, snapshot.mode); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return errors.Join(err, removeErr)
		}
		if retryErr := os.WriteFile(path, snapshot.content, snapshot.mode); retryErr != nil {
			return errors.Join(err, retryErr)
		}
	}
	return os.Chmod(path, snapshot.mode)
}

func removeCreatedDirs(dirs []string) error {
	errs := []error{}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Remove(dirs[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// mkdirAllTracked создаёт недостающие каталоги и возвращает только те,
// которые нужно удалить при откате неуспешного patch.
func mkdirAllTracked(path string) ([]string, error) {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil, nil
		}
		return nil, fmt.Errorf("not a directory: %s", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return nil, err
	}
	dirs, err := mkdirAllTracked(parent)
	if err != nil {
		return dirs, err
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
				return dirs, nil
			} else if statErr != nil {
				return dirs, statErr
			}
			return dirs, fmt.Errorf("not a directory: %s", path)
		}
		return dirs, err
	}
	return append(dirs, path), nil
}

func snapshotPaths(snapshots map[string]fileSnapshot) []string {
	paths := make([]string, 0, len(snapshots))
	for path := range snapshots {
		paths = append(paths, path)
	}
	return paths
}

func pathDepth(path string) int {
	return strings.Count(filepath.Clean(path), string(filepath.Separator))
}

func uniqueChangePaths(changes []Change) map[string]bool {
	paths := map[string]bool{}
	for _, change := range changes {
		paths[change.Path] = true
	}
	return paths
}
