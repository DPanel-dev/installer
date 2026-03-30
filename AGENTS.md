# Repository Guidelines

## 项目结构与模块组织
本仓库是 DPanel 的 Go 安装器，支持两种入口模式：无参数进入 TUI，有参数进入 CLI。
- `main.go`：程序入口、模式分发与 `run.log` 初始化。
- `internal/config/`：配置模型、环境检测辅助方法、`With*` 选项函数。
- `internal/core/`：安装引擎与执行流程。
- `internal/handler/cli/`：命令解析与参数映射。
- `internal/handler/tui/`：Bubble Tea 交互流程与步骤渲染。
- `internal/types/`：动作、版本、容器连接等共享常量与类型。
- `pkg/i18n/`：内嵌语言包（`translations/en.json`、`translations/zh.json`）。
- `internal/script/`、`shell/`：安装辅助脚本。
- `runtime/`：构建产物目录。

## 构建、测试与开发命令
- `go build -o runtime/dpanel-installer main.go`：构建本地二进制。
- `go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date +%F)" -o runtime/dpanel-installer main.go`：注入版本信息构建。
- `go run .`：直接运行（默认 TUI）。
- `go run . --help`：验证 CLI 命令与参数。
- `go test ./...`：运行单元测试。
- `go test -race ./...`：运行竞态检测（提交前建议执行）。
- `go vet ./...` 与 `golangci-lint run ./...`：执行静态检查。

## 代码风格与命名约定
遵循 Go 官方格式与惯例。
- 提交前执行 `gofmt`（或 `go fmt ./...`）。
- 包名使用小写短词；导出标识符使用 `CamelCase`。
- 配置变更优先使用 `With*` 选项函数。
- 优先复用 `internal/types/` 常量，避免硬编码枚举字符串。
- `main.go` 只负责编排流程，业务逻辑放在 `internal/*`。
- `cli.go`、`tui.go` 等入口文件仅保留流程控制；解析、校验、归一化等辅助逻辑下沉到 `helpers.go`。

## 测试约定
当前仓库暂无 `*_test.go` 文件。新增功能或行为变更时应补充测试。
- 测试文件命名为 `*_test.go`，测试函数命名为 `TestXxx`。
- 配置解析、命令构建优先采用表驱动测试。
- 最低要求执行 `go test ./...`；涉及核心流程建议再跑 `go test -race ./...`。

## 提交与 Pull Request 规范
提交历史遵循 Conventional Commits（如 `feat(tui): ...`、`refactor(config): ...`、`fix(cli): ...`）。
- 格式：`<type>(<scope>): <subject>`。
- 常见范围：`tui`、`cli`、`config`、`core`、`types`、`docs`。
- 每次提交聚焦单一目标，避免混入无关改动。
- PR 应包含：变更摘要、影响模块、已执行的测试/检查命令；若涉及 TUI，请附关键终端截图或录屏。

## Agent 协作约定
- Issue、评审与 PR 讨论默认使用中文。
- 代码标识符、CLI 参数、commit type 使用英文原文，说明文字使用中文。
- 当全局规范变更时，同步等价规则到仓库规范，并提供影响范围与变更摘要。
- 在进行任何代码修改前，必须先提出实现方案并等待用户确认；未经确认不得直接修改文件。
- 进行任何代码组织与功能设计时，必须优先采用分层递进、职责清晰、默认行为优先的方式：先走默认路径，再按明确层级逐步回退或补充探测；用户传入参数仅用于覆盖或附加，不能替代默认流程；禁止把多层探测、状态中间层和过度抽象混在一起。
- 每个模块组都必须建立独立的 `helpers` 文件承接辅助函数；主文件只保留核心结构、主流程和对外入口，禁止把辅助逻辑继续堆在主文件中。
- 当当前配置或流程上下文中已经持有明确的 SDK / Client 对象时，后续执行阶段必须优先直接复用该对象；禁止重新回退到 shell command、重新探测运行时或重新选择命令路径。对于本仓库的容器执行逻辑，若已持有 `cfg.Client`，则必须使用 Docker SDK，禁止再以 `docker` / `podman` CLI 作为主实现。
- 每次代码修改完成后，必须以构建作为收尾验证，默认执行 `go build -o runtime/dpanel-installer main.go`；`go test ./...` 仅在任务本身需要测试验证时额外执行。
- TUI 底部帮助信息统一通过 `renderHelp()` 渲染；成功页支持按任意键退出，错误页保留 `Ctrl+C` 退出以便查看错误信息。
- 无论任何步骤或状态，底部帮助栏都必须统一由 `renderHelp()` 输出；禁止在 `renderContent()` 或其它分支中额外渲染独立底部帮助文案。
- 禁止使用会遮蔽导入包名的局部变量名，尤其避免 `path`、`json`、`http`、`time` 等标准库或常用包名与变量同名；命名应优先保证可读性，必要时为导入包显式起别名。
