package scaffold

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

//go:embed templates/pack-release.ps1
var packScript []byte

//go:embed templates/check-identity.ps1
var identityScript []byte

//go:embed templates/test-book.ps1
var bookTestScript []byte

//go:embed templates/build.ps1
var buildScript []byte

//go:embed templates/tool-main.go.txt
var toolProgram []byte

//go:embed templates/tool-session.sh
var toolSessionScript []byte

//go:embed templates/go-license.txt
var goNotice []byte

var (
	uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	slugBreak   = regexp.MustCompile(`[^a-z0-9]+`)
)

type Result struct {
	Kind      string
	Directory string
	ID        string
}

type NamedOptions struct {
	Publisher       string
	Kind            string
	ShortName       string
	DisplayName     string
	Description     string
	Version         string
	OutputDirectory string
	Members         []string
}

type manifest struct {
	Schema      int                        `json:"schema"`
	ID          string                     `json:"id"`
	Type        string                     `json:"type"`
	Name        string                     `json:"name"`
	Publisher   string                     `json:"publisher,omitempty"`
	ShortName   string                     `json:"shortName,omitempty"`
	DisplayName string                     `json:"displayName,omitempty"`
	Description *string                    `json:"description,omitempty"`
	Summary     string                     `json:"summary"`
	Version     string                     `json:"version"`
	CoreAPI     int                        `json:"coreApi"`
	Platforms   []string                   `json:"platforms"`
	Help        string                     `json:"help"`
	Changelog   string                     `json:"changelog"`
	Data        string                     `json:"data,omitempty"`
	Library     *libraryContract           `json:"library,omitempty"`
	Provides    map[string]map[string]tool `json:"provides,omitempty"`
	Members     []string                   `json:"members,omitempty"`
}

type example struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type topic struct {
	Summary  string    `json:"summary"`
	Usage    []string  `json:"usage"`
	Examples []example `json:"examples"`
}

type release struct {
	Version string   `json:"version"`
	Date    string   `json:"date"`
	Changes []string `json:"changes"`
}

func Create(kind, directory, name string, members []string) (Result, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(directory)
	}
	if !singleLine(name) {
		return Result{}, errors.New("插件名称必须是有效的单行文本")
	}
	m := manifest{Type: kind, Name: name, Version: "1.0.0"}
	return create(directory, m, slug(filepath.Base(directory)), members)
}

func CreateNamed(options NamedOptions) (Result, error) {
	name, err := NameFor(options.Publisher, options.Kind, options.ShortName)
	if err != nil {
		return Result{}, err
	}
	if !versionRE.MatchString(options.Version) {
		return Result{}, fmt.Errorf("invalid package version %q", options.Version)
	}
	displayName := strings.TrimSpace(options.DisplayName)
	if displayName == "" {
		displayName = name
	}
	if !singleLine(displayName) || !singleLine(options.Description) {
		return Result{}, errors.New("显示名称和描述必须是有效的单行文本")
	}
	kind := options.Kind
	if kind == "tool" {
		kind = "toolplugin"
	}
	if options.OutputDirectory == "" {
		return Result{}, errors.New("输出父目录不能为空")
	}
	m := manifest{
		Type: kind, Name: name, Publisher: options.Publisher, ShortName: options.ShortName,
		DisplayName: displayName, Description: &options.Description, Version: options.Version,
	}
	return create(filepath.Join(options.OutputDirectory, name), m, name, options.Members)
}

func singleLine(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func create(directory string, m manifest, commandSlug string, members []string) (Result, error) {
	if strings.TrimSpace(directory) == "" {
		return Result{}, errors.New("请输入项目目录")
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return Result{}, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return Result{}, fmt.Errorf("项目父目录必须已存在: %w", err)
	}
	dest := filepath.Join(parent, filepath.Base(abs))
	if err := requireAbsent(dest); err != nil {
		return Result{}, err
	}
	id, err := newUUID()
	if err != nil {
		return Result{}, err
	}
	m, topics, err := blueprint(m, id, commandSlug, members)
	if err != nil {
		return Result{}, err
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return Result{}, errors.New("请先将 Git 添加到 PATH")
	}
	stage, err := os.MkdirTemp(parent, ".bmc-scaffold-")
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if stage != "" {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := populate(stage, m, topics); err != nil {
		return Result{}, err
	}
	cmd := exec.Command(git, "init", "--initial-branch=main")
	cmd.Dir = stage
	for _, variable := range os.Environ() {
		key, _, _ := strings.Cut(variable, "=")
		// Keep the new repository and its object store inside the staging directory.
		switch strings.ToUpper(key) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY":
		default:
			cmd.Env = append(cmd.Env, variable)
		}
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("初始化 Git 失败: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := requireAbsent(dest); err != nil {
		return Result{}, err
	}
	if err := os.Rename(stage, dest); err != nil {
		return Result{}, err
	}
	stage = ""
	return Result{Kind: m.Type, Directory: dest, ID: id}, nil
}

func requireAbsent(path string) error {
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return err
		}
		return fmt.Errorf("项目目录已存在: %s", path)
	}
	return nil
}

