package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/compat"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/plan"
)

// checkUpstreamParity enforces D5: every function we share with upstream mlx-c
// must be declared the same way. Our extras are additive and exempt.
func checkUpstreamParity(opts checkOptions, lock *apilock.Lock, manifest plan.Manifest) error {
	headersDir, cleanup, err := upstreamHeaders(opts, manifest)
	if err != nil {
		return err
	}
	defer cleanup()
	if headersDir == "" {
		return nil
	}
	if lock == nil {
		return fmt.Errorf("upstream parity needs the API lock; configure --lock")
	}
	upstream, err := apilock.GenerateTarget(headersDir, "mlx.h")
	if err != nil {
		return fmt.Errorf("parse upstream headers: %w", err)
	}
	waivers, err := compat.LoadParityWaivers(repoPath(opts.Options.RepoRoot, opts.ParityWaiverPath))
	if err != nil {
		return err
	}
	res := compat.CheckParity(lock.Targets["mlxc"], upstream, waivers)
	fmt.Fprintf(os.Stderr, "upstream parity: %d shared, %d ours-only, %d divergent (%d waived)\n",
		res.Shared, res.OursOnly, len(res.Divergent), res.Waived)
	return res.Err()
}

// upstreamHeaders resolves the upstream header directory to compare against.
// An explicit --upstream-headers wins; otherwise the manifest's upstream ref is
// extracted from this repository, so a working tree with the upstream remote
// fetched needs no second checkout. It returns an empty directory only when the
// manifest does not require the gate.
func upstreamHeaders(opts checkOptions, manifest plan.Manifest) (string, func(), error) {
	noop := func() {}
	if opts.UpstreamHeaders != "" {
		return opts.UpstreamHeaders, noop, nil
	}
	ref := manifest.Upstream.Ref
	if ref == "" {
		if manifest.Report.RequireUpstreamParity {
			return "", noop, fmt.Errorf("manifest requires upstream parity but declares no upstream.ref; pass --upstream-headers")
		}
		return "", noop, nil
	}
	dir, err := os.MkdirTemp("", "mlx-c-upstream-")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	headers := manifest.Upstream.HeadersDir
	if headers == "" {
		headers = "mlx/c"
	}
	if err := gitExtract(opts.Options.RepoRoot, ref, headers, dir); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("extract upstream headers from %s: %w (fetch the upstream remote, or pass --upstream-headers)", ref, err)
	}
	return filepath.Join(dir, filepath.FromSlash(headers)), cleanup, nil
}

// gitExtract writes the tree at ref:path into dir.
func gitExtract(repoRoot, ref, path, dir string) error {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	archive := exec.Command("git", "-C", root, "archive", ref, path)
	untar := exec.Command("tar", "-x", "-C", dir)
	untar.Stdin, err = archive.StdoutPipe()
	if err != nil {
		return err
	}
	archive.Stderr = os.Stderr
	untar.Stderr = os.Stderr
	if err := untar.Start(); err != nil {
		return err
	}
	if err := archive.Run(); err != nil {
		untar.Wait()
		return err
	}
	return untar.Wait()
}

// checkLockMonotonicity enforces that regeneration does not drop a function the
// previously committed lock recorded.
func checkLockMonotonicity(opts checkOptions, lock *apilock.Lock, manifest plan.Manifest) error {
	required := manifest.Report.RequireLockMonotonicity
	if !required && opts.Baseline == "HEAD" {
		return nil
	}
	if lock == nil {
		return fmt.Errorf("lock monotonicity needs the API lock; configure --lock")
	}
	baseline, err := loadBaselineLock(opts)
	if err != nil {
		return err
	}
	waivers, err := compat.LoadRemovalWaivers(repoPath(opts.Options.RepoRoot, opts.RemovalWaiverPath))
	if err != nil {
		return err
	}
	res := compat.CheckMonotonic(baseline, lock, waivers)
	fmt.Fprintf(os.Stderr, "lock monotonicity: %d added, %d removed, %d waived removals\n",
		len(res.Added), len(res.Removed), len(res.Waived))
	return res.Err()
}

// loadBaselineLock reads the lock to compare against. --baseline takes either
// a lock file path or a git ref. The default is HEAD: the committed lock, not
// the file on disk — by the time check runs, regeneration has already rewritten
// the working tree, so an on-disk baseline would compare the new lock against
// itself.
func loadBaselineLock(opts checkOptions) (*apilock.Lock, error) {
	if _, err := os.Stat(opts.Baseline); err == nil {
		return apilock.Load(opts.Baseline)
	}
	data, err := gitShow(opts.Options.RepoRoot, opts.Baseline, opts.LockPath)
	if err != nil {
		return nil, fmt.Errorf("read baseline lock %s:%s: %w (pass --baseline FILE to override)",
			opts.Baseline, opts.LockPath, err)
	}
	return apilock.Decode(data)
}

func gitShow(repoRoot, ref, path string) ([]byte, error) {
	dir, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "-C", dir, "show", ref+":"+filepath.ToSlash(path))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}
