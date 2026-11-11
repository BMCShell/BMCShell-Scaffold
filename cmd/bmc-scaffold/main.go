package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bmcshell-scaffold/internal/scaffold"
)

type members []string

func (m *members) String() string { return fmt.Sprint([]string(*m)) }
func (m *members) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "说明")
	fmt.Fprintln(w, "  创建 BMCShell 插件项目，生成包清单、帮助和示例文件。")
	fmt.Fprintln(w, "\n用法")
	fmt.Fprintln(w, "  BMCShell-Scaffold create [-o <父目录>]")
	fmt.Fprintln(w, "  BMCShell-Scaffold create <类型> <项目目录> [--name <显示名>] [--member <UUID> ...]")
	fmt.Fprintln(w, "  BMCShell-Scaffold create --name <完整包名> [选项]")
	fmt.Fprintln(w, "  BMCShell-Scaffold create --publisher <发布方> --type <book|library|theme|tool|bundle> --short-name <短名> [选项]")
	fmt.Fprintln(w, "\n参数与默认值")
	fmt.Fprintln(w, "  --name <完整包名>       格式为 发布方-类型-短名。旧式位置参数用法中指定显示名。")
	fmt.Fprintln(w, "  --publisher <发布方>    小写英文字母开头，后续可含小写字母和数字。")
	fmt.Fprintln(w, "  --type <类型>           选择 book、library、theme、tool 或 bundle。")
	fmt.Fprintln(w, "  --short-name <短名>     使用 1 到 48 个小写英文字母、数字或单个连字符。")
	fmt.Fprintln(w, "  --display-name <显示名>  默认使用完整包名")
	fmt.Fprintln(w, "  --description <描述>    指定插件用途，支持中文。")
	fmt.Fprintln(w, "  --version <版本>         默认 1.0.0")
	fmt.Fprintln(w, "  -o, --output <父目录>    默认当前目录")
	fmt.Fprintln(w, "  --member <UUID>         为 bundle 指定成员，可重复指定，至少提供一个。")
	fmt.Fprintln(w, "  -h, --help              显示帮助。")
	fmt.Fprintln(w, "\n示例")
	fmt.Fprintln(w, "  启动交互式向导")
	fmt.Fprintln(w, "  BMCShell-Scaffold create")
	fmt.Fprintln(w, "  创建示例工具书项目")
	fmt.Fprintln(w, "  BMCShell-Scaffold create --name demo-book-api --display-name \"API 工具书\"")
	fmt.Fprintln(w, "\n使用说明")
	fmt.Fprintln(w, "  --name 与 --publisher、--type、--short-name 两种命名方式任选其一。")
	fmt.Fprintln(w, "  需要在 PATH 中安装 Git。输出父目录须已存在，目标项目目录须可创建。")
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h")) {
		usage(stdout)
		return 0
	}
	if len(args) == 2 && args[0] == "create" && (args[1] == "--help" || args[1] == "-h") {
		usage(stdout)
		return 0
	}
	if args[0] != "create" {
		return argumentError(stderr, fmt.Sprintf("未知命令 %q。", args[0]))
	}
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		return runLegacy(args[1:], stdout, stderr)
	}
	return runNamed(args[1:], stdin, stdout, stderr)
}

func runNamed(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { usage(stdout) }
	var options scaffold.NamedOptions
	name := flags.String("name", "", "完整包名")
	flags.StringVar(&options.Publisher, "publisher", "", "发布方")
	flags.StringVar(&options.Kind, "type", "", "扩展类型")
	flags.StringVar(&options.ShortName, "short-name", "", "功能短名")
	flags.StringVar(&options.DisplayName, "display-name", "", "显示名称")
	flags.StringVar(&options.Description, "description", "", "用途描述")
	flags.StringVar(&options.Version, "version", "1.0.0", "版本")
	flags.StringVar(&options.OutputDirectory, "output", ".", "输出父目录")
	flags.StringVar(&options.OutputDirectory, "o", ".", "输出父目录")
	var memberIDs members
	flags.Var(&memberIDs, "member", "Bundle 成员的 UUID，可重复指定")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return flagError(stderr, err)
	}
	if flags.NArg() != 0 {
		return argumentError(stderr, fmt.Sprintf("无法识别多余的位置参数 %q。", flags.Arg(0)))
	}
	options.Members = memberIDs
	specified := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { specified[f.Name] = true })
	hasParts := specified["publisher"] || specified["type"] || specified["short-name"]
	switch {
	case specified["name"] && hasParts:
		fmt.Fprintln(stderr, "BMCShell-Scaffold: --name 与 --publisher、--type、--short-name 互斥")
		return 2
	case specified["name"]:
		var err error
		options.Publisher, options.Kind, options.ShortName, err = scaffold.ParseName(*name)
		if err != nil {
			fmt.Fprintln(stderr, "BMCShell-Scaffold:", err)
			return 2
		}
	case hasParts:
		if !specified["publisher"] || !specified["type"] || !specified["short-name"] {
			fmt.Fprintln(stderr, "BMCShell-Scaffold: 请同时指定 --publisher、--type 和 --short-name")
			return 2
		}
		if !supportedType(options.Kind) {
			fmt.Fprintln(stderr, "BMCShell-Scaffold: --type 只接受 book、library、theme、tool 或 bundle")
			return 2
		}
	default:
		delete(specified, "o")
		delete(specified, "output")
		if len(specified) != 0 {
			fmt.Fprintln(stderr, "BMCShell-Scaffold: 请指定包名或完整的命名字段")
			return 2
		}
		confirmed, err := interactive(stdin, stdout, &options)
		if errors.Is(err, io.ErrUnexpectedEOF) {
			fmt.Fprintln(stderr, "输入已结束，无法完成创建向导。")
			return 1
		}
		if err != nil {
			fmt.Fprintln(stderr, "BMCShell-Scaffold 创建向导失败。", err)
			return 1
		}
		if !confirmed {
			fmt.Fprintln(stdout, "已取消创建。")
			return 0
		}
	}
	result, err := scaffold.CreateNamed(options)
	return report(result, err, stdout, stderr)
}

