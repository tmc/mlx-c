// revisions validates release revision progression against a base ref.
// Folded from the former tools/mlx-c-revision-check.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/revision"
	"gopkg.in/yaml.v3"
)

var stableRefRE = regexp.MustCompile(`^v([0-9]+\.[0-9]+\.[0-9]+)$`)

type manifestFile struct {
	MLX plan.MLXPolicy `yaml:"mlx"`
}

type provenanceFile struct {
	ExpectedMLXRef string `json:"expected_mlx_ref"`
	CoreVersion    string `json:"core_version"`
	InputDigests   struct {
		RegenReportSHA256 string `json:"regen_report_sha256"`
	} `json:"input_digests"`
}

type regenReportFile struct {
	MLXRef struct {
		MatchesExpected bool `json:"matches_expected"`
	} `json:"mlx_ref"`
}

func runRevisions(args []string) error {
	fs := flag.NewFlagSet("mlx-c-gen revisions", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	base := fs.String("base", "origin/main", "base commit or branch")
	root, rest, err := takeRoot(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	return check(root, *base)
}

func check(root, base string) error {
	if err := checkCandidateReport(root); err != nil {
		return err
	}
	currentData, err := os.ReadFile(root + "/codegen/manifest.yaml")
	if err != nil {
		return fmt.Errorf("read current manifest: %w", err)
	}
	current, err := parseManifest(currentData)
	if err != nil {
		return err
	}
	baseData, err := gitOutput(root, "show", base+":codegen/manifest.yaml")
	if err != nil {
		return fmt.Errorf("read base manifest: %w", err)
	}
	previous, err := parseManifest([]byte(baseData))
	if err != nil {
		return fmt.Errorf("parse base manifest: %w", err)
	}

	changed, err := generatorContentChanged(root, base)
	if err != nil {
		return err
	}
	changed = changed || previous.MLX.ExpectedGitRef != current.MLX.ExpectedGitRef
	currentCore, err := manifestCore(root, "", current.MLX)
	if err != nil {
		return err
	}
	baseCore, err := manifestCore(root, base, previous.MLX)
	if err != nil {
		return err
	}

	if previous.MLX.ReleaseRevision == 0 {
		if changed || baseCore != currentCore || current.MLX.ReleaseRevision != 1 {
			return fmt.Errorf("legacy manifest migration must preserve MLX content and set release_revision to 1")
		}
		return nil
	}
	return revision.ValidateTransition(
		revision.State{Core: baseCore, Revision: previous.MLX.ReleaseRevision},
		revision.State{Core: currentCore, Revision: current.MLX.ReleaseRevision},
		changed,
	)
}

func checkCandidateReport(root string) error {
	provenancePath := root + "/codegen/candidate-provenance.json"
	reportPath := root + "/codegen/candidate-regen-report.json"
	provenanceData, provenanceErr := os.ReadFile(provenancePath)
	reportData, reportErr := os.ReadFile(reportPath)
	if os.IsNotExist(provenanceErr) && os.IsNotExist(reportErr) {
		return nil
	}
	if provenanceErr != nil {
		return fmt.Errorf("read candidate provenance: %w", provenanceErr)
	}
	if reportErr != nil {
		return fmt.Errorf("read candidate regeneration report: %w", reportErr)
	}
	var provenance provenanceFile
	if err := json.Unmarshal(provenanceData, &provenance); err != nil {
		return fmt.Errorf("parse candidate provenance: %w", err)
	}
	var report regenReportFile
	if err := json.Unmarshal(reportData, &report); err != nil {
		return fmt.Errorf("parse candidate regeneration report: %w", err)
	}
	if !report.MLXRef.MatchesExpected {
		return fmt.Errorf("candidate regeneration report does not match expected MLX ref")
	}
	got := fmt.Sprintf("%x", sha256.Sum256(reportData))
	if got != provenance.InputDigests.RegenReportSHA256 {
		return fmt.Errorf("candidate regeneration report digest = %s, want %s", got, provenance.InputDigests.RegenReportSHA256)
	}
	return nil
}

func parseManifest(data []byte) (manifestFile, error) {
	var manifest manifestFile
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return manifestFile{}, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.MLX.ExpectedGitRef == "" {
		return manifestFile{}, fmt.Errorf("manifest has empty mlx expected_git_ref")
	}
	if manifest.MLX.ReleaseRevision < 0 {
		return manifestFile{}, fmt.Errorf("manifest release_revision = %d, want a non-negative integer", manifest.MLX.ReleaseRevision)
	}
	return manifest, nil
}

func manifestCore(root, ref string, policy plan.MLXPolicy) (string, error) {
	if match := stableRefRE.FindStringSubmatch(policy.ExpectedGitRef); match != nil {
		return match[1], nil
	}
	data, err := readProvenance(root, ref)
	if err != nil {
		return "", fmt.Errorf("MLX ref %s is not a release tag and has no candidate provenance: %w", policy.ExpectedGitRef, err)
	}
	var provenance provenanceFile
	if err := json.Unmarshal(data, &provenance); err != nil {
		return "", fmt.Errorf("parse candidate provenance: %w", err)
	}
	if provenance.ExpectedMLXRef != policy.ExpectedGitRef || provenance.CoreVersion == "" {
		return "", fmt.Errorf("candidate provenance does not identify MLX ref %s", policy.ExpectedGitRef)
	}
	return provenance.CoreVersion, nil
}

func readProvenance(root, ref string) ([]byte, error) {
	if ref == "" {
		return os.ReadFile(root + "/codegen/candidate-provenance.json")
	}
	data, err := gitOutput(root, "show", ref+":codegen/candidate-provenance.json")
	return []byte(data), err
}

func generatorContentChanged(root, base string) (bool, error) {
	args := []string{
		"diff", "--quiet", base, "--",
		"mlx/c",
		"codegen/custom",
		"codegen/modules",
		"codegen/mlxc-capi.lock.json",
		"codegen/types.yaml",
	}
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("check generator content diff: %w", err)
}

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