func newUUID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}

func slug(name string) string {
	slug := strings.Trim(slugBreak.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		return "example"
	}
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	return slug
}

func blueprint(m manifest, id, slug string, members []string) (manifest, map[string]topic, error) {
	m.Schema, m.ID, m.CoreAPI = 1, id, 1
	m.Platforms, m.Help, m.Changelog = []string{"any"}, "help.json", "changelog.json"
	topics := map[string]topic{}
	if m.Type != "bundle" && len(members) != 0 {
		return m, nil, errors.New("--member 仅用于 bundle")
	}
	var usage string
	switch m.Type {
	case "toolplugin":
		m.Summary = "演示全局命令和会话命令。"
		m.Platforms = []string{"windows-amd64"}
		base := "bmc_" + strings.ReplaceAll(slug, "-", "_")
		global, session := base+"_global", base+"_session"
		shellSession := base + "_shell"
		reads := []environmentRead{
			{Name: "BMCSHELL_TARGET_IP", Required: true},
			{Name: "BMCSHELL_TARGET_MODEL"},
		}
		m.Provides = map[string]map[string]tool{"tools": {
			global:       {Scope: "global", Entry: "bin/tool.exe"},
			session:      {Scope: "session", Entry: "bin/tool.exe", EnvReads: reads},
			shellSession: {Scope: "session", Entry: "tool/session.sh", Runtime: "bash", EnvReads: reads},
		}}
		topics["tool."+global] = commandTopic("运行全局示例。", global)
		topics["tool."+session] = commandTopic("运行会话示例并显示当前目标。", session)
		topics["tool."+shellSession] = commandTopic("使用 Bash 读取同一组会话变量。", shellSession)
		usage = global
	case "book":
		m.Summary = "查询 SQLite 工具书中的示例条目。"
		dataName := m.ShortName
		if dataName == "" {
			dataName = slug
		}
		m.Data = "book/" + dataName + ".book.db"
	case "library":
		m.Summary = "可按键查询和同步的 SQLite 数据库示例。"
		m.Data = "library/" + slug + ".library.db"
		m.Library = &libraryContract{
			SchemaVersion: 1,
			Tables:        []libraryTable{{Name: "records", Keys: []string{"key"}}},
			Lookup:        libraryLookup{Table: "records", Value: "value"},
		}
		usage = "bmc library query " + id + " DEMO01"
	case "theme":
		m.Summary = "显示当前 BMC 目标的提示符主题示例。"
		m.Data = "themes/theme.json"
		usage = "bmc setting --set theme.active " + id
	case "bundle":
		m.Summary = "按 UUID 组合多个已安装的扩展。"
		if len(members) == 0 {
			return m, nil, errors.New("bundle 至少需要一个 --member UUID")
		}
		seen := map[string]bool{}
		for _, member := range members {
			member = strings.ToLower(member)
			if !uuidPattern.MatchString(member) || seen[member] {
				return m, nil, fmt.Errorf("成员 UUID 无效或重复: %q", member)
			}
			seen[member] = true
			m.Members = append(m.Members, member)
		}
		usage = "bmc plugin show " + id
	default:
		return m, nil, fmt.Errorf("未知插件类型 %q", m.Type)
	}
	if m.Description != nil && strings.TrimSpace(*m.Description) != "" {
		m.Summary = *m.Description
	}
	if m.Type == "book" {
		topics["package"] = bookTopic(m)
	} else {
		topics["package"] = commandTopic(m.Summary, usage)
	}
	return m, topics, nil
}

func commandTopic(summary, usage string) topic {
	return topic{Summary: summary, Usage: []string{usage}, Examples: []example{{Command: usage, Description: summary}}}
}

func bookTopic(m manifest) topic {
	name := m.ShortName
	if name == "" {
		// Legacy packages use the full manifest name, which may contain shell metacharacters.
		name = "'" + strings.ReplaceAll(m.Name, "'", "'\"'\"'") + "'"
	}
	command := "bmc book " + name
	examples := []example{
		{Command: command + " -l", Description: "按 ID 列出条目。"},
		{Command: command + " -s '示例'", Description: "按名称搜索示例条目。"},
		{Command: command + " --id 1", Description: "按 ID 查看正文。"},
		{Command: command + " 1", Description: "用位置参数按 ID 查看正文。"},
		{Command: command + " --name '示例条目'", Description: "按完整名称查看正文。"},
	}
	result := topic{Summary: m.Summary, Examples: examples}
	for _, item := range examples {
		result.Usage = append(result.Usage, item.Command)
	}
	return result
}

