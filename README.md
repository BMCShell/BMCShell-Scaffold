# BMCShell Scaffold

为 BMCShell 3.0 生成扩展仓库，支持工具、工具书、数据仓、主题和捆绑包。脚手架可以独立构建，生成的扩展可安装到使用 Core API 1 的 BMCShell。

## 构建

安装 Go 1.26 或更新版本以及支持 `git init --initial-branch` 的 Git。从本仓库根目录运行以下命令。

```powershell
go test ./...
go build -trimpath -o BMCShell-Scaffold.exe ./cmd/bmc-scaffold
```

## 创建扩展

```powershell
.\BMCShell-Scaffold.exe create --name bmcshell-book-redfish --display-name 'Redfish 工具书'
.\BMCShell-Scaffold.exe create --publisher bmcshell --type tool --short-name diagnostics
.\BMCShell-Scaffold.exe create -o D:\projects
```

`create` 在项目生成后自动初始化独立的 Git 仓库，初始分支为 `main`。生成的文件可供首次提交。项目还包含包清单、帮助、更新记录和打包脚本。Book 与 Library 模板会创建可查询的 SQLite 文件。Book 数据库使用 `book/<shortName>.book.db`，例如 `bmcshell-book-redfish` 的数据文件是 `book/redfish.book.db`。工具模板包含 Windows x64 构建脚本。用 `create --help` 查看所有选项。

## 测试 Book 项目

使用 PowerShell 7.4 或更新版本，从生成的 Book 项目根目录运行以下命令。将路径换成本机 BMCShell 的可执行文件路径。

```powershell
pwsh -File ./tests/Test-Book.ps1 -BmcExe 'D:\tools\BMCShell.exe'
```

脚本在临时 `BMCSHELL_HOME` 中安装项目，检查工具书列表、短名帮助、示例检索、详情和版本。替换示例条目后，同步更新测试数据和查询参数。

## 打包脚手架

```powershell
pwsh -File ./scripts/pack-release.ps1 -Version 3.0.0-dev.5
```

Windows x64 归档与 SHA256 清单默认输出到 `dist/`。第三方许可见 [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt)。项目许可证由权利人确认。
