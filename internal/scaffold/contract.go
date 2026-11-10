package scaffold

import (
	"fmt"
	"regexp"
	"strings"
)

// These fields are the subset of the BMCShell coreApi 1 package contract
// emitted by the scaffold. The runtime validates the generated manifest.
type tool struct {
	Scope    string            `json:"scope"`
	Entry    string            `json:"entry"`
	Runtime  string            `json:"runtime"`
	EnvReads []environmentRead `json:"envReads,omitempty"`
}

type environmentRead struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

type libraryContract struct {
	SchemaVersion int            `json:"schemaVersion"`
	Tables        []libraryTable `json:"tables"`
	Lookup        libraryLookup  `json:"lookup"`
}

type libraryTable struct {
	Name string   `json:"name"`
	Keys []string `json:"keys"`
}

type libraryLookup struct {
	Table string `json:"table"`
	Value string `json:"value"`
}

var (
	publisherRE = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	shortNameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	versionRE   = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

// NameFor constructs the machine-readable package name.
func NameFor(publisher, kind, shortName string) (string, error) {
	if !publisherRE.MatchString(publisher) {
		return "", fmt.Errorf("invalid publisher %q: use lowercase ASCII letters and digits, starting with a letter", publisher)
	}
	if len(shortName) > 48 || !shortNameRE.MatchString(shortName) {
		return "", fmt.Errorf("invalid shortName %q: use 1–48 lowercase ASCII letters and digits, with single hyphens between words", shortName)
	}
	switch kind {
	case "book", "library", "theme", "tool", "bundle":
	case "toolplugin":
		kind = "tool"
	default:
		return "", fmt.Errorf("invalid extension type %q", kind)
	}
	return publisher + "-" + kind + "-" + shortName, nil
}

// ParseName splits a canonical package name and returns the manifest type.
func ParseName(name string) (publisher, kind, shortName string, err error) {
	parts := strings.SplitN(name, "-", 3)
	if len(parts) != 3 || parts[1] == "toolplugin" {
		return "", "", "", fmt.Errorf("invalid package name %q: expected publisher-type-shortName", name)
	}
	publisher, kind, shortName = parts[0], parts[1], parts[2]
	if _, err = NameFor(publisher, kind, shortName); err != nil {
		return "", "", "", err
	}
	if kind == "tool" {
		kind = "toolplugin"
	}
	return publisher, kind, shortName, nil
}
