package compat

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WaiverSchemaVersion is the schema version of both waiver files.
const WaiverSchemaVersion = 1

// Default waiver paths, relative to the repository root.
const (
	DefaultParityWaiverPath  = "codegen/compat-waivers.yaml"
	DefaultRemovalWaiverPath = "codegen/removals.yaml"
)

// ParityWaivers declares the shared symbols whose declarations are allowed to
// differ from upstream.
type ParityWaivers struct {
	SchemaVersion int          `yaml:"schema_version"`
	Divergences   []Divergence `yaml:"divergences"`
}

// Divergence declares one shared symbol that diverges from upstream. Ours
// and Upstream pin the divergence: the gate fails if the tree no longer
// declares exactly what the waiver describes.
type Divergence struct {
	Name       string `yaml:"name"`
	Ours       string `yaml:"ours"`
	Upstream   string `yaml:"upstream"`
	Reason     string `yaml:"reason"`
	Resolution string `yaml:"resolution"`
}

// RemovalWaivers declares the locked functions that regeneration is allowed to
// drop.
type RemovalWaivers struct {
	SchemaVersion int       `yaml:"schema_version"`
	Removals      []Removal `yaml:"removals"`
}

// Removal declares one function dropped from the API lock.
type Removal struct {
	Target     string `yaml:"target"`
	Name       string `yaml:"name"`
	Reason     string `yaml:"reason"`
	Resolution string `yaml:"resolution"`
}

// LoadParityWaivers reads the parity waiver file at path. A missing file is
// an empty waiver set, which is the correct default: no divergence is allowed
// until one is declared.
func LoadParityWaivers(path string) (ParityWaivers, error) {
	var w ParityWaivers
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return ParityWaivers{SchemaVersion: WaiverSchemaVersion}, nil
	}
	if err != nil {
		return w, fmt.Errorf("open parity waivers: %w", err)
	}
	defer f.Close()
	if err := decodeWaivers(f, &w); err != nil {
		return ParityWaivers{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := w.validate(); err != nil {
		return ParityWaivers{}, fmt.Errorf("%s: %w", path, err)
	}
	return w, nil
}

// LoadRemovalWaivers reads the removals waiver file at path.
func LoadRemovalWaivers(path string) (RemovalWaivers, error) {
	var w RemovalWaivers
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return RemovalWaivers{SchemaVersion: WaiverSchemaVersion}, nil
	}
	if err != nil {
		return w, fmt.Errorf("open removal waivers: %w", err)
	}
	defer f.Close()
	if err := decodeWaivers(f, &w); err != nil {
		return RemovalWaivers{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := w.validate(); err != nil {
		return RemovalWaivers{}, fmt.Errorf("%s: %w", path, err)
	}
	return w, nil
}

func decodeWaivers(r io.Reader, out any) error {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (w ParityWaivers) validate() error {
	if err := checkSchema(w.SchemaVersion); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, d := range w.Divergences {
		switch {
		case d.Name == "":
			return fmt.Errorf("divergence %d: missing name", i)
		case d.Ours == "":
			return fmt.Errorf("divergence %s: missing ours", d.Name)
		case d.Upstream == "":
			return fmt.Errorf("divergence %s: missing upstream", d.Name)
		case d.Reason == "":
			return fmt.Errorf("divergence %s: missing reason", d.Name)
		case d.Resolution == "":
			return fmt.Errorf("divergence %s: missing resolution", d.Name)
		case seen[d.Name]:
			return fmt.Errorf("divergence %s: declared twice", d.Name)
		}
		seen[d.Name] = true
	}
	return nil
}

func (w RemovalWaivers) validate() error {
	if err := checkSchema(w.SchemaVersion); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, r := range w.Removals {
		key := r.Target + "/" + r.Name
		switch {
		case r.Target == "":
			return fmt.Errorf("removal %d: missing target", i)
		case r.Name == "":
			return fmt.Errorf("removal %d: missing name", i)
		case r.Reason == "":
			return fmt.Errorf("removal %s: missing reason", key)
		case r.Resolution == "":
			return fmt.Errorf("removal %s: missing resolution", key)
		case seen[key]:
			return fmt.Errorf("removal %s: declared twice", key)
		}
		seen[key] = true
	}
	return nil
}

func checkSchema(version int) error {
	if version != WaiverSchemaVersion {
		return fmt.Errorf("schema_version %d, want %d", version, WaiverSchemaVersion)
	}
	return nil
}

// normalizeDecl collapses declaration whitespace so that formatting changes do
// not read as divergences.
func normalizeDecl(decl string) string {
	return strings.Join(strings.Fields(decl), " ")
}
