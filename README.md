# Kepub

面向 Amp 的 EPUB 阅读、制作预览与编辑工作台。

**当前状态：设计 v0.3，已实现首个 M1-A 只读 Go 核心与 CLI。没有 GUI、编辑操作、安装包或已完成的 Mac 实机测试。**

## 已实现：M1-A

需要 Go **1.27.1**。纯核心只依赖标准库、`golang.org/x/text v0.29.0`（Unicode 碰撞检查）和 `golang.org/x/sys v0.36.0`（原子不覆盖发布），不依赖 MyGo、Amp、Calibre、Java 或前端工具链。

```sh
go build -o kepub ./cmd/kepub
./kepub capabilities --json
./kepub info 'book.epub' --json
./kepub inspect 'book.epub' --section manifest --json
./kepub unpack 'book.epub' --output 'new directory' --json
./kepub info --json -- '-book.epub'
```

- `inspect` 当前支持 `metadata`、`manifest`、`spine`、`capabilities`；`info` 复用相同读取用例。多 rootfile 必须传 `--rootfile '书/Deep/package.opf'`，值是精确 BookPath，不是 URL。
- 仅支持 EPUB2/3 ZIP 与 UTF-8 XML（含 BOM、内建/数字实体）；目录输入、UTF-16、DTD、自定义实体、`xml:base`、远程 manifest href 不支持，明确失败而非容错改写。
- 拒绝穿越、绝对路径、特殊/符号链接/加密 ZIP 条目、重复和大小写/Unicode 碰撞（包括隐式父目录）。实际解压限制：20,000 条目、256MiB/文件、2GiB 总量；路径 4096 字节/128 层；解析 XML 8MiB/128 层/200,000 tokens。这些是初始产品策略，不是 EPUB 标准。
- 原书只读，所有资源（含非 manifest 文件与空目录）保留；unpack 完整 staging 后原子发布，不覆盖已有路径。输出父目录必须已存在。Linux 和 macOS 支持原子不覆盖发布；macOS **只做交叉编译，尚未实机验证**。突然终止可能留下私有临时 staging，但不会发布半成品目录；不宣称断电耐久性。
- `--json` 成功/失败 stdout 都是一个统一 envelope；失败用稳定 code，退出码 1 内容/安全问题、2 参数、3 未实现能力、6 I/O。`--output/-o`、`--rootfile`、`--section` 可置于 BOOK 前后，`--` 结束选项，JSON 和 `--no-input` 不问答。其他契约中的选项尚未实现，会拒绝，不忽略。
- 固定版式、脚本标记、SMIL/音视频、签名、加密/字体混淆声明会记录限制；不执行、不解密、不验证签名、不删除内容。缺失 manifest 资源显式报告。成功只表示只读请求完成，**不是 EPUB 合规验证**。

`toc`、引用图、`validate`、`pack` 留给 M1-B；`doctor`、workspace、plan/apply、GUI、预览、Agent、Mac 发布仍未实现。capabilities 中这些项为 `planned`，不能据此执行。当前 schema 描述查询参数和结果形状，完整 OperationRegistry schema/help 生成及编辑注册表仍待后续完成。

验证：`go test ./...`、`go test -race ./...`、`go vet ./...`；生成的小型 EPUB2/3 与恶意输入样本在测试中创建，无第三方书籍。详细证据见 [M1-A 验证记录](docs/verification/M1_A.md)。

### Orb 启动

`.agents/setup` 为 Linux amd64 orb 固定安装 Go 1.27.1，校验官方 SHA-256，在临时目录完整验证后发布；有模块时从锁文件副本预热/验证缓存，不改仓库锁文件，无模块时跳过。`.agents/resume` 仅检查固定版本与恢复 Go/gofmt 链接，不装依赖、不启动服务。非交互 login shell 可用；没有为后续 Java/Calibre/GUI 预装依赖。Mac 本机请自行安装 Go，此 orb 脚本不是 Mac 安装器。

## 文档

