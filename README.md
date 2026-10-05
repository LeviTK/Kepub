# Kepub

面向 Amp 的 EPUB 阅读、制作预览与编辑工作台。

**当前状态：设计 v0.5，第一轮已发布只读 Go 核心与 CLI、工作区库，以及两种独立 Amp 接入实验。第二轮 Medium 的 validate/pack CLI、metadata.set 与 plan/apply/diff 库已本地集成；公共编辑、审核接受和工作区导出闭环正在实现，第二轮尚未发布。没有 GUI、生产 Agent、安装包或已完成的 Mac 实机测试。**

## 已实现：M1-A / M1-B1 只读 CLI

需要 Go **1.27.1**。纯核心只依赖标准库、`golang.org/x/text v0.29.0`（Unicode 碰撞检查）和 `golang.org/x/sys v0.36.0`（原子不覆盖发布），不依赖 MyGo、Amp、Calibre、Java 或前端工具链。

```sh
go build -o kepub ./cmd/kepub
./kepub capabilities --json
./kepub info 'book.epub' --json
./kepub inspect 'book.epub' --section manifest --json
./kepub toc 'book.epub' --json
./kepub inspect 'book.epub' --section references --resource 'EPUB/chapter.xhtml' --direction incoming --json
./kepub unpack 'book.epub' --output 'new directory' --json
./kepub info --json -- '-book.epub'
```

- `inspect` 支持 `metadata`、`manifest`、`spine`、`navigation`、`references`、`capabilities`。`toc` 与 navigation 共用读取用例，按声明选择 EPUB3 nav / EPUB2 NCX，不从 spine 合成目录。多 rootfile 必须传 `--rootfile '书/Deep/package.opf'`，值是精确 BookPath，不是 URL。
- 引用边保留源位置、原 href、精确目标路径、query/fragment 和解析器版本；`--resource` / `--direction incoming|outgoing` 只过滤边，保留全局 coverage 与诊断。CSS 仅提取 literal url/import 子集，完整 CSS grammar 为 partial；脚本、SMIL、srcset 等明确不完整。**complete 只表示相应语法的提取覆盖，不是 EPUB 合规或编辑授权。**
- 仅支持 EPUB2/3 ZIP 与 UTF-8 XML（含 BOM、内建/数字实体）；目录输入、UTF-16、DTD、自定义实体、`xml:base`、远程 manifest href 不支持，明确失败而非容错改写。
- 拒绝穿越、绝对路径、特殊/符号链接/加密 ZIP 条目、重复和大小写/Unicode 碰撞（包括隐式父目录）。实际解压限制：20,000 条目、256MiB/文件、2GiB 总量；路径 4096 字节/128 层；解析 XML 8MiB/128 层/200,000 tokens、累计文本/位置索引 32MiB。这些是初始产品策略，不是 EPUB 标准。
- 原书只读，所有资源（含非 manifest 文件与空目录）保留；unpack 完整 staging 后原子发布，不覆盖已有路径。输出父目录必须已存在。Linux 和 macOS 支持原子不覆盖发布；macOS **只做交叉编译，尚未实机验证**。突然终止可能留下私有临时 staging，但不会发布半成品目录；不宣称断电耐久性。
- `--json` 成功/失败 stdout 都是一个统一 envelope；失败用稳定 code，退出码 1 内容/安全问题、2 参数、3 未实现能力、6 I/O。`--output/-o`、`--rootfile`、`--section` 可置于 BOOK 前后，`--` 结束选项，JSON 和 `--no-input` 不问答。其他契约中的选项尚未实现，会拒绝，不忽略。
- 固定版式、脚本标记、SMIL/音视频、签名、加密/字体混淆声明会记录限制；不执行、不解密、不验证签名、不删除内容。缺失 manifest 资源显式报告。成功只表示只读请求完成，**不是 EPUB 合规验证**。

`doctor`、workspace CLI、plan/apply 命令、GUI、预览、生产 Agent、Mac 发布仍未实现。capabilities 中这些项为 `planned`，不能据此执行。当前 schema 描述查询参数和结果形状，完整 OperationRegistry schema/help 生成及编辑注册表仍待后续完成。

