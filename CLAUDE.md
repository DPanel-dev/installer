# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此代码库中工作时提供指导。

## 项目概述

DPanel 安装器 - 用于安装、升级和卸载 DPanel 容器和二进制文件的工具。安装器支持两种模式：
- **TUI 模式**：通过 `--mode tui` 启动的交互式终端 UI 向导
- **CLI 模式**：通过命令行标志进行程序化调用（默认模式）

**架构原则**：TUI 模式最终路由回 CLI 命令+参数的形式执行安装。

## 构建命令

```bash
# 构建安装器（输出到 runtime 目录）
go build -o runtime/dpanel-installer.exe main.go

# 构建带版本信息
go build -ldflags "-X main.version=1.0.0 -X main.commit=abc123 -X main.date=2025-03-22" -o runtime/dpanel-installer.exe main.go

# 运行 TUI 模式（交互式向导）
./runtime/dpanel-installer.exe --mode tui

# 运行 CLI 模式（默认）
./runtime/dpanel-installer.exe --action install --version community --edition lite

# 查看帮助
./runtime/dpanel-installer.exe --help
```

## 架构

### 入口点 (`main.go`)
- 使用 Cobra 构建命令结构
- **默认行为**：显示 CLI 帮助信息
- **TUI 模式**：通过 `--mode tui` 启动，加载语言包
- **CLI 模式**：通过命令行标志配置，直接执行安装
- **日志记录**：使用 `slog` 将 JSON 日志写入程序同级目录下的 `run.log`
- **CLI 帮助信息**：全部使用英文

### 核心组件

**`internal/install/config.go`**：安装配置
- `Config` 结构体包含所有安装配置项
- `DockerConnection` 结构体包含 Docker/Podman 连接配置
- `NewConfig()` 函数创建带默认值的配置

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

**`internal/ui/tui/tui.go`**：Bubble Tea TUI 实现
- **全屏显示**：无边框，使用整个终端屏幕
- **Claude Code / VS Code Dark 配色**：
  - 主色：`#007ACC` (VS Code 蓝色)
  - 背景：`#1E1E1E` (VS Code 背景)
  - 选中：`#094771` (VS Code 选中色)
  - 文本：`#D4D4D4` (VS Code 前景色)
- **DPANEL Logo**：使用 Unicode 块字符构建的 ASCII 艺术字
  - 在语言选择页面显示
  - 使用 VS Code 蓝色主题
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
├── Action: "install" | "upgrade" | "uninstall"
├── Language: "zh" | "en" (仅 TUI 使用)
├── InstallType: "container" | "binary"
├── Version: "community" | "pro" | "dev"
├── Edition: "standard" | "lite"
├── OS: "alpine" | "debian"
├── ImageRegistry: "hub" | "aliyun"
├── ContainerName: string (默认 "dpanel")
├── Port: int (0 表示随机端口)
├── DataPath: string (默认 "/home/dpanel")
├── DockerConnection:
│   ├── Type: "local" | "tcp" | "ssh"
│   ├── SockPath (local)
│   ├── Host, TLSEnabled, TLSPath (tcp)
│   └── Host, SSHUser, SSHPass, SSHKey (ssh)
└── Network: Proxy, DNS
```

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

### 全屏无边框设计
- 移除了传统的边框样式
- 使用整个终端屏幕
- 添加分隔线区分不同区域
- 更舒适的间距和布局

### 配色方案（Claude Code / VS Code Dark）
- 主色：`#007ACC` (蓝色)
- 成功：`#4EC9B0` (青色)
- 警告：`#CE9178` (橙色)
- 错误：`#F14C4C` (红色)
- 背景：`#1E1E1E` (深色背景)
- 输入框：`#3C3C3C` (输入背景)

### 交互优化
- 清晰的视觉反馈
- 合理的间距
- 明确的操作提示
- 详细的配置描述

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
- ✅ Claude Code / VS Code Dark 配色方案
- ✅ 完整的安装日志记录
- ✅ 最终执行命令的保存
- ✅ Docker 命令构建
- ✅ 支持远程 Docker 连接（TCP/SSH）

### 待实现功能
- ⏳ Upgrade/Uninstall 功能
- ⏳ 二进制安装
- ⏳ TCP/SSH 连接的实际检测
- ⏳ TLS 证书生成
- ⏳ 自动安装 Docker
- ⏳ Windows 平台支持

## 语言使用规范

### CLI 模式
- **所有帮助信息使用英文**
- **所有错误消息使用英文**
- **不加载语言包**

### TUI 模式
- **第一步选择语言**
- **所有界面文本使用语言包**
- **支持英文和简体中文**

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
- **主要文件**：`internal/ui/tui/tui.go`
- **专业领域**：终端 UI 设计、状态管理、VS Code Dark 主题

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

1. **CLI 模式优先**：所有功能应该能在 CLI 模式下工作
2. **TUI 路由到 CLI**：TUI 收集配置后，创建 Config 对象，调用 Engine 执行
3. **语言包只在 TUI 使用**：不要在 engine 或其他包中使用 i18n
4. **错误处理**：Engine 返回的错误应该在 TUI 中友好显示
5. **默认值**：所有配置项都应该有合理的默认值
6. **日志记录**：使用 slog 记录到 run.log，便于调试
7. **环境检测**：区分"命令存在"和"服务可用"两种情况
8. **全屏显示**：TUI 使用全屏无边框设计，最大化利用屏幕空间
9. **配色一致**：使用 Claude Code / VS Code Dark 配色方案
10. **命令记录**：所有安装过程和最终命令必须完整记录

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