- [开发方案 v0.3](docs/DEVELOPMENT_PLAN.md)：产品范围、MyGo 0.2.0 桌面层、工作区、预览隔离、操作内核、校验与实施阶段。
- [CLI 与操作契约](docs/CLI_CONTRACT.md)：拟定命令、OperationRegistry、plan/apply、机器输出、退出码与验收要求。
- [Calibre CLI 与编辑内核研究](docs/research/CALIBRE_CLI_REVIEW.md)：官方命令全景、关键源码调用链、证据及采用/不采用的设计。
- [开发方案 v0.2 历史原文](docs/history/DEVELOPMENT_PLAN_V0_2.md)：Calibre 研究后形成的上一版设计。
- [开发方案 v0.1 历史原文](docs/history/DEVELOPMENT_PLAN_V0_1.md)：最初的完整方案。

## 产品方向

首发只适配 Apple Silicon Mac，后期再考虑跨平台。桌面端以 **MyGo 0.2.0** 作为当前候选基线：主编辑窗口继续使用 Go + TypeScript + WKWebView，所有网页控制通过新的 `Window.Page()` API；MyGo 纯 Go 原生 UI 只作为独立设置、诊断、检查器等辅助窗口的候选。独立 `kepub` CLI 与 GUI 共用 Go EPUB 核心。

Amp 负责内容理解与编辑；Kepub 提供不依赖模型的确定性 EPUB 操作，并管理工作区、预览、差异、检查、审核和导出。Calibre 是参考和可选适配，不是运行核心功能的必需依赖。

```text
打开 EPUB → 阅读并定位
                ↓
     确定性操作 或 Amp 编辑
                ↓
       候选结果 ↔ 实时预览
                ↓
 差异 + 检查覆盖 → 审核接受 → 导出
```

## v0.3 的关键变化

MyGo 0.2.0 已正式发布。Kepub 因此固定新的桌面接口假设：网页能力从 `Window` 迁移到 `Page`；Preview 的加载、刷新、导航和崩溃恢复统一使用 `Window.Page()`。页面就绪不能只检查 `document.readyState`，而要绑定 workspace/task/generation/bookPath 并完成目标视图握手。

MyGo 新增的纯 Go native UI 在 macOS 使用 Metal/Core Text，但它不是 EPUB 阅读引擎，也不能从“同一应用可混用两种窗口”推导出“同一窗口可任意混排 native UI 与 WKWebView”。首发主窗口继续使用 Web UI + WKWebView；native UI 只作为独立辅助窗口候选。

CEF 没有随 MyGo 0.2.0 正式发布；公开 CEF PR 当前面向 Linux 且仍未合并，因此不进入 Apple Silicon 首发基线。

v0.2 的 Calibre 优化继续保留：转换、整理、结构编辑和只读查询分开；GUI、CLI 与 Agent 共用 OperationRegistry；原始 EPUB、accepted revision 与候选任务分离；`apply`、`accept`、`export` 保持不同语义。

## 基本约束

- 原书默认不覆盖；未修改资源尽量保持原字节，结构修改必须维护引用。
- 书籍页面不可信，不能拥有应用的文件或进程权限。
- 配置、聊天记录、检查点和工具文件不进入 EPUB。
- 正式导出以冻结归档及明确版本的 EPUBCheck 报告为依据。
- 项目名 Kepub 不表示默认输出 Kobo KEPUB，默认仍是普通 EPUB。

## 实施路线

| 阶段 | 目标 |
|---|---|
| M0 | 验证 MyGo 0.2.0 Page API、目标页面 readiness、安全预览、Amp 协议及依赖版本 |
| M1 | EPUB 核心、路径/引用模型、文件级 CLI、覆盖报告 |
| M2 | 工作区与确定性编辑，plan/apply、检查点、审核 |
| M3 | MyGo Page 生命周期、制作预览、Locator 与本机预览服务 |
| M4 | Amp 编辑、工具交接、GUI/CLI审核导出闭环 |
| M5 | Apple Silicon 打包分发和实机验收 |
| M6 | 可选 Calibre 适配及其他受控扩展 |

除上面明确列出的 M1-A 命令外，设计文档中的命令和接口仍待实现；不能将其他设计示例当作当前安装使用说明。