func populate(root string, m manifest, topics map[string]topic) error {
	files := map[string][]byte{
		"README.md":                  []byte(readme(m, topics["package"].Usage)),
		".gitignore":                 []byte("/dist/\n/bin/\n"),
		"scripts/pack-release.ps1":   packScript,
		"scripts/check-identity.ps1": identityScript,
	}
	if m.Type == "book" {
		files["tests/Test-Book.ps1"] = bookTestScript
	}
	if m.Type == "toolplugin" {
		files[".gitattributes"] = []byte("*.sh text eol=lf\n")
		files["go.mod"] = []byte("module bmcshell-tool-example\n\ngo 1.22\n")
		files["cmd/tool/main.go"] = toolProgram
		files["tool/session.sh"] = toolSessionScript
		files["THIRD_PARTY_NOTICES.txt"] = goNotice
		files["scripts/build.ps1"] = buildScript
	}
	for rel, data := range files {
		if err := write(root, rel, data); err != nil {
			return err
		}
	}
	if err := writeJSON(root, "bmc-package.json", m); err != nil {
		return err
	}
	if err := writeJSON(root, m.Help, map[string]any{"topics": topics}); err != nil {
		return err
	}
	history := []release{{Version: m.Version, Date: time.Now().Format("2006-01-02"), Changes: []string{"创建插件脚手架。"}}}
	if err := writeJSON(root, m.Changelog, map[string]any{"releases": history}); err != nil {
		return err
	}
	switch m.Type {
	case "book", "library":
		path := filepath.Join(root, filepath.FromSlash(m.Data))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if m.Type == "library" {
			return createLibrary(path, m.ID)
		}
		return createBook(path, m)
	case "theme":
		return writeJSON(root, m.Data, map[string]string{"prefix": "[BMC {ip}] "})
	}
	return nil
}

func writeJSON(root, rel string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return write(root, rel, append(data, '\n'))
}

func write(root, rel string, data []byte) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readme(m manifest, commands []string) string {
	var out strings.Builder
	title := m.Name
	if m.DisplayName != "" {
		title = m.DisplayName
	}
	fmt.Fprintf(&out, "# %s\n\n%s\n\n", title, m.Summary)
	if m.ShortName != "" {
		fmt.Fprintf(&out, "包名 `%s`。\n\n", m.Name)
	}
	fmt.Fprintf(&out, "包 ID `%s`。\n\n## 打包与使用\n\n", m.ID)
	if m.Type == "toolplugin" {
		out.WriteString("在配置好 Go 的 Windows x64 环境中构建工具。\n\n```powershell\npwsh -File ./scripts/build.ps1\n```\n\n")
	}
	if m.Type == "book" {
		fmt.Fprintf(&out, "使用 SQLite 工具编辑 `%s`。保留 `PRAGMA user_version = 1` 和 `book_meta` 中 `id = 1`、`schema_version = 1` 的唯一一行。示例数据位于 `entries` 表。修改查询字段时同步更新 `book_meta`。\n\n", m.Data)
		out.WriteString("在 PowerShell 中运行工具书测试。测试使用独立的 `BMCSHELL_HOME` 安装并验证示例查询。替换 SQLite 示例数据后，同步更新 `tests/Test-Book.ps1` 中的预期结果。\n\n```powershell\npwsh -File ./tests/Test-Book.ps1 -BmcExe 'D:\\tools\\BMCShell.exe'\n```\n\n")
	}
	if m.Type == "library" {
		fmt.Fprintf(&out, "使用 SQLite 工具编辑 `%s`。包清单的 `library` 字段声明结构版本、参与同步的表和唯一键，以及按键查询的表和返回列。\n\n", m.Data)
	}
	if m.Type == "bundle" {
		out.WriteString("先安装 `bmc-package.json` 中声明的成员包。\n\n")
	}
	out.WriteString("从仓库根目录在 PowerShell 中打包。\n\n```powershell\n./scripts/pack-release.ps1\n```\n\n")
	fmt.Fprintf(&out, "在已加载 BMCShell 的 Bash 中安装当前目录并验证。\n\n```bash\nbmc plugin install --file .\n%s\n```\n\n", strings.Join(commands, "\n"))
	if m.Type == "toolplugin" {
		base := strings.TrimSuffix(commands[0], "_global")
		out.WriteString("安装工具后重新加载 Bash 适配层，使全局命令生效。选定目标后运行两种会话示例。\n\n")
		fmt.Fprintf(&out, "```bash\nbmc use 192.0.2.10\n%s_session\n%s_shell\nbmc exit\n```\n\n", base, base)
		out.WriteString("会话 EXE（`_session`）和 Bash（`_shell`）通过清单中的 `envReads` 声明相同的依赖。框架提供必选的 `BMCSHELL_TARGET_IP` 和可选的 `BMCSHELL_TARGET_MODEL`。如需在成功结束后向框架提交会话变量，使用 `envWrites` 声明写入范围。\n\n")
	}
	out.WriteString("发布前由权利人确认许可，选定后将 `LICENSE` 放在仓库根目录。\n")
	return out.String()
}