验证：`go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...`；真实 CLI smoke 含 EPUB2/3、双向引用与错误 envelope。小型 EPUB 与恶意输入由测试生成，无第三方书籍。证据见 [M1-A](docs/verification/M1_A.md) 和 [M1-B1](docs/verification/M1_B1.md)。

## 本地已集成：M1-B2 校验与打包

```sh
./kepub validate 'book.epub' --json
./kepub validate 'publication directory' --strict --timeout 30 --json
./kepub pack 'publication directory' --output 'new-book.epub' --json
./kepub pack 'publication directory' --output 'draft.epub' --draft --json
```

正式检查需要 Java 17+ 和官方 EPUBCheck **5.3.0**，通过 `KEPUB_EPUBCHECK_JAR` 指定主 JAR；orb setup 已准备这些依赖。命令不会自动下载，也不会在缺依赖时降级成草稿。`--strict` 使 warning 也阻止通过；显式 `--draft` 产物始终 `verified:false`。

pack 从明确的出版根冻结完整资源清单，不递归归档工作区；正式产物经过最终 ZIP 校验及前后哈希核对，再原子发布，不覆盖已有输出。输出须位于出版根之外。发布前观察到取消不会生成文件；发布成功后到达的取消不撤销或误报已完成的产物。

报告分别保留必需检查和引用提取 coverage；CSS partial 不自行阻止无结构修改的打包，实际 error/fatal 仍阻止。工具缺失、超时、残留受管进程或不完整报告不能记为 pass。进程组回收不是同用户 OS 沙箱，不覆盖脱组进程；合规通过也不代表阅读器渲染或人工可访问性审核。接口、真实 EPUB2/3 fixtures 和边界验证见 [M1-B2](docs/verification/M1_B2.md)。

父 orb 在干净非交互 login shell 中配置真实 EPUBCheck 5.3.0，合并后 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 全通过；validation 分别为 93.575s / 99.354s。Darwin arm64 交叉编译得到 Mach-O，没有执行。普通未配置 checker 的测试会跳过外部检查用例，不能代替上述验证。

## 已实现：M2-A / 本地已集成：M2-B 工作区库

`internal/workspace` 提供 Create/Open/Close、唯一独立候选、Checkpoint/Checkpoints/Restore 和精确内容树 SHA-256。原书、初始版本、候选与检查点均为独立副本；flock 保证协作进程单写者，恢复 journal 处理已记录的中断边界。候选普通写入不会污染基线；已知不支持内容和书内 Agent 配置保留但限制候选创建。

M2-B 库增加 `metadata.set`、Plan/Apply/Diff/Execution。每个计划仅修改一个唯一选中的现有 `dc:title` 或 `dc:creator` 简单文本，匹配预期旧值；目标外 OPF 与资源字节不变，no-op 不重排 XML 或更新时间。计划绑定工作区身份、绝对路径、初始版本、完整树和策略；应用时重新计算，失败回滚，成功仍是 `review_required` / `conformance:not_run`。

这还不是 `kepub workspace` 命令，没有 accept/export 或校验通过状态。后续闭环会将计划基线推进到当前 accepted revision。外部写者必须先停止；flock 与 cwd 不是同用户 OS 沙箱，断电和 ENOSPC 未证明。详见 [M2-A](docs/verification/M2_A.md) 与 [M2-B 验证与限制](docs/verification/M2_B.md)。

## Orb 启动

`.agents/setup` 仅支持 Linux amd64，固定 Go 1.27.1，从根及 SDK 实验的锁文件副本预热/验证 Go 缓存。存在 SDK 实验 npm 锁时，复用精确 Node 26.10.0 / npm 10.9.9，否则校验官方归档后局部安装；`npm ci` 从固定锁和完整性校验缓存准备实验依赖。没有 SDK 锁时不安装 Node/npm。所有工具链通过非交互 login shell 可用，不改系统 Node 或仓库锁文件。

第二轮 setup 另准备 Java 17+ 与官方 EPUBCheck 5.3.0：验证完整下载包，保留全部依赖 JAR，暖运行核对精确文件集合和内容，并持久提供 `KEPUB_EPUBCHECK_JAR`。这只是开发/正式检查依赖，不使只读 CLI 依赖 Java。缺失时才安装 Debian JRE；其安全更新版本不伪称为固定 patch。

