# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此代码库中工作时提供指导。

## 项目概述

DPanel 安装器 - 用于安装、升级和卸载 DPanel 容器和二进制文件的工具。安装器支持两种模式：
- **TUI 模式**：默认启动的交互式终端 UI 向导（无参数时）
- **CLI 模式**：通过命令行标志进行程序化调用（有参数时）

**架构原则**：
1. **单一职责**：main.go 只负责根据参数分发到合适的配置源
2. **配置驱动**：TUI 和 CLI 都修改同一个 Config 对象
3. **统一执行**：所有配置源最终都通过 Config.Run() 执行安装
4. **可扩展**：通过 ConfigSource 接口可以轻松添加新的配置源（如配置文件、API 等）

## 构建命令

根据当前系统构建到 runtime 目录下

```bash
# 构建
go build -o runtime/dpanel-installer main.go

# 构建带版本信息
go build -ldflags "-X main.version=1.0.0 -X main.commit=abc123 -X main.date=2025-03-22" \
  -o runtime/dpanel-installer main.go
```

## 使用示例

### TUI 模式（默认）
```bash
# 无参数：启动交互式 TUI 向导
./dpanel-installer

# 显式指定 TUI 模式
./dpanel-installer --mode tui
```

### CLI 模式
```bash
# 显示帮助
./dpanel-installer --help
./dpanel-installer -h

# 显示版本
./dpanel-installer --version
./dpanel-installer -v

# 快速安装（使用默认值）
./dpanel-installer --action config

# 完整配置安装
./dpanel-installer \
  --action config \
  --dpanel-version community \
  --edition lite \
  --config-type container \
  --port 8080 \
  --container-name my-dpanel

# 强制 CLI 模式（即使只有一个参数）
./dpanel-installer --mode cli --action config
```

## 架构

### 入口点 (`main.go`)

**单一职责：分发器** - main.go 只负责根据参数选择合适的配置源，不处理任何业务逻辑。

#### 行为逻辑

```
无参数 → TUI 模式（默认）
有参数 → CLI 模式（如 --help, --version, --action 等）
--mode tui → 强制 TUI 模式
--mode cli → 强制 CLI 模式
```

#### 源选择流程

1. **检查显式 --mode 参数**
   - `--mode tui` → TUI 配置源
   - `--mode cli` → CLI 配置源

2. **检查各配置源的 CanHandle()**
   - CLI.CanHandle(): 有任何参数时返回 true（除了 `--mode tui`）
   - TUI.CanHandle(): 只在 `--mode tui` 时返回 true

3. **返回默认配置源**
   - 默认：TUI 配置源

#### 实现要点

- **简洁性**：main.go 只有约 60 行代码
- **单一职责**：只负责分发，不处理业务逻辑
- **日志记录**：使用 `slog` 将 JSON 日志写入程序同级目录下的 `run.log`
- **版本信息**：通过 ldflags 注入，传递给 CLI 包用于显示

#### 代码示例

```go
func run() error {
    args := os.Args[1:]

    // Create default Config
    cfg, err := install.NewConfig()
    if err != nil {
        return err
    }

    // Initialize source registry with TUI as default
    registry := source.NewRegistry()
    registry.RegisterDefault(tui.NewTUIConfigSource()) // TUI is default
    registry.Register(cli.NewCLI())

    // Detect and run appropriate source
    selectedSource := registry.Detect(args)
    return selectedSource.Run(cfg, args)
}
```

### 核心组件

**`internal/install/constants.go`**：配置常量定义
- 所有配置值的命名常量（操作类型、安装类型、版本、版本类型、系统、镜像源、Docker 连接类型等）
- 常量验证函数（`IsValidAction()`, `IsValidInstallType()` 等）
- 有效值数组（`ValidActions`, `ValidInstallTypes` 等）
- **重要**：所有代码必须使用这些常量，禁止硬编码字符串字面量

**`internal/install/config.go`**：安装配置
- `Config` 结构体包含所有安装配置项（扁平化设计，无嵌套指针）
- `EnvCheck` 结构体包含环境检测结果
- `NewConfig()` 函数：执行环境检测 + 应用智能默认值 + 应用用户选项
- `detectEnvironment()` 方法：自动检测系统信息、Docker/Podman、网络连通性、现有安装
- `applySmartDefaults()` 方法：根据环境设置最优默认值
- `GetImageName()` 方法：根据配置构建完整的镜像名称
- `Run()` 方法：验证配置并执行安装

**`internal/install/options.go`**：配置选项模式（Option Pattern）
- `Option` 类型：`func(*Config) error`
- 所有 `With*()` 函数用于修改配置
- 内置验证逻辑，确保配置值的有效性和兼容性
- 支持链式调用：`install.NewConfig(install.WithAction(...), install.WithVersion(...))`

**`internal/install/engine.go`**：安装引擎
- `Engine` 结构体包装 `Config` 并执行操作
- `Run()` 根据 `Config.Action` 分发到 `install()`、`upgrade()` 或 `uninstall()`
- **改进的环境检测**：
  - `checkDocker()`：不仅检测命令存在，还测试服务是否真正可用
  - `testDockerService()`：执行 `docker ps` 命令测试服务，带 5 秒超时
  - 区分"命令存在"和"服务可用"两种情况
- **完善的日志记录**：
  - `logInstallationConfig()`：记录完整配置信息
  - `logInstallationSteps()`：记录安装步骤
  - `saveInstallationLog()`：保存安装日志到 `install_logs/install_YYYYMMDD_HHMMSS.log`
  - `saveInstallationResult()`：记录安装结果
- `buildDockerCommand()`：构建 docker/podman run 命令
- `buildImageName()`：根据配置构建镜像名称
- `executeCommand()`：执行 shell 命令

**`internal/cli/cli.go`**：CLI 配置源
- 实现 `source.ConfigSource` 接口
- **CanHandle()**: 有任何参数时返回 true（除了 `--mode tui`）
- **Run()**: 解析命令行参数，应用到 Config，执行安装
- **特殊处理**:
  - `--help` / `-h`: 显示帮助信息后退出
  - `--version` / `-v`: 显示版本信息后退出
- **标志定义**:
  - `--action`: 操作类型（install, upgrade, uninstall）
  - `--dpanel-version`: DPanel 版本（community, pro, dev）
  - `--edition`: 版本类型（standard, lite）
  - `--install-type`: 安装类型（container, binary）
  - `--docker-type`: Docker 连接类型（local, tcp, ssh）
  - 以及其他配置选项
- **所有帮助信息使用英文**

**`internal/handler/tui/tui.go`**：Bubble Tea TUI 实现
- **全屏显示**：无边框，使用整个终端屏幕
- **DPanel 配色方案**：
  - 主色：`#1890FF` (DPanel 蓝色)
  - 成功：`#52C41A` (成功绿色)
  - 警告：`#FAAD14` (警告琥珀色)
  - 错误：`#FF4D4F` (错误红色)
  - 背景：`#141414` (深色背景)
  - 选中：`#0050B3` (选中蓝色)
  - 文本：`#E8E8E8` (主文本色)
- **DPANEL Logo**：使用 Unicode 块字符构建的 ASCII 艺术字
  - 在语言选择页面显示
  - 使用 DPanel 蓝色主题
  - 正确拼写 "DPANEL" 六个字母
- **Step 类型**：定义所有安装步骤
- **model 结构体**：保存 TUI 状态
- **真实的环境检测**：
  - 检测命令存在：`exec.LookPath("docker")`
  - 测试服务可用：执行 `docker ps` 验证
  - 区分本地容器和远程连接的场景
