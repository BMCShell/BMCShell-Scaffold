package scaffold

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBookProjectTemplate(t *testing.T) {
	const (
		packageName = "bmcshell-book-api-demo"
		shortName   = "api-demo"
		displayName = "接口示例工具书"
		description = "查询接口和请求示例"
		version     = "1.2.0"
		id          = "11111111-1111-4111-8111-111111111111"
		dataPath    = "book/api-demo.book.db"
	)
	packageDescription := description
	m, topics, err := blueprint(manifest{
		Type: "book", Name: packageName, Publisher: "bmcshell", ShortName: shortName,
		DisplayName: displayName, Description: &packageDescription, Version: version,
	}, id, packageName, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := populate(root, m, topics); err != nil {
		t.Fatal(err)
	}

	t.Run("manifest", func(t *testing.T) {
		var got manifest
		if err := json.Unmarshal(readProjectFile(t, root, "bmc-package.json"), &got); err != nil {
			t.Fatal(err)
		}
		if got.Schema != 1 || got.CoreAPI != 1 || got.ID != id {
			t.Errorf("unexpected manifest schema, API or ID: %+v", got)
		}
		if got.Type != "book" || got.Name != packageName || got.ShortName != shortName || got.Publisher != "bmcshell" {
			t.Errorf("unexpected package naming fields: %+v", got)
		}
		if got.DisplayName != displayName || got.Version != version || got.Summary != description {
			t.Errorf("unexpected display name, version or summary: %+v", got)
		}
		if got.Data != dataPath || got.Help != "help.json" || got.Changelog != "changelog.json" ||
			!slices.Equal(got.Platforms, []string{"any"}) {
			t.Errorf("unexpected package paths or platforms: %+v", got)
		}
	})

	t.Run("documentation", func(t *testing.T) {
		var help struct {
			Topics map[string]topic `json:"topics"`
		}
		if err := json.Unmarshal(readProjectFile(t, root, "help.json"), &help); err != nil {
			t.Fatal(err)
		}
		bookHelp := help.Topics["package"]
		readme := string(readProjectFile(t, root, "README.md"))
		for _, syntax := range []string{
			"bmc book api-demo [-l|--list]",
			"bmc book api-demo -s <关键词>",
			"bmc book api-demo [--id] <ID>",
			"bmc book api-demo --name <完整名称>",
			"bmc book api-demo [-v|--version]",
		} {
			if !slices.Contains(bookHelp.Usage, syntax) {
				t.Errorf("topics.package.usage missing %q: %v", syntax, bookHelp.Usage)
			}
		}
		for _, command := range []string{
			"bmc book api-demo -l",
			"bmc book api-demo -s '示例'",
			"bmc book api-demo --id 1",
			"bmc book api-demo 1",
			"bmc book api-demo --name '示例条目'",
		} {
			if !slices.ContainsFunc(bookHelp.Examples, func(item example) bool { return item.Command == command }) {
				t.Errorf("topics.package.examples missing %q", command)
			}
			if !strings.Contains(readme, "\n"+command+"\n") {
				t.Errorf("README missing command %q", command)
			}
		}
		for _, invalid := range []string{"bmc book " + packageName, "bmc book " + id} {
			if strings.Contains(readme, invalid) {
				t.Errorf("README uses %q instead of the short name", invalid)
			}
			for _, usage := range bookHelp.Usage {
				if strings.HasPrefix(usage, invalid) {
					t.Errorf("help uses %q instead of the short name", usage)
				}
			}
		}
		readProjectFile(t, root, "tests/Test-Book.ps1")
	})

	t.Run("sqlite", func(t *testing.T) {
		path := filepath.ToSlash(filepath.Join(root, filepath.FromSlash(dataPath)))
		uri := url.URL{Scheme: "file", Opaque: (&url.URL{Path: path}).EscapedPath(), RawQuery: "mode=ro"}
		db, err := sql.Open("sqlite3", uri.String())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		var userVersion int
		if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
			t.Fatal(err)
		}
		if userVersion != 1 {
			t.Errorf("PRAGMA user_version = %d, want 1", userVersion)
		}
		var schemaVersion, detailEnabled int
		var title, metaName, desc, contentVersion, dataTable, contentField, lookupKeys string
		err = db.QueryRow(`SELECT schema_version, title, name, description, content_version, data_table,
			detail_enabled, content_field, lookup_keys FROM book_meta WHERE id = 1`).
			Scan(&schemaVersion, &title, &metaName, &desc, &contentVersion, &dataTable, &detailEnabled, &contentField, &lookupKeys)
		if err != nil {
			t.Fatal(err)
		}
		if schemaVersion != 1 || title != displayName || metaName != shortName || desc != description || contentVersion != version {
			t.Errorf("unexpected book_meta schema and identity: %d, %q, %q, %q", schemaVersion, title, desc, contentVersion)
		}
		if dataTable != "entries" || detailEnabled != 1 || contentField != "content" {
			t.Errorf("unexpected book_meta detail settings: %q, %d, %q", dataTable, detailEnabled, contentField)
		}
		type lookupRule struct {
			Option     string
			Fields     []string
			Positional bool
		}
		var rules []lookupRule
		if err := json.Unmarshal([]byte(lookupKeys), &rules); err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(rules, func(rule lookupRule) bool {
			return rule.Option == "--id" && rule.Positional && slices.Equal(rule.Fields, []string{"id"})
		}) {
			t.Errorf("book_meta missing positional ID lookup: %s", lookupKeys)
		}
		var name, content string
		if err := db.QueryRow("SELECT name, content FROM entries WHERE id = 1").Scan(&name, &content); err != nil {
			t.Fatal(err)
		}
		if name != "示例条目" || content != "请替换为你的工具书正文。" {
			t.Errorf("unexpected seed entry: %q, %q", name, content)
		}
	})
}

func readProjectFile(t *testing.T, root, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
