package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/allisonmahmood/nt/internal/git"
)

// LockMutation serializes NT's home refreshes and transfers, including processes
// started from different worktrees. Git and editors do not participate in this lock.
func LockMutation(dir string) (func(), error) {
	common, ok := git.Query(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok {
		return nil, fmt.Errorf("cannot locate repository metadata")
	}
	f, err := os.OpenFile(filepath.Join(common, "nt-mutation.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another NT home refresh or transfer is running")
	}
	return func() { _ = f.Close() }, nil
}

// CheckIdle refuses operations and index states whose work cannot safely be
// interpreted as an ordinary checkout. Query failures always fail closed.
func CheckIdle(dir string) error {
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG", "rebase-merge", "rebase-apply", "sequencer", "index.lock"} {
		path, ok := git.Query(dir, "rev-parse", "--path-format=absolute", "--git-path", name)
		if !ok {
			return fmt.Errorf("cannot inspect Git operation state")
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("git operation or lock present (%s)", name)
		}
	}
	entries, ok := git.Query(dir, "ls-files", "--stage")
	if !ok {
		return fmt.Errorf("cannot inspect index")
	}
	for _, entry := range git.Lines(entries) {
		meta, _, _ := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[2] != "0" || fields[0] == "160000" {
			return fmt.Errorf("submodules or unmerged index entries need manual handling")
		}
	}
	flags, ok := git.Query(dir, "ls-files", "-v")
	if !ok {
		return fmt.Errorf("cannot inspect index flags")
	}
	for _, entry := range git.Lines(flags) {
		if entry[0] == 'S' || (entry[0] >= 'a' && entry[0] <= 'z') {
			return fmt.Errorf("skip-worktree or assume-unchanged entries need manual handling")
		}
	}
	return nil
}