- `setupXXXChoices()` 方法：为每个步骤设置选项
- `renderMenu()` / `renderInput()`：渲染界面
- `handleEnter()`：处理回车键，控制步骤流转
- **只在 TUI 模式加载语言包**

**`pkg/i18n/i18n.go`**：国际化系统
- 使用 `//go:embed` 捆绑翻译 JSON 文件
- 使用 `encoding/json` 解析到内存
- `T()` 简单翻译，`Tf()` 格式化翻译
- **只在 TUI 模式使用**，CLI 模式不加载

## 环境检测逻辑

### 问题说明
仅仅检测 `docker` 命令存在不足以证明可以使用容器安装：
- 远程 Docker 连接：本地有 docker 命令但连接远程服务，无法在本地部署容器
- Docker 服务未运行：命令存在但 daemon 没有启动

### 检测流程
1. **命令检测**：检查 `docker` 或 `podman` 命令是否在 PATH 中
2. **服务检测**：执行 `docker ps` 测试服务是否真正可用
3. **连接类型检测**：
   - Local：要求本地 Docker 服务可用
   - TCP/SSH：远程连接，不要求本地服务
4. **安装类型限制**：
   - 容器安装：要求 Docker/Podman 服务可用
   - 二进制安装：不依赖 Docker

### TUI 流程调整
- 如果检测到本地 Docker 服务不可用，只显示"二进制安装"选项
- 如果用户选择远程连接，可以在本地使用容器安装（连接到远程 Docker）

## 配置流程

```
Config (install.Config)
├── Action: install.ActionInstall | ActionUpgrade | ActionUninstall
├── Language: "zh" | "en" (仅 TUI 使用)
├── InstallType: install.InstallTypeContainer | InstallTypeBinary | InstallTypeInstallDocker
├── Version: install.VersionCommunity | VersionPro | VersionDev
├── Edition: install.EditionStandard | EditionLite
├── OS: install.OSAlpine | OSDebian
├── ImageRegistry: install.RegistryHub | RegistryAliyun
├── ContainerName: string (默认 "dpanel")
├── Port: int (0 表示随机端口)
├── DataPath: string (默认 "/home/dpanel")
├── DockerConnection:
│   ├── Type: install.DockerConnLocal | DockerConnTCP | DockerConnSSH
│   ├── SockPath (local)
│   ├── Host, TLSEnabled, TLSPath (tcp)
│   └── Host, SSHUser, SSHPass, SSHKey (ssh)
└── Network: Proxy, DNS
```

**重要说明**：
- 所有配置字段使用 `internal/install/constants.go` 中定义的常量
- 禁止在代码中硬编码字符串字面量（如 `"install"`, `"container"` 等）
- 常量提供类型安全、编译时检查和 IDE 支持

## 日志和命令记录

### 日志文件位置
- **运行日志**：`run.log`（程序同级目录，JSON 格式）
- **安装日志**：`install_logs/install_YYYYMMDD_HHMMSS.log`
- **最新日志**：`install_logs/latest.log`

### 安装日志内容
```
=== DPanel Installation Log ===
Date: 2025-03-22 15:04:05
Version: community
Edition: lite
Install Type: container

=== Configuration ===
Container Name: dpanel
Port: 8080
Data Path: /home/dpanel
Docker Connection: local

=== Execution Command ===
docker run -d --name dpanel --restart=on-failure:5 ...

=== End Log ===

[2025-03-22 15:04:10] Installation: SUCCESS
```

### 日志记录时机
- 开始安装：记录完整配置
- 构建命令后：记录即将执行的命令
- 执行命令：记录执行过程
- 安装完成：记录成功/失败结果

## TUI 界面特点

### TUI 布局规范（⚠️ 严格遵守）

**TUI 界面从上到下分为5个区域，必须严格遵守此布局顺序**：

```
┌────────────────────────────────────┐
│ 1. Logo区（始终显示）               │
│    DPANEL ASCII艺术字               │
├────────────────────────────────────┤
│ 2. 标题区                          │
│    🚀 DPanel 安装器 - 步骤名 (n/m)  │
├────────────────────────────────────┤
│ 3. 提示区（可选）                  │
│    根据不同信息显示样式：           │
│    - successStyle: 成功信息 ✓       │
│    - warningColor/hintBoxStyle: 警告 ⚠️ │
│    - errorStyle: 错误信息 ✗         │
│    - infoStyle: 一般信息 ℹ          │
├────────────────────────────────────┤
│ 4. 操作区                          │
│    菜单选择 或 输入框               │
├────────────────────────────────────┤
│ 5. 底部帮助提示区                  │
│    ↑/↓ 移动 | Enter 确认 | ...      │
└────────────────────────────────────┘
```

#### 各区域详细说明