`.agents/resume` 只修复 Go 链接与快速检查工具/依赖，不安装、不认证、不启动服务。已有 Go/SDK 的父 orb 补装 Java/checker 为 10.92s，最终暖运行 3.51s、resume 0.57s；额外 JAR 被 resume 拒绝，setup 重装修复 4.73s，干净 login shell 验证通过。证据见 [EPUBCheck 环境验证](docs/verification/ORB_EPUBCHECK.md)，此前 Go/SDK 的空 HOME 测试见 [原 setup 验证](docs/verification/M2_A.md#独立后续sdk-实验的-orb-setup-验证)。不预装 Calibre/GUI，不是 Mac 安装器；第二轮 setup 仍待推送默认分支后才影响未来 orbs，尚未证明新服务端快照。

## 文档

- [开发方案 v0.5](docs/DEVELOPMENT_PLAN.md)：产品范围、MyGo 0.2.0 桌面层、Amp 接入 A/B 实验、工作区与第二轮 Medium 分工。
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

## v0.4：核心继续开发，Amp 接入做对照实验

Go 仍负责出版物、工作区、写租约、候选冻结、审核和导出。Amp 接入并行验证 **A：Go 直接管理 CLI/JSONL** 与 **B：Go 监督 Node/TypeScript SDK 辅助进程，再调用 CLI**，按相同输入/事件约定和取消、异常退出等故障场景比较，最终择一进入生产适配。SDK 不放入 WebView，也不成为只读 CLI 的依赖。

第一轮四条 Ultra 工作线已经完成：M1-B1 目录/引用/覆盖、M2-A 工作区快照与单写者、M0-A CLI 原型、M0-B SDK 原型。实现目录与依赖边界见 [第一轮分工](docs/DEVELOPMENT_PLAN.md#111-第一轮并行工作与文件所有权)。第二轮改用 Medium，先并行完成 M1-B2 校验/打包和 M2-B 元数据计划/候选/diff，再集成审核与导出；范围见 [第二轮计划](docs/DEVELOPMENT_PLAN.md#112-第二轮medium-并行实现随后集成编辑闭环)。原型成功不等于生产 Agent 已接入；真实 Amp 与 Mac 实机验证单独列为门槛。

本轮建议 **A 作为下一轮接入候选，B 暂不晋级**。固定 SDK 实包复现 stderr 排空、退出早于 EOF、原始未结束消息无大小上限的缺口；其测试通过代表成功复现并收敛失败，不是协议门槛通过。A 无需新增运行时，已通过 Linux 受控协议与进程测试，但两者都未验证真实 Amp、配置/权限发现、任意脱组写者或 Mac 回收。证据与离线命令见 [CLI 原型](docs/verification/AMP_CLI_SPIKE.md) / [SDK 原型](docs/verification/AMP_SDK_SPIKE.md)；后者为独立模块，根目录 `go test ./...` 不运行其 Node/Go 测试，须在 `experiments/amp-sdk` 执行 `npm test` 和 `npm run check`。

编辑器集成只借鉴文档/选区/诊断与 Agent 分离的职责，不复刻已停用 VS Code 侧栏或其私有协议。后续如需实时工具交接，再在 OperationRegistry 稳定后评估公开 MCP/插件 API。

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
| M0 | 验证 MyGo 0.2.0 Page API、目标页面 readiness、安全预览、Amp CLI/SDK 两方案及依赖版本 |
| M1 | EPUB 核心、路径/引用模型、文件级 CLI、覆盖报告 |
| M2 | 工作区与确定性编辑，plan/apply、检查点、审核 |
| M3 | MyGo Page 生命周期、制作预览、Locator 与本机预览服务 |
| M4 | Amp 编辑、工具交接、GUI/CLI审核导出闭环 |
| M5 | Apple Silicon 打包分发和实机验收 |
| M6 | 可选 Calibre 适配及其他受控扩展 |

除上面明确列出的 M1 命令、内部工作区库和独立实验外，设计文档中的命令和接口仍待实现；不能将其他设计示例当作当前安装使用说明。
