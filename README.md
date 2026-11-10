# BMCShell Scaffold

为 BMCShell 3.0 生成扩展仓库，支持工具、工具书、数据仓、主题和捆绑包。脚手架可以独立构建，生成的扩展可安装到使用 Core API 1 的 BMCShell。

## 构建

安装 Go 1.26 或更新版本以及 Git。从本仓库根目录运行以下命令。

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

每个扩展生成自己的 Git 仓库、包清单、帮助、更新记录和打包脚本。Book 与 Library 模板会创建可查询的 SQLite 文件。工具模板包含 Windows x64 构建脚本。用 `create --help` 查看所有选项。

## 打包脚手架

```powershell
pwsh -File ./scripts/pack-release.ps1 -Version 3.0.0-dev.3
```

Windows x64 归档与 SHA256 清单默认输出到 `dist/`。第三方许可见 [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt)。项目许可证由权利人确认。
