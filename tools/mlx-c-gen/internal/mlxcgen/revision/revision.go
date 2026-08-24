// Package revision derives release identities for generated MLX C sources.
package revision

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	coreVersionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	shaRE         = regexp.MustCompile(`^[0-9a-fA-F]{12,64}$`)
	versionLineRE = regexp.MustCompile(`^#define MLX_VERSION_(MAJOR|MINOR|PATCH) ([0-9]+)$`)
)

// State identifies one reviewed generator revision for an MLX core version.
type State struct {
	Core     string
	Revision int
}

// Identity names either a stable revision or a preview of one.
type Identity struct {
	State
	PreviewSHA string
}

// Next returns the required state for targetCore. A changed cut of the same
// core increments the revision; a new core starts at revision 1.
func Next(base State, targetCore string, changed bool) (State, error) {
	if err := validateCore(targetCore); err != nil {
		return State{}, err
	}
	if base.Core == "" {
		return State{Core: targetCore, Revision: 1}, nil
	}
	if err := ValidateState(base); err != nil {
		return State{}, fmt.Errorf("base: %w", err)
	}
	if base.Core != targetCore {
		return State{Core: targetCore, Revision: 1}, nil
	}
	rev := base.Revision
	if changed {
		rev++
	}
	return State{Core: targetCore, Revision: rev}, nil
}

// ValidateTransition checks that next follows the revision progression rule.
func ValidateTransition(base, next State, changed bool) error {
	want, err := Next(base, next.Core, changed)
	if err != nil {
		return err
	}
	if next.Revision != want.Revision {
		if base.Core == next.Core && changed && next.Revision == base.Revision {
			return fmt.Errorf("release revision %d reuses changed core %s; want %d", next.Revision, next.Core, want.Revision)
		}
		return fmt.Errorf("release revision for core %s = %d, want %d", next.Core, next.Revision, want.Revision)
	}
	return nil
}

// ValidateState checks that a release state can be named.
func ValidateState(state State) error {
	if err := validateCore(state.Core); err != nil {
		return err
	}
	if state.Revision < 1 {
		return fmt.Errorf("release revision = %d, want a positive integer", state.Revision)
	}
	return nil
}

// NewIdentity returns an identity for state. previewSHA may be empty for a
// stable identity; otherwise it must be a hexadecimal commit ID.
func NewIdentity(state State, previewSHA string) (Identity, error) {
	if err := ValidateState(state); err != nil {
		return Identity{}, err
	}
	if previewSHA != "" && !shaRE.MatchString(previewSHA) {
		return Identity{}, fmt.Errorf("preview commit %q is not a hexadecimal commit ID", previewSHA)
	}
	return Identity{State: state, PreviewSHA: strings.ToLower(previewSHA)}, nil
}

// GeneratorTag returns the mlx-c-gen tag-shaped identity.
func (id Identity) GeneratorTag() string {
	name := fmt.Sprintf("mlx-v%s-rev%d", id.Core, id.Revision)
	if id.PreviewSHA != "" {
		name += "-dev." + id.PreviewSHA[:12]
	}
	return name
}

// LibraryTag returns the matching native-library release identity.
func (id Identity) LibraryTag() string {
	return "libs-" + strings.TrimPrefix(id.GeneratorTag(), "mlx-")
}

// CandidateBranch returns the review branch for id.
func (id Identity) CandidateBranch() string {
	return "build/" + id.GeneratorTag()
}

// CoreVersion reads the MLX core version from mlx/version.h below root.
func CoreVersion(root string) (string, error) {
	f, err := os.Open(filepath.Join(root, "mlx", "version.h"))
	if err != nil {
		return "", fmt.Errorf("read MLX version: %w", err)
	}
	defer f.Close()
	return ParseCoreVersion(f)
}

// ParseCoreVersion parses MLX_VERSION_MAJOR, MINOR, and PATCH definitions.
func ParseCoreVersion(r io.Reader) (string, error) {
	parts := map[string]int{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		match := versionLineRE.FindStringSubmatch(strings.TrimSpace(s.Text()))
		if match == nil {
			continue
		}
		n, err := strconv.Atoi(match[2])
		if err != nil {
			return "", fmt.Errorf("parse MLX version %s: %w", match[1], err)
		}
		parts[match[1]] = n
	}
	if err := s.Err(); err != nil {
		return "", fmt.Errorf("read MLX version: %w", err)
	}
	for _, name := range []string{"MAJOR", "MINOR", "PATCH"} {
		if _, ok := parts[name]; !ok {
			return "", fmt.Errorf("MLX version header has no %s definition", name)
		}
	}
	return fmt.Sprintf("%d.%d.%d", parts["MAJOR"], parts["MINOR"], parts["PATCH"]), nil
}

func validateCore(core string) error {
	if !coreVersionRE.MatchString(core) {
		return fmt.Errorf("MLX core version %q is not major.minor.patch", core)
	}
	return nil
}
