package scaffold

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

const bookSchema = `
CREATE TABLE book_meta (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  schema_version INTEGER NOT NULL CHECK (schema_version = 1),
  title TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  content_version TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  data_table TEXT NOT NULL,
  list_fields TEXT NOT NULL,
  search_fields TEXT NOT NULL,
  detail_enabled INTEGER NOT NULL CHECK (detail_enabled IN (0,1)),
  lookup_keys TEXT NOT NULL,
  content_field TEXT,
  content_format TEXT NOT NULL DEFAULT 'plain',
  field_options TEXT NOT NULL DEFAULT '{}',
  order_by TEXT NOT NULL,
  help_text TEXT NOT NULL DEFAULT ''
);
CREATE TABLE entries (id INTEGER PRIMARY KEY, name TEXT NOT NULL, content TEXT NOT NULL DEFAULT '');
PRAGMA user_version = 1;
`

const librarySchema = `
CREATE TABLE _bmcshell_library (
  singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
  id TEXT NOT NULL,
  schema_version INTEGER NOT NULL,
  data_version TEXT NOT NULL
);
CREATE TABLE records (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL);
PRAGMA user_version = 1;
`

func createBook(path string, m manifest) error {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(path)), ".book.db")
	title := m.DisplayName
	if title == "" {
		title = m.Name
	}
	return createSQLite(path, bookSchema, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO book_meta (id, schema_version, title, name, description,
			content_version, updated_at, data_table, list_fields, search_fields, detail_enabled,
			lookup_keys, content_field, field_options, order_by)
			VALUES (1, 1, ?, ?, ?, ?, ?, 'entries', '["id","name"]', '["name"]', 1,
			'[{"name":"id","option":"--id","fields":["id"],"positional":true},{"name":"name","option":"--name","fields":["name"]}]',
			'content', '{"id":{"title":"ID","format":"integer","width":6},"name":{"title":"名称","width":48}}', '["id"]')`,
			title, name, m.Summary, m.Version, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO entries (id, name, content) VALUES (1, ?, ?)", "示例条目", "请替换为你的工具书正文。")
		return err
	})
}

func createLibrary(path, id string) error {
	return createSQLite(path, librarySchema, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO _bmcshell_library (singleton, id, schema_version, data_version) VALUES (1, ?, 1, '1')", id)
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO records (key, value) VALUES (?, ?)", "DEMO01", "示例设备 A")
		return err
	})
}

func createSQLite(path, schema string, seed func(*sql.Tx) error) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if err := f.Close(); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	uri := url.URL{
		Scheme:   "file",
		Opaque:   (&url.URL{Path: filepath.ToSlash(abs)}).EscapedPath(),
		RawQuery: "mode=rw",
	}
	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); err == nil {
			err = closeErr
		}
	}()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schema); err != nil {
		return err
	}
	if err := seed(tx); err != nil {
		return err
	}
	return tx.Commit()
}