**1. Logo区**
- 内容：DPANEL ASCII艺术字（使用DPanel蓝色）
- 显示规则：所有页面始终显示
- 样式：`primaryColor` (#1890FF) + Bold
- **顶部间距**：1个空行（Top padding）

**2. 标题区**
- 格式：`🚀 DPanel 安装器 - 步骤名 (当前步骤/总步骤数)`
- 示例：`🚀 DPanel 安装器 - 选择版本 (4/18)`
- 语言：根据选择的语言显示步骤名
- 样式：`titleStyle` (蓝色 + 粗体)
- ⚠️ **禁止**：不要在操作区再次显示步骤名（避免重复）

**3. 提示区（可选）**
- 用途：显示环境状态、警告、错误、提示信息
- 样式规则：
  - 成功信息：`successStyle` ("✓ " + 文本)
  - 警告信息：`warningBoxStyle` (带边框的提示框)
  - 错误信息：`errorStyle`
  - 一般信息：`infoStyle`
- 显示规则：只在需要提示时显示，不需要时不留白
- 位置：标题区和操作区之间

**4. 操作区**
- 内容：菜单选择 或 输入框
- 菜单样式：`menuItemStyle` / `menuItemSelectedStyle`
- 输入样式：`inputStyle`
- ⚠️ **禁止**：不要在此区域显示步骤名（已在标题区显示）

**5. 底部帮助提示区**
- 内容：操作快捷键说明
- 菜单页面：`help_navigation` ("↑/↓ 移动 | Enter 确认 | Esc/Backspace 返回 | q/Ctrl+C 退出")
- 输入页面：`help_input` ("Enter 确认 | Esc/Backspace 返回 | q/Ctrl+C 退出")
- 完成页面：`quit_prompt` ("按 'q' 退出")
- 样式：`helpStyle` (灰色 #8C8C8C)

#### 间距规范（⚠️ 严格遵守）

**所有步骤必须遵循统一的间距规则**：

```
┌────────────────────────────────┐
│ [空行]                         │ ← Top padding (1个空行)
│                                │
│ [DPANEL Logo]                  │ ← Logo区
│                                │ ← Logo后换行
│ 🚀 DPanel 安装器 - 步骤名 (n/m) │ ← 标题区
│ [空行]                         │ ← 标题后换行
│ [空行]                         │ ← 额外间距
│ [提示区 - 可选]                │ ← 提示信息（如果需要）
│ [操作区]                       │ ← 菜单或输入
│                                │
│ ↑/↓ 移动 | Enter 确认 | ...     │ ← 帮助提示区
└────────────────────────────────┘
```

**间距定义**：

| 位置 | 换行符数量 | 说明 | 代码位置 |
|------|-----------|------|---------|
| Logo顶部 | 1个`\n` | Top padding | View() line 308 |
| Logo底部 | 1个`\n` | Logo后换行 | View() line 312 |
| 标题底部 | 2个`\n` | 标题后固定换行 + 额外间距 | View() line 330 + 各步骤case |
| 标题与内容 | 2个空行 | 固定间距 | 所有步骤统一 |

**代码实现**：

```go
func (m model) View() string {
    var content strings.Builder

    // Top padding - 所有步骤统一
    content.WriteString("\n")

    // 1. Logo区
    content.WriteString(renderLogo())
    content.WriteString("\n")

    // 2. 标题区
    titleText := fmt.Sprintf("🚀 %s - %s (%d/%d)", ...)
    content.WriteString(titleStyle.Render(titleText))
    content.WriteString("\n")

    // 标题后固定换行 - 所有步骤统一
    content.WriteString("\n")

    // 3-5. 各步骤内容（包含额外间距）
    switch m.step {
    case StepLanguage:
        content.WriteString("\n")  // ← 额外间距
        content.WriteString(m.renderMenu())

    case StepEdition:
        content.WriteString("\n")  // ← 额外间距
        content.WriteString(m.renderEdition())

    // ... 其他步骤相同
    }

    // 6. 底部帮助提示区
    content.WriteString("\n")
    content.WriteString(helpStyle.Render(i18n.T("help_navigation")))

    return content.String()
}
```

**重要规则**：

✅ **必须遵守**：
1. 所有步骤的顶部必须有1个空行（Top padding）
2. 所有步骤的标题后必须有2个空行（固定 + 额外）
3. Logo后面必须有1个换行符
4. 所有步骤的间距必须完全一致

❌ **禁止行为**：
1. 省略任何步骤的Top padding
2. 省略任何步骤的额外间距（`\n`）
3. 在不同步骤使用不同的间距规则
4. 在操作区显示步骤名（已在标题区显示）

**常见错误**：

❌ **错误1**：不同步骤间距不一致
```go
// ❌ 错误：有些步骤有额外间距，有些没有
case StepLanguage:
    content.WriteString(m.renderMenu())         // 缺少额外"\n"

case StepEdition:
    content.WriteString("\n")
    content.WriteString(m.renderEdition())      // 有额外"\n"

// ✅ 正确：所有步骤统一
case StepLanguage:
    content.WriteString("\n")
    content.WriteString(m.renderMenu())

case StepEdition:
    content.WriteString("\n")
    content.WriteString(m.renderEdition())
```

❌ **错误2**：省略Top padding
```go
// ❌ 错误：没有Top padding
content.WriteString(renderLogo())

// ✅ 正确：所有页面都有Top padding
content.WriteString("\n")
content.WriteString(renderLogo())
```

#### 代码实现规范

```go
func (m model) View() string {
    var content strings.Builder

    // 1. Logo区（始终显示）
    content.WriteString(renderLogo())
    content.WriteString("\n")

    // 2. 标题区（带步骤号）
    titleText := fmt.Sprintf("🚀 %s - %s (%d/%d)",
        i18n.T("title"),
        stepName,
        m.step,
        StepError-1)
    content.WriteString(titleStyle.Render(titleText))
    content.WriteString("\n")

    // 3. 提示区（可选）
    if needsWarning {
        content.WriteString(warningStyle.Render("⚠️ " + warningText))
        content.WriteString("\n")
    }
    if needsHint {
        content.WriteString(hintBoxStyle.Render(hintText))
        content.WriteString("\n")
    }

    // 4. 操作区
    content.WriteString(m.renderMenu()) // 或 renderInput()

    // 5. 底部帮助提示区
    content.WriteString("\n")
    content.WriteString(helpStyle.Render(i18n.T("help_navigation")))

    return content.String()
}
```

#### 常见错误 ❌

1. **标题重复**：标题区显示"选择版本"后，操作区又显示"选择版本"
   - ❌ 错误：标题区 + 操作区都显示步骤名
   - ✅ 正确：只在标题区显示，操作区直接显示菜单

2. **缺少提示区**：环境检测信息直接显示在操作区
   - ❌ 错误：提示信息和菜单混在一起
   - ✅ 正确：提示信息独立显示在提示区，使用对应样式

3. **样式使用错误**：错误信息使用了info样式
   - ❌ 错误：错误信息用 `infoStyle`
   - ✅ 正确：错误信息用 `errorStyle`，警告用 `hintBoxStyle`

### 全屏无边框设计
- 移除了传统的边框样式
- 使用整个终端屏幕
- 添加分隔线区分不同区域
- 更舒适的间距和布局

### 配色方案（DPanel 主题）
- 主色：`#1890FF` (蓝色)
- 成功：`#52C41A` (绿色)
- 警告：`#FAAD14` (琥珀色)
- 错误：`#FF4D4F` (红色)
- 背景：`#141414` (深色背景)
- 输入框：`#2A2A2A` (输入背景)
- 选中：`#0050B3` (选中背景)
- 文本：`#E8E8E8` (主文本)

### 提示信息样式规范（⚠️ 严格遵守）

**TUI 中的提示信息分为 4 种类型，每种类型有固定的样式和图标**

#### 1. 成功信息 (Success)

**用途**：操作成功完成、状态正常

**样式**：`successStyle`
- 颜色：`#52C41A` (成功绿色)
- 图标：`✓`
- 粗体：`Bold(true)`

**代码示例**：
```go
content.WriteString(successStyle.Render("✓ " + i18n.T("installation_complete")))
```

**使用场景**：
- ✅ 安装成功
- ✅ Docker/Podman 检测成功
- ✅ 配置验证通过

#### 2. 错误信息 (Error)

**用途**：操作失败、严重错误

**样式**：`errorStyle`
- 颜色：`#FF4D4F` (错误红色)
- 图标：`✗`
- 粗体：`Bold(true)`

**代码示例**：
```go
content.WriteString(errorStyle.Render("✗ " + i18n.T("installation_failed")))
```

**使用场景**：
- ✅ 安装失败
- ✅ 权限不足
- ✅ 配置错误

**错误框样式**（多行错误信息）：`errorBoxStyle`
```go
errorBoxStyle = lipgloss.NewStyle().
    Foreground(errorColor).          // 红色文字
    Background(bgInputColor).         // 深色背景
    Padding(1, 2).                   // 内边距
    MarginBottom(1).                 // 底部外边距
    Border(lipgloss.RoundedBorder()).// 圆角边框
    BorderForeground(errorColor)     // 红色边框
```

#### 3. 警告信息 (Warning)

**用途**：需要注意但不阻止操作的信息

**样式**：`warningStyle`
- 颜色：`#FAAD14` (警告琥珀色)
- 图标：`⚠️`
- 粗体：`Bold(true)`

**代码示例**：
```go
content.WriteString(warningStyle.Render("⚠️ " + warningText))
```

**使用场景**：
- ✅ Docker 未安装但可继续
- ✅ 功能限制提示
- ✅ 配置建议

**警告框样式**（重要警告）：`warningBoxStyle`
```go
warningBoxStyle = lipgloss.NewStyle().
    Foreground(warningColor).         // 琥珀色文字
    Background(bgInputColor).         // 深色背景
    Padding(1, 2).                   // 内边距
    MarginBottom(1).                 // 底部外边距
    Border(lipgloss.RoundedBorder()).// 圆角边框
    BorderForeground(warningColor)    // 琥珀色边框
```

**视觉效果**：
```
╭────────────────────────────────────────╮
│ 您的系统未安装 Docker Desktop。         │
│ https://www.docker.com/products/...    │
╰────────────────────────────────────────╯
```

#### 4. 信息提示 (Info)

**用途**：一般性提示、说明文字

**样式**：`infoStyle`
- 颜色：`#1890FF` (信息蓝色)
- 图标：`ℹ️`、`⏳`（等待中）
- 粗体：`Bold(true)`

**代码示例**：
```go
content.WriteString(infoStyle.Render("ℹ️  " + infoText))
content.WriteString(infoStyle.Render("⏳ " + i18n.T("please_wait")))
```

**使用场景**：
- ✅ 功能说明
- ✅ 操作提示
- ✅ 等待状态
- ✅ 一般性建议

**信息框样式**（重要提示）：`infoBoxStyle`
```go
infoBoxStyle = lipgloss.NewStyle().
    Foreground(infoColor).             // 蓝色文字
    Background(bgInputColor).          // 深色背景
    Padding(1, 2).                     // 内边距
    MarginBottom(1).                   // 底部外边距
    Border(lipgloss.RoundedBorder()).  // 圆角边框
    BorderForeground(infoColor)        // 蓝色边框
```

#### 样式选择指南

| 场景 | 使用样式 | 图标 | 是否带框 |
|------|---------|------|---------|
| 操作成功 | `successStyle` | ✓ | 否 |
| 检测成功 | `successStyle` | ✓ | 否 |
| 安装失败 | `errorStyle` | ✗ | 否 |
| 严重错误 | `errorStyle` | ✗ | 是（多行） |
| 未安装Docker | `warningBoxStyle` | - | 是 |
| 功能限制 | `warningStyle` | ⚠️ | 否 |
| 配置建议 | `infoStyle` | ℹ️ | 否 |
| 等待中 | `infoStyle` | ⏳ | 否 |
| 功能说明 | `infoStyle` | ℹ️ | 是（重要） |

#### 常见错误 ❌

1. **样式混淆**：错误信息使用了 infoStyle
   - ❌ 错误：`infoStyle.Render("✗ 安装失败")`
   - ✅ 正确：`errorStyle.Render("✗ 安装失败")`

2. **缺少图标**：提示信息没有图标前缀
   - ❌ 错误：`successStyle.Render("安装完成")`
   - ✅ 正确：`successStyle.Render("✓ 安装完成")`

3. **图标不匹配**：成功信息使用了错误图标
   - ❌ 错误：`successStyle.Render("✗ 安装完成")`
   - ✅ 正确：`successStyle.Render("✓ 安装完成")`

4. **多行信息未使用框样式**：多行警告信息未使用 warningBoxStyle
   - ❌ 错误：`warningStyle.Render("长文本1\n长文本2")`
   - ✅ 正确：`warningBoxStyle.Render("长文本1\n长文本2")`

#### 完整代码示例

```go
// 成功信息 - 简单文本
content.WriteString(successStyle.Render("✓ " + i18n.T("docker_detected")))
content.WriteString("\n\n")

// 信息提示 - 简单文本
content.WriteString(infoStyle.Render(i18n.T("container_available_prompt")))
content.WriteString("\n\n")

// 等待状态 - 使用时钟图标
content.WriteString(infoStyle.Render("⏳ " + i18n.T("please_wait")))
content.WriteString("\n")

// 警告框 - 多行重要信息
warningText := i18n.T("docker_not_found_desktop") + "\n\n" +
    i18n.T("docker_download_url")
content.WriteString(warningBoxStyle.Render(warningText))
content.WriteString("\n")

// 错误信息 - 简单文本
content.WriteString(errorStyle.Render("✗ " + i18n.T("installation_failed")))
content.WriteString("\n\n")
if err != nil {
    content.WriteString(err.Error())
}
```
```

### 禁用选项规范

**用于标记不可用的菜单选项（如未安装 Docker 时的容器安装选项）**

#### 样式定义
```go
menuItemDisabledStyle = lipgloss.NewStyle().
    Foreground(mutedColor).      // 灰色文字 #858585
    PaddingLeft(2).               // 左侧2列内边距
    MarginBottom(0).              // 底部无外边距
    Italic(true)                  // 斜体显示

menuItemSelectedDisabledStyle = lipgloss.NewStyle().
    Foreground(mutedColor).       // 灰色文字 #858585
    Background(bgInputColor).     // 深色背景 #3C3C3C
    PaddingLeft(2).               // 左侧2列内边距
    PaddingRight(1).              // 右侧1列内边距
    MarginBottom(0).              // 底部无外边距
    Italic(true)                  // 斜体显示
```

#### 使用场景
- ✅ **容器安装未安装 Docker 时**：显示但禁用容器安装选项
- ✅ **功能未实现时**：显示但禁用相关功能选项
- ✅ **权限不足时**：显示但禁用需要更高权限的选项
- ✅ **依赖缺失时**：显示但禁用缺少依赖的选项

#### Model 结构扩展
```go
type model struct {
    // ... 其他字段
    choices      []string  // 选项文本
    descriptions []string  // 选项描述
    disabled     []bool    // 禁用状态（与 choices 一一对应）
    // ... 其他字段
}
```

#### 设置选项示例
```go
func (m *model) setupInstallTypeChoices() {
    dockerAvailable := m.envCheck.DockerAvailable || m.envCheck.PodmanAvailable

    if dockerAvailable {
        // Docker 可用 - 两个选项都启用
        m.choices = []string{
            i18n.T("container_install"),
            i18n.T("binary_install"),
        }
        m.descriptions = []string{
            i18n.T("container_install_desc"),
            i18n.T("binary_install_desc"),
        }
        m.disabled = []bool{false, false}
    } else {
        // Docker 不可用 - 容器安装显示但禁用
        m.choices = []string{
            i18n.T("container_install"),
            i18n.T("binary_install"),
        }
        m.descriptions = []string{
            i18n.T("container_install_desc") + " " + i18n.T("container_install_disabled"),
            i18n.T("binary_install_desc"),
        }
        m.disabled = []bool{true, false}  // 容器安装被禁用
    }
}
```

#### 渲染选项示例
```go
func (m model) renderMenu() string {
    var s strings.Builder
    for i, choice := range m.choices {
        isDisabled := len(m.disabled) > i && m.disabled[i]
        isSelected := i == m.cursor

        if isDisabled && isSelected {
            s.WriteString(menuItemSelectedDisabledStyle.Render(fmt.Sprintf("▸ %s", choice)))
        } else if isDisabled {
            s.WriteString(menuItemDisabledStyle.Render(fmt.Sprintf("  %s", choice)))
        } else if isSelected {
            s.WriteString(menuItemSelectedStyle.Render(fmt.Sprintf("▸ %s", choice)))
        } else {
            s.WriteString(menuItemStyle.Render(fmt.Sprintf("  %s", choice)))
        }
        // ... 渲染描述
    }
    return s.String()
}
```

#### 导航跳过禁用选项
```go
case "up", "k":
    if m.cursor > 0 {
        m.cursor--
        // 跳过禁用的选项
        for m.cursor > 0 && len(m.disabled) > m.cursor && m.disabled[m.cursor] {
            m.cursor--
        }
    }
case "down", "j":
    if m.cursor < len(m.choices)-1 {
        m.cursor++
        // 跳过禁用的选项
        for m.cursor < len(m.choices)-1 && len(m.disabled) > m.cursor && m.disabled[m.cursor] {
            m.cursor++
        }
    }
```

#### 防止选择禁用选项
```go
case StepInstallType:
    // 检查选定选项是否被禁用
    if len(m.disabled) > m.cursor && m.disabled[m.cursor] {
        // 选项被禁用，不执行任何操作
        return m, nil
    }
    // 处理正常选择逻辑...
```

#### 视觉效果
```
安装方式

  容器安装（Docker 不可用 - 需要手动安装）
▸ 二进制安装
  将 DPanel 安装为独立二进制程序

↑/↓ 移动 | Enter 确认 | Esc/Backspace 返回 | q/Ctrl+C 退出
```

#### 翻译键规范
**禁用提示文本**（附加到描述后）：
- 英文：`"container_install_disabled": "(Docker not available - requires manual installation)"`
- 中文：`"container_install_disabled": "（Docker 不可用 - 需要手动安装）"`

**格式**：
- 英文使用括号：`(reason)`
- 中文使用括号：`（原因）`
- 文本简洁明了，说明为什么禁用

#### 实现要点
1. **显示但禁用**：用户可以看到有哪些选项，但无法选择
2. **清晰提示**：在描述中添加禁用原因
3. **视觉区分**：使用灰色+斜体样式
4. **自动跳过**：上下键自动跳过禁用选项
5. **防止选择**：回车键检查选项是否被禁用

### 交互优化
- 清晰的视觉反馈
- 合理的间距
- 明确的操作提示
- 详细的配置描述

### 帮助提示规范（⚠️ 严格遵守）

**TUI 界面底部的帮助提示必须保持以下格式，不得随意更改**：

#### 菜单选择页面
```
↑/↓ Navigate | Enter Confirm | Esc/Backspace Back | q/Ctrl+C Quit
↑/↓ 移动 | Enter 确认 | Esc/Backspace 返回 | q/Ctrl+C 退出
```

#### 输入页面
```
Enter Confirm | Esc/Backspace Back | q/Ctrl+C Quit
Enter 确认 | Esc/Backspace 返回 | q/Ctrl+C 退出
```

#### 完成页面
```
Press 'q' to quit
按 'q' 退出
```

**重要说明**：
- ✅ **Enter**：确认/选择/继续
- ✅ **Esc/Backspace**：返回上一步（在输入页面，Backspace 在有输入内容时删除字符，无内容时返回）
- ✅ **q/Ctrl+C**：退出程序
- ✅ **↑/↓**：在菜单项之间移动
- ⚠️ 帮助文本使用 `helpStyle` 样式（灰色，`#858585`）
- ⚠️ 语言选择页面使用硬编码文本（因为 i18n 尚未初始化）
- ⚠️ 其他页面使用 i18n 翻译键：`help_navigation`、`help_input`、`quit_prompt`

## TUI 安装流程

按照 `docs/Installer.md` 中的完整流程实现：

1. **选择语言** → 初始化语言包
2. **选择操作** → install/upgrade/uninstall
3. **环境检测** → 真实测试 Docker 服务可用性
4. **安装 Docker**（可选）→ 如果未检测到可用服务
5. **安装方式** → 根据环境检测结果显示可用选项
6. **选择版本** → 社区版 / 专业版 / 开发版
7. **选择版本类型** → 标准版 / 精简版
8. **选择基础系统** → Alpine / Debian
9. **选择镜像仓库** → Docker Hub / 阿里云
10. **配置 Docker 连接** → 本地 socket / 远程 TCP / 远程 SSH
11. **配置连接参数** → 根据 connection type 配置
12. **配置容器名称** → 默认 "dpanel"
13. **配置访问端口** → 0 表示随机端口
14. **配置数据存储路径** → 默认 "/home/dpanel"
15. **配置代理地址**（可选）
16. **配置 DNS 地址**（可选）
17. **确认安装** → 显示配置摘要
18. **执行安装** → 调用 Engine 执行，记录完整日志

## Docker 命令格式

### 容器安装
```bash
docker run -d \
  --name 容器名称 \
  --restart=on-failure:5 \
  --log-driver json-file \
  --log-opt max-size=5m \
  --log-opt max-file=10 \
  --hostname 容器名称.pod.dpanel.local \
  --add-host host.dpanel.local:host-gateway \
  -e APP_NAME=容器名称 \
  -e HTTP_PROXY=代理地址 \
  -e HTTPS_PROXY=代理地址 \
  --dns dns地址 \
  -p 绑定端口:8080 \
  -p 80:80 \
  -p 443:443 \
  -v sock文件:/var/run/docker.sock \
  -v 数据存储目录:/dpanel \
  镜像
```

### 镜像命名规则
- 社区版标准版：`dpanel/dpanel:latest`
- 社区版精简版：`dpanel/dpanel:lite`
- 专业版标准版：`dpanel/dpanel-pe:latest`
- 专业版精简版：`dpanel/dpanel-pe:lite`
- 开发版标准版：`dpanel/dpanel:beta`
- 开发版精简版：`dpanel/dpanel:beta-lite`
- 阿里云镜像：`registry.cn-hangzhou.aliyuncs.com/{镜像名}`

## 实现说明

### 已实现功能
- ✅ 完整的 TUI 向导流程（全屏无边框）
- ✅ CLI 模式支持所有参数
- ✅ 多语言支持（英文/简体中文）
- ✅ 真实的环境检测（测试服务可用性）
- ✅ DPanel 主题配色方案
- ✅ 完整的安装日志记录
- ✅ 最终执行命令的保存
- ✅ Docker 命令构建
- ✅ 支持远程 Docker 连接（TCP/SSH）
- ✅ 配置常量系统（所有配置值使用命名常量）

### 待实现功能
- ⏳ Upgrade/Uninstall 功能
- ⏳ 二进制安装
- ⏳ TCP/SSH 连接的实际检测
- ⏳ TLS 证书生成
- ⏳ 自动安装 Docker
- ⏳ Windows 平台支持

## 语言使用规范

### 模式选择
- **默认行为**：无参数时启动 TUI 模式
- **有参数**：任何参数（如 `--help`, `--action` 等）启动 CLI 模式
- **显式指定**：使用 `--mode tui` 或 `--mode cli` 强制指定模式

### CLI 模式
- **所有帮助信息使用英文**
- **所有错误消息使用英文**
- **不加载语言包**
- **适用于自动化脚本和程序化调用**

### TUI 模式
- **第一步选择语言**
- **所有界面文本使用语言包**
- **支持英文和简体中文**
- **适用于交互式安装和手动配置**

## 日志规范（⚠️ 严格遵守）

### 核心原则

**所有日志记录必须使用 `log/slog` 包，禁止使用 `fmt` 包进行日志输出。**

### 日志使用场景

#### 1. **使用 slog 的场景**

```go
// ✅ 正确：所有日志记录
slog.Info("Starting installation", "version", cfg.Version)
slog.Error("Installation failed", "error", err)
slog.Warn("Docker not available", "reason", "not installed")
slog.Debug("Environment check", "docker", true)

// ✅ 正确：带上下文的日志
slog.Info("Creating container",
    "name", cfg.ContainerName,
    "port", cfg.Port,
    "image", cfg.GetImageName())

// ✅ 正确：错误日志
slog.Error("Failed to pull image",
    "image", imageName,
    "error", err)
```

#### 2. **使用 fmt 的场景（仅限用户界面输出）**

```go
// ✅ 允许：用户界面输出（帮助信息）
fmt.Println("DPanel Installer")
fmt.Printf("Version: %s\n", version)

// ✅ 允许：写入文件（安装日志）
fmt.Fprintf(logFile, "=== Installation Log ===\n")
fmt.Fprintf(logFile, "Date: %s\n", timestamp)

// ❌ 禁止：使用 fmt 输出日志
fmt.Printf("Error: %v\n", err)  // ✗ 错误！应该用 slog
fmt.Println("Starting...")       // ✗ 错误！应该用 slog
```

### 日志级别

```go
// Debug：详细的调试信息
slog.Debug("Parsing command line flags", "args", args)

// Info：一般信息（默认级别）
slog.Info("Installation completed successfully")

// Warn：警告信息（不影响功能）
slog.Warn("Docker Desktop not installed", "action", "using binary installation")

// Error：错误信息（影响功能）
slog.Error("Failed to create container", "error", err)
```

### 结构化日志

```go
// ✅ 正确：使用键值对
slog.Info("Configuration loaded",
    "version", cfg.Version,
    "edition", cfg.Edition,
    "port", cfg.Port)

// ❌ 错误：拼接字符串
slog.Info(fmt.Sprintf("Configuration loaded: version=%s edition=%s port=%d",
    cfg.Version, cfg.Edition, cfg.Port))
```

### 日志文件位置

```
程序目录/
├── run.log                 # 运行日志（JSON 格式，slog 输出）
└── install_logs/
    ├── latest.log           # 最新安装日志（文本格式，fmt.Fprintf 输出）
    ├── install_20250322_150405.log
    └── ...
```

### 日志记录时机

```go
// ✅ 程序启动
slog.Info("Starting DPanel Installer", "version", version, "mode", mode)

// ✅ 关键操作开始
slog.Info("Creating container", "name", containerName)

// ✅ 关键操作完成
slog.Info("Container created successfully", "id", containerID)

// ✅ 环境检测
slog.Info("Environment check completed",
    "docker_available", envCheck.DockerAvailable,
    "os", envCheck.OS)

// ✅ 配置应用
slog.Info("Configuration applied",
    "source", source.Name(),
    "action", cfg.Action)

// ✅ 错误发生
slog.Error("Operation failed",
    "operation", "pull_image",
    "image", imageName,
    "error", err)
```

### 禁止的模式

```go
// ❌ 禁止：使用 fmt 输出日志
fmt.Printf("Error: %v\n", err)
fmt.Println("Starting installation...")
fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)

// ❌ 禁止：使用 log 包
log.Println("Starting...")
log.Printf("Error: %v", err)

// ❌ 禁止：使用 fmt + slog 组合
fmt.Println(slog.String("message", "value"))
```

### 安装日志文件

安装日志文件（`install_logs/*.log`）使用 `fmt.Fprintf` 写入，因为：
1. 需要特定格式便于用户查看
2. 不是程序日志，是安装记录
3. 需要人类可读的文本格式

```go
// ✅ 正确：写入安装日志
func (e *Engine) saveInstallationLog() error {
    logFile, _ := os.Create(logPath)
    defer logFile.Close()

    fmt.Fprintf(logFile, "=== DPanel Installation Log ===\n")
    fmt.Fprintf(logFile, "Date: %s\n", time.Now().Format("2006-01-02 15:04:05"))
    fmt.Fprintf(logFile, "Version: %s\n", e.Config.Version)
    fmt.Fprintf(logFile, "\n=== Configuration ===\n")
    fmt.Fprintf(logFile, "Container Name: %s\n", e.Config.ContainerName)
    fmt.Fprintf(logFile, "\n=== Execution Command ===\n")
    fmt.Fprintf(logFile, "%s\n", command)

    slog.Info("Installation log saved", "path", logPath)
    return nil
}
```

## 开发注意事项

### ⚠️ 核心原则：优先使用现有的方法和逻辑

**在开发新功能或修改代码时，必须遵循以下原则**：

#### 1. **优先复用现有样式**
- ✅ **优先使用**：已定义的 TUI 样式（`titleStyle`, `successStyle`, `hintBoxStyle` 等）
- ❌ **避免创建**：新的相似样式，除非有显著差异
- 📋 **检查位置**：`internal/handler/tui/tui.go` 第 84-148 行

**示例**：
```go
// ✅ 好的做法：使用现有的 hintBoxStyle
s.WriteString(hintBoxStyle.Render(warningText))

// ❌ 不好的做法：创建新的类似样式
myWarningStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("#FF8800")).  // 与 warningColor 重复
    Background(lipgloss.Color("#3C3C3C"))   // 与 bgInputColor 重复
```

#### 2. **优先复用现有的翻译键**
- ✅ **优先使用**：已存在的翻译键
- ❌ **避免添加**：重复或相似的新翻译键
- 📋 **检查位置**：`pkg/i18n/translations/en.json` 和 `zh.json`

**示例**：
```json
// ✅ 好的做法：合并相似翻译
"docker_not_found_desktop": "Docker Desktop is not installed"

// ❌ 不好的做法：为每个系统创建单独的键
"docker_not_found_windows": "Docker Desktop is not installed",
"docker_not_found_macos": "Docker Desktop is not installed"
```

#### 3. **优先复用现有代码逻辑**
- ✅ **优先使用**：已有的函数和方法
- ❌ **避免重复**：实现相同逻辑的新代码
- 📋 **检查方法**：使用 `Grep` 工具搜索相关实现

**示例**：
```go
// ✅ 好的做法：使用现有的环境检测逻辑
if m.envCheck.DockerAvailable {
    // 处理逻辑
}

// ❌ 不好的做法：重新实现环境检测
dockerExists := checkDockerCommand()  // 重复实现
if dockerExists {
    // 处理逻辑
}
```

#### 4. **优先使用常量而非字符串字面量**（⚠️ 严格遵守）
- ✅ **优先使用**：已定义的常量（`install.ActionInstall`, `install.InstallTypeContainer` 等）
- ❌ **禁止使用**：硬编码的字符串字面量（如 `"install"`, `"container"` 等）
- 📋 **常量位置**：`internal/install/constants.go`

**所有配置值必须使用常量**：

```go
// ✅ 正确：使用常量
if m.config.InstallType == install.InstallTypeContainer {
    // 处理逻辑
}

m.config.Action = install.ActionInstall
m.config.Version = install.VersionCommunity
m.config.Edition = install.EditionLite
m.config.OS = install.OSDebian
m.config.Registry = install.RegistryHub
m.config.DockerConnType = install.DockerConnLocal

// ❌ 错误：使用字符串字面量
if m.config.InstallType == "container" {  // ✗ 错误！
    // 处理逻辑
}

m.config.Action = "config"  // ✗ 错误！
```

**可用常量列表**：

| 类别 | 常量 | 值 |
|------|------|-----|
| 操作类型 | `ActionInstall`, `ActionUpgrade`, `ActionUninstall` | `"install"`, `"upgrade"`, `"uninstall"` |
| 安装类型 | `InstallTypeContainer`, `InstallTypeBinary`, `InstallTypeInstallDocker` | `"container"`, `"binary"`, `"install_docker"` |
| 版本 | `VersionCommunity`, `VersionPro`, `VersionDev` | `"community"`, `"pro"`, `"dev"` |
| 版本类型 | `EditionStandard`, `EditionLite` | `"standard"`, `"lite"` |
| 系统 | `OSAlpine`, `OSDebian` | `"alpine"`, `"debian"` |
| 镜像源 | `RegistryHub`, `RegistryAliyun` | `"hub"`, `"aliyun"` |
| Docker 连接 | `DockerConnLocal`, `DockerConnTCP`, `DockerConnSSH` | `"local"`, `"tcp"`, `"ssh"` |

**验证函数**：
```go
// ✅ 使用验证函数
if install.IsValidAction(action) {
    // 有效
}

if install.IsValidInstallType(installType) {
    // 有效
}
```

**原因**：
- ✅ **类型安全**：编译时检查，避免拼写错误
- ✅ **IDE 支持**：自动补全和重构
- ✅ **单一真实来源**：修改常量定义即可全局更新
- ✅ **文档化**：常量名即文档，清晰表达意图
- ❌ **字符串风险**：拼写错误在运行时才能发现

#### 5. **遵循已定义的规范**
- ✅ **严格遵守**：帮助提示格式、样式使用规范
- ✅ **保持一致**：配色方案、交互模式
- 📋 **参考文档**：`CLAUDE.md` 中的相关规范章节

#### 6. **修改前必查**
在修改或添加代码前，必须完成以下检查：
1. 🔍 使用 `Grep` 搜索是否有类似实现
2. 📖 查阅 `CLAUDE.md` 相关规范
3. 🎨 检查是否有可复用的样式和翻译键
4. 🔢 检查是否有可用的常量（查看 `internal/install/constants.go`）
5. 💡 评估是否可以通过扩展现有功能实现

#### 7. **规范更新**
当确实需要创建新的样式、翻译键或方法时：
- ✅ 必须更新相关文档（`CLAUDE.md`）
- ✅ 添加清晰的注释说明用途
- ✅ 遵循命名约定
- ✅ 考虑未来可扩展性

**添加新常量时**：
- ✅ 在 `internal/install/constants.go` 中定义
- ✅ 添加对应的 `IsValid*()` 验证函数（如果需要）
- ✅ 在 `Valid*` 数组中注册
- ✅ 更新本规范的常量列表

### 代码审查检查项

在提交代码前，确认以下问题：
- [ ] 是否复用了现有的样式？（检查 TUI 样式定义）
- [ ] 是否复用了现有的翻译键？（检查翻译文件）
- [ ] 是否复用了现有的代码逻辑？（使用 Grep 搜索）
- [ ] 是否使用了常量而非字符串字面量？（检查 `internal/install/constants.go`）
- [ ] 是否遵循了已定义的规范？（查阅 CLAUDE.md）
- [ ] 新增的内容是否已更新文档？
- [ ] 代码风格是否与现有代码一致？
- [ ] 新增常量是否已添加到 `constants.go` 并更新文档？

## Agent Teams 系统

项目使用基于 Claude Code 的多 Agent 协作系统，用于提高开发效率、保证代码质量、实现模块化开发。

### 系统架构

```
.claude/
├── agents/          # 12 个专门 agents 配置
├── skills/          # 7 个可重用技能
└── workflows/       # 4 个预定义工作流
```

### 核心开发 Agents

#### **backend-dev** - 后端开发专家
- **职责**：核心安装逻辑实现
- **主要文件**：`internal/install/engine.go`
- **专业领域**：Docker/Podman 集成、系统编程、错误处理

#### **ui-dev** - TUI 界面专家
- **职责**：Bubble Tea TUI 实现
- **主要文件**：`internal/handler/tui/tui.go`
- **专业领域**：终端 UI 设计、状态管理、DPanel 主题

#### **config-dev** - 配置架构师
- **职责**：配置结构设计和验证
- **主要文件**：`internal/install/config.go`
- **专业领域**：Go 结构设计、验证逻辑、默认值策略

#### **cli-dev** - CLI 框架专家
- **职责**：Cobra 命令结构和标志定义
- **主要文件**：`main.go`
- **专业领域**：CLI UX、标志解析、帮助文本（英文）

#### **i18n-dev** - 国际化专家
- **职责**：多语言支持实现
- **主要文件**：`pkg/i18n/i18n.go`
- **专业领域**：Go embed、JSON 翻译、语言切换

### 质量保证 Agents

#### **reviewer** - 代码审查专家
- **职责**：代码审查和质量保证
- **工具**：golangci-lint, go vet, gosec
- **关注点**：错误处理、资源清理、安全、性能

#### **test-dev** - 测试专家
- **职责**：测试实现和覆盖率
- **方法**：表驱动测试、Mock、集成测试
- **目标**：覆盖率 > 80%

#### **security-dev** - 安全专家
- **职责**：安全审计和漏洞扫描
- **关注点**：命令注入、路径遍历、凭证处理
- **工具**：gosec、安全审查清单

### 文档 Agents

#### **docs-writer** - 文档专家
- **职责**：项目文档维护
- **主要文件**：CLAUDE.md, README.md
- **内容**：架构文档、指南、示例

#### **api-docs** - API 文档专家
- **职责**：Go 代码文档
- **标准**：godoc 注释、示例
- **内容**：包、类型、函数文档

### DevOps Agents

#### **build-dev** - 构建发布专家
- **职责**：编译和发布管理
- **平台**：Windows, Linux, macOS
- **任务**：版本注入、交叉编译、CI/CD

#### **platform-dev** - 平台兼容性专家
- **职责**：多平台支持
- **平台**：Windows, Linux, macOS
- **方法**：构建标签、条件编译

### 可重用技能

#### **go-review.skill**
代码审查工作流：
1. 自动化检查（lint, vet, gosec）
2. 代码结构审查
3. 错误处理审查
4. 资源管理审查
5. 安全审查
6. 文档审查
7. 性能审查
8. 测试审查

#### **go-test.skill**
测试实现工作流：
1. 识别测试目标
2. 设计测试结构
3. 编写表驱动测试
4. 实现 Mock
5. 测试错误情况
6. 测试资源清理
7. 运行覆盖率检查
8. 验证覆盖率 > 80%

#### **go-build.skill**
构建工作流：
1. 准备构建环境
2. 设置版本信息
3. 交叉编译所有平台
4. 优化二进制文件
5. 测试二进制文件
6. 创建发布包
7. 生成校验和
8. 上传发布

#### **docker-integration.skill**
Docker 集成工作流：
1. 检测容器运行时
2. 构建 Docker 命令
3. 构建镜像名称
4. 验证命令
5. 执行命令
6. 测试连接

#### **tui-development.skill**
TUI 开发工作流：
1. 设计屏幕流程
2. 定义模型结构
3. 实现 Update 函数
4. 实现 View 函数
5. 实现渲染函数
6. 实现设置函数
7. 实现导航
8. 应用样式
9. 添加国际化

#### **i18n-management.skill**
国际化管理工作流：
1. 添加新翻译键
2. 组织翻译键
3. 在代码中使用翻译
4. 维护翻译文件
5. 测试语言切换
6. 处理复数形式
7. 处理格式化

#### **documentation.skill**
文档更新工作流：
1. 识别文档需求
2. 确定文档类型
3. 更新 CLAUDE.md
4. 更新 README.md
5. 更新 API 文档
6. 创建/更新图表
7. 更新发布说明
8. 审查文档

### 预定义工作流

#### **feature-development.json**
新功能开发流程（10 步）：
1. config-dev: 设计配置结构
2. backend-dev: 实现核心逻辑
3. ui-dev: 创建 TUI 界面（并行）
4. i18n-dev: 添加翻译
5. cli-dev: 添加 CLI 标志
6. test-dev: 编写测试
7. security-dev: 安全审查（并行）
8. reviewer: 代码审查
9. api-docs: 添加 API 文档（并行）
10. docs-writer: 更新文档

#### **bug-fix.json**
Bug 修复流程（8 步）：
1. backend-dev: 分析 bug
2. platform-dev: 平台调查（并行）
3. backend-dev: 实现修复
4. test-dev: 添加回归测试
5. platform-dev: 跨平台测试（并行）
6. security-dev: 安全审查
7. reviewer: 代码审查
8. docs-writer: 更新文档

#### **release-prep.json**
发布准备流程（10 步）：
1. build-dev: 版本规划
2. reviewer: 功能冻结
3. test-dev: 综合测试
4. security-dev: 安全审计（并行）
5. platform-dev: 平台测试（并行）
6. docs-writer: 文档审查
7. build-dev: 构建发布
8. build-dev: 创建发布包
9. reviewer: 最终审查
10. build-dev: 发布

#### **documentation-update.json**
文档更新流程（10 步）：
1. docs-writer: 需求评估
2. docs-writer: 收集技术信息
3. docs-writer: 更新 CLAUDE.md（并行）
4. docs-writer: 更新 README.md（并行）
5. api-docs: 更新 API 文档（并行）
6. docs-writer: 更新架构文档
7. i18n-dev: 更新翻译文档（并行）
8. docs-writer: 创建示例和教程
9. reviewer: 文档审查
10. docs-writer: 最终润色

### Agent 协作模式

#### 并行协作
- backend-dev 实现 ↔ ui-dev 界面
- test-dev 准备测试结构

#### 顺序依赖
- config-dev → backend-dev → test-dev → reviewer → docs-writer

### 使用示例

#### 开发新功能
```
用户：实现 TCP 连接测试功能
系统：启动 feature-development 工作流
1. config-dev 设计配置
2. backend-dev 实现测试逻辑
3. ui-dev 添加 TUI 界面
4. test-dev 编写测试
5. reviewer 审查代码
6. docs-writer 更新文档
```

#### 修复 Bug
```
用户：环境检测不工作
系统：启动 bug-fix 工作流
1. backend-dev 分析问题
2. platform-dev 调查平台差异
3. backend-dev 实现修复
4. test-dev 添加回归测试
5. reviewer 审查修复
```

### 质量保证

#### 自动化检查
```bash
golangci-lint run ./...
go vet ./...
go test -race ./...
gosec ./...
```

#### 质量门槛
- 所有测试通过
- 覆盖率 > 80%
- 无安全漏洞
- 代码审查批准
- 文档完整

## 依赖项

- `github.com/spf13/cobra`：CLI 框架
- `github.com/charmbracelet/bubbletea`：TUI 框架
- `github.com/charmbracelet/lipgloss`：终端样式
- `embed`：嵌入翻译文件
- `encoding/json`：解析 JSON

## 开发注意事项

1. **模式选择逻辑**：无参数 → TUI，有参数 → CLI（由 Registry.Detect 自动判断）
2. **main.go 单一职责**：只负责分发，不处理业务逻辑，保持在 60 行左右
3. **TUI 和 CLI 平等**：两者都是配置源，都修改 Config，都通过 Config.Run() 执行
4. **语言包只在 TUI 使用**：不要在 engine 或其他包中使用 i18n
5. **错误处理**：Engine 返回的错误应该在 TUI 中友好显示，在 CLI 中直接输出
6. **默认值**：所有配置项都应该有合理的默认值
7. **日志记录**：使用 slog 记录到 run.log，便于调试
8. **环境检测**：区分"命令存在"和"服务可用"两种情况
9. **全屏显示**：TUI 使用全屏无边框设计，最大化利用屏幕空间
10. **配色一致**：使用 DPanel 主题配色方案
11. **命令记录**：所有安装过程和最终命令必须完整记录
12. **使用常量**：所有配置值必须使用 `internal/install/constants.go` 中定义的常量，禁止硬编码字符串字面量
13. **任务完成编译**：每次完成重要任务后，必须编译二进制文件到 runtime 目录：`go build -o runtime/dpanel-installer main.go`
14. **代码拆分**：适当拆分代码，把辅助函数（如文件操作、OCI 镜像提取等）放到 `helpers.go`，保持主文件（如 `binary.go`）只包含核心业务逻辑

## 调试技巧

### 查看运行日志
```bash
# 查看实时日志
tail -f run.log

# 查看安装日志
cat install_logs/latest.log
```

### 测试环境检测
```bash
# 测试 Docker 是否可用
docker ps

# 测试 Podman 是否可用
podman ps
```

### 手动执行生成的命令
从安装日志中复制最终的 docker run 命令，手动执行测试。

## Git 提交规范（⚠️ 严格遵守）

本项目遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范，使用统一的提交消息格式。

### 提交消息格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### 提交类型 (type)

| 类型 | 说明 | 示例 |
|------|------|------|
| `fix` | Bug 修复 | `fix(tui):修复语言切换后界面不更新的问题` |
| `feat` | 新功能 | `feat(tui):添加终端窗口自适应宽度支持` |
| `refactor` | 代码重构（不改变功能） | `refactor(tui):统一消息提示样式和间距规则` |
| `docs` | 文档更新 | `docs(claudemd):添加 TUI 布局规范说明` |
| `style` | 代码风格调整（不影响逻辑） | `style(tui):调整变量命名提高可读性` |
| `perf` | 性能优化 | `perf(engine):优化 Docker 命令构建性能` |
| `test` | 测试相关 | `test(engine):添加环境检测单元测试` |
| `chore` | 构建/工具/依赖相关 | `chore(deps):升级 bubbletea 到最新版本` |
| `ci` | CI/CD 配置 | `ci(github):添加自动化测试工作流` |

### 提交范围 (scope)

常用范围：
- `tui`: TUI 界面相关 (`internal/handler/tui/`)
- `engine`: 安装引擎相关 (`internal/install/`)
- `config`: 配置相关 (`internal/install/config.go`)
- `cli`: CLI 命令相关 (`main.go`)
- `i18n`: 国际化相关 (`pkg/i18n/`)
- `claudemd`: 项目文档 (`CLAUDE.md`)

### 提交说明 (subject)

- 使用中文描述（与项目文档语言一致）
- 简洁明了，不超过 50 字符
- 不以句号结尾
- 使用祈使句（如"添加"而非"添加了"）

### 提交正文 (body)（可选）

- 详细说明本次提交的内容
- 列出主要的修改点
- 可以分多条说明，使用列表格式

### 提交脚注 (footer)（可选）

- 关联 Issue：`Closes #123`
- 破坏性变更：`BREAKING CHANGE: API 变更说明`
- 协作者：`Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>`

### 示例提交消息

#### 简单 Bug 修复
```bash
git commit -m "fix(tui):修复禁用选项仍可被选中的问题"
```

#### 复杂功能添加
```bash
git commit -m "feat(tui):添加响应式宽度支持

- 标题、警告框自动换行适应窄窗口
- 最小宽度 40 字符保证可读性
- 使用 getResponsiveWidth() 辅助函数

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

#### 重构提交
```bash
git commit -m "refactor(tui):统一布局、间距和消息样式

本次提交基于布局规范和用户反馈实现全面的 TUI 改进。

Bug Fixes:
- 修复语言切换后界面不更新
- 修复版本选择中禁用选项仍可被选中
- 修复所有步骤顶部间距不一致
- 修复 logo 字符串导致过多垂直间距

Features:
- 添加终端窗口自适应宽度支持
- 添加二进制安装限制：标准版禁用
- 二进制安装时显示警告框

Refactoring:
- 统一消息提示样式（error、warning、info、success）
- 统一所有 TUI 步骤的间距规则
- 清理重复的翻译键
- 移除描述中的内联禁用提示

Documentation:
- 添加"TUI 布局规范（⚠️ 严格遵守）"章节
- 添加"消息提示样式规范（⚠️ 严格遵守）"章节
- 添加"间距规范（⚠️ 严格遵守）"章节
- 添加开发原则："优先使用现有的方法和逻辑"

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

#### 文档更新
```bash
git commit -m "docs(claudemd):添加 Git 提交规范章节

- 说明 Conventional Commits 格式
- 定义提交类型和范围
- 提供多个提交消息示例"
```

### 提交前检查清单

- [ ] 提交类型是否正确？
- [ ] 提交范围是否合适？
- [ ] 标题是否简洁明了（不超过 50 字符）？
- [ ] 是否使用了中文描述？
- [ ] 是否需要详细的正文说明？
- [ ] 是否需要添加 Co-Authored-By？
- [ ] 是否关联了相关 Issue？

### Git 工作流建议

1. **频繁提交**：每个小的功能点或修复都应该单独提交
2. **逻辑分组**：相关修改放在同一提交中，无关修改分开提交
3. **可回滚**：每个提交应该是可独立回滚的
4. **清晰的原子性**：一个提交只做一件事，不要混杂多个不同的修改
5. **测试后再提交**：确保代码能正常工作后再提交

### 查看提交历史

```bash
# 查看最近 10 条提交
git log --oneline -10

# 查看详细提交信息
git show <commit-hash>

# 查看某次提交的文件变更
git show --stat <commit-hash>
```
