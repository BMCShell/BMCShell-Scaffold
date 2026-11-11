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
}

type libraryTable struct {
	Name        string                  `json:"name"`
	Keys        []string                `json:"keys"`
	ViewColumns []string                `json:"viewColumns"`
	Fields      map[string]libraryField `json:"fields"`
}

type libraryField struct {
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Pattern     string `json:"pattern,omitempty"`
}

var (
	publisherRE = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	shortNameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	versionRE   = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

// NameFor constructs the machine-readable package name.
func NameFor(publisher, kind, shortName string) (string, error) {
	if !publisherRE.MatchString(publisher) {
		return "", fmt.Errorf("发布方 %q 无效。请用小写英文字母开头，后续可含小写字母和数字", publisher)
	}
	if len(shortName) > 48 || !shortNameRE.MatchString(shortName) {
		return "", fmt.Errorf("短名 %q 无效。请使用 1 到 48 个小写英文字母和数字，单词间可用单个连字符", shortName)
	}
	switch kind {
	case "book", "library", "theme", "tool", "bundle":
	case "toolplugin":
		kind = "tool"
	default:
		return "", fmt.Errorf("插件类型 %q 无效", kind)
	}
	return publisher + "-" + kind + "-" + shortName, nil
}

// ParseName splits a canonical package name and returns the manifest type.
func ParseName(name string) (publisher, kind, shortName string, err error) {
	parts := strings.SplitN(name, "-", 3)
	if len(parts) != 3 || parts[1] == "toolplugin" {
		return "", "", "", fmt.Errorf("包名 %q 无效。请使用 发布方-类型-短名 格式", name)
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
