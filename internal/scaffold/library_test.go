package scaffold

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLibraryProjectTemplate(t *testing.T) {
	const (
		name    = "bmcshell-library-device-map"
		id      = "11111111-1111-4111-8111-111111111111"
		command = "bmc library device-map"
	)
	m, topics, err := blueprint(manifest{
		Type: "library", Name: name, Publisher: "bmcshell", ShortName: "device-map", Version: "1.0.0",
	}, id, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := populate(root, m, topics); err != nil {
		t.Fatal(err)
	}

	var generated struct {
		Data    string `json:"data"`
		Library struct {
			libraryContract
			Lookup json.RawMessage `json:"lookup"`
		} `json:"library"`
	}
	if err := json.Unmarshal(readProjectFile(t, root, "bmc-package.json"), &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Library.Lookup) != 0 || len(generated.Library.Tables) != 1 {
		t.Fatalf("expected one Library table and no lookup: %+v", generated.Library)
	}
	table := generated.Library.Tables[0]
	if table.Name != "records" || !slices.Equal(table.Keys, []string{"device_id"}) ||
		!slices.Equal(table.ViewColumns, []string{"device_id", "display_name"}) {
		t.Fatalf("unexpected table definition: %+v", table)
	}
	for _, column := range table.ViewColumns {
		field := table.Fields[column]
		if field.Description == "" || field.Type != "text" || !field.Required {
			t.Errorf("expected a description and required text type for %s: %+v", column, field)
		}
	}

	var help struct {
		Topics map[string]topic `json:"topics"`
	}
	if err := json.Unmarshal(readProjectFile(t, root, "help.json"), &help); err != nil {
		t.Fatal(err)
	}
	if usage := help.Topics["package"].Usage; !slices.Equal(usage, []string{command}) {
		t.Errorf("unexpected Library help usage: %v", usage)
	}
	if readme := string(readProjectFile(t, root, "README.md")); !strings.Contains(readme, "\n"+command+"\n") {
		t.Errorf("generated README is missing %q", command)
	}

	path := filepath.ToSlash(filepath.Join(root, filepath.FromSlash(generated.Data)))
	uri := url.URL{Scheme: "file", Opaque: (&url.URL{Path: path}).EscapedPath(), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var deviceID, displayName string
	err = db.QueryRow("SELECT device_id, display_name FROM records").Scan(&deviceID, &displayName)
	if err != nil {
		t.Fatal(err)
	}
	if deviceID != "DEMO01" || displayName != "示例设备 A" {
		t.Errorf("unexpected seed record: %q, %q", deviceID, displayName)
	}
}
