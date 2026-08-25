package apilock

import (
	"encoding/json"
	"sort"
	"strings"
)

// ABISchemaVersion is the schema version of emitted abi.json files.
const ABISchemaVersion = 1

// ABIEntry records the calling-convention classification of one C function's
// parameters, matching the split encoded in hand-written call helpers
// (e.g. Call8Float32x1 takes eight integer-class arguments followed by one
// float32).
type ABIEntry struct {
	Params            []string `json:"params"`
	IntegerClassCount int      `json:"integer_class_count"`
	FloatClassCount   int      `json:"float_class_count"`
}

// ABIMap is the machine-readable ABI manifest written next to the generated
// headers as abi.json. It describes the exact tree it ships beside.
type ABIMap struct {
	SchemaVersion int                 `json:"schema_version"`
	Functions     map[string]ABIEntry `json:"functions"`
}

// ClassifyParam reports whether a C parameter declaration belongs to the
// integer or floating-point argument class. Pointers (including typedef'd
// handles like mlx_array) pass as integers regardless of their pointee type;
// only scalar float/double values are float-class.
func ClassifyParam(param string) string {
	t := strings.TrimSpace(param)
	if strings.Contains(t, "*") {
		return "integer"
	}
	if strings.Contains(t, "float") || strings.Contains(t, "double") {
		return "float"
	}
	return "integer"
}

// ABIFromFunctions builds an ABIMap from parsed function declarations.
func ABIFromFunctions(fns []Function) ABIMap {
	m := ABIMap{SchemaVersion: ABISchemaVersion, Functions: make(map[string]ABIEntry, len(fns))}
	for _, fn := range fns {
		e := ABIEntry{Params: append([]string(nil), fn.Parameters...)}
		for _, p := range e.Params {
			switch ClassifyParam(p) {
			case "float":
				e.FloatClassCount++
			default:
				e.IntegerClassCount++
			}
		}
		sort.Strings(e.Params)
		m.Functions[fn.Name] = e
	}
	return m
}

// JSON renders the manifest.
func (m ABIMap) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// SortNames returns the sorted symbol names in the map.
func (m ABIMap) SortNames() []string {
	names := make([]string, 0, len(m.Functions))
	for name := range m.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