func supportedType(kind string) bool {
	switch kind {
	case "book", "library", "theme", "tool", "bundle":
		return true
	}
	return false
}

func interactive(stdin io.Reader, out io.Writer, options *scaffold.NamedOptions) (bool, error) {
	scanner := bufio.NewScanner(stdin)
	prompt := func(label string) (string, error) {
		fmt.Fprint(out, label)
		if scanner.Scan() {
			return strings.TrimSpace(scanner.Text()), nil
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.ErrUnexpectedEOF
	}
	for _, field := range []struct {
		label string
		value *string
	}{
		{"发布方：", &options.Publisher},
		{"类型 [book/library/theme/tool/bundle]：", &options.Kind},
		{"功能短名：", &options.ShortName},
		{"显示名称（留空使用完整包名）：", &options.DisplayName},
		{"用途描述（可选）：", &options.Description},
	} {
		value, err := prompt(field.label)
		if err != nil {
			return false, err
		}
		*field.value = value
	}
	if !supportedType(options.Kind) {
		return false, fmt.Errorf("不支持的扩展类型 %q", options.Kind)
	}
	name, err := scaffold.NameFor(options.Publisher, options.Kind, options.ShortName)
	if err != nil {
		return false, err
	}
	if options.Kind == "bundle" {
		value, err := prompt("成员 UUID（用空格分隔）：")
		if err != nil {
			return false, err
		}
		options.Members = strings.Fields(value)
	}
	path, err := filepath.Abs(filepath.Join(options.OutputDirectory, name))
	if err != nil {
		return false, err
	}
	fmt.Fprintf(out, "\n包名 %s\n目标目录 %s\n", name, path)
	confirm, err := prompt("确认创建 [y/N]：")
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return false, nil
	}
	return strings.EqualFold(confirm, "y") || strings.EqualFold(confirm, "yes"), err
}

func runLegacy(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return argumentError(stderr, "请同时指定插件类型和项目目录。")
	}
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { usage(stdout) }
	name := flags.String("name", "", "插件显示名称，默认使用目录名")
	var memberIDs members
	flags.Var(&memberIDs, "member", "Bundle 成员的 UUID，可重复指定")
	if err := flags.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return flagError(stderr, err)
	}
	if flags.NArg() != 0 {
		return argumentError(stderr, fmt.Sprintf("无法识别多余的位置参数 %q。", flags.Arg(0)))
	}
	result, err := scaffold.Create(args[0], args[1], *name, memberIDs)
	return report(result, err, stdout, stderr)
}

func flagError(out io.Writer, err error) int {
	message := err.Error()
	if name, ok := strings.CutPrefix(message, "flag provided but not defined: "); ok {
		return argumentError(out, "未知选项 "+name+"。")
	}
	if name, ok := strings.CutPrefix(message, "flag needs an argument: "); ok {
		return argumentError(out, "选项 "+name+" 缺少参数值。")
	}
	if option, ok := strings.CutPrefix(message, "bad flag syntax: "); ok {
		return argumentError(out, "选项格式无效 "+option+"。")
	}
	return argumentError(out, "无法解析命令选项。")
}

func argumentError(out io.Writer, message string) int {
	fmt.Fprintln(out, message+" 运行 BMCShell-Scaffold create --help 查看用法。")
	return 2
}

func report(result scaffold.Result, err error, stdout, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintln(stderr, "BMCShell-Scaffold:", err)
		return 1
	}
	fmt.Fprintf(stdout, "已创建 %s 插件\n目录 %s\nID %s\n", result.Kind, result.Directory, result.ID)
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
