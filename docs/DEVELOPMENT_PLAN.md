# Kepub 开发方案

> 文档版本：0.4 · 更新日期：2026-10-04
>
> 状态：M1-A/M1-B1 只读核心与 CLI、M2-A 内部工作区库已通过父 orb 的 Linux 组合测试、race 和 vet；两个 Amp 原型仅供对照实验。实际支持范围见 README 与各验证记录。未完成 GUI、生产 Agent 或 Mac 实机验证，不能将后续命令、接口或性能目标当成现有能力。
>
> 首发：Apple Silicon Mac；MyGo 0.2.0 为当前桌面层候选基线，主编辑窗口采用 Go + TypeScript + WKWebView；独立 `kepub` CLI。Amp 为唯一首期 Agent，Calibre 仅作为设计参考和可选外部适配器。

## 0. 本次修订与阅读顺序

v0.4 在 v0.3 的 MyGo 0.2.0 与出版物安全边界上，增加 Amp CLI 直连 / TypeScript SDK 辅助进程的并行验证，以及互不重叠的 Ultra orb 开发分工。两种接入方式不是两套产品；最终只选一条程序化接入路径。已完成的 M1-A 是后续工作的共同代码基线。

v0.2 基于 Calibre 官方 CLI 手册、编辑/转换说明和相关源码修订；v0.3 在此基础上纳入 MyGo 0.2.0 的正式发行变化。Calibre 的研究范围、固定源码版本、证据及不应照搬的实现见 [Calibre CLI 研究](research/CALIBRE_CLI_REVIEW.md)；命令、计划、操作和机器输出见 [CLI 与操作契约](CLI_CONTRACT.md)。[v0.1 原文](history/DEVELOPMENT_PLAN_V0_1.md) 与 [v0.2 原文](history/DEVELOPMENT_PLAN_V0_2.md) 保留供追溯，不再作为优先实施基线。

本次不是将 Kepub 改成 Calibre 的前端。主要调整如下：

| v0.1 | v0.2 决策 |
|---|---|
| Agent 修改文件是主要编辑能力 | 增加独立于模型的确定性 EPUB 操作层，Agent 负责选择操作、解释与内容编辑 |
| 引用查询和安全改名放到后续 | 将引用图、覆盖率、受限安全改名和局部元数据修改前移到 M1/M2 |
| 任务只有 Agent 编辑的主要语义 | 统一 deterministic / agent / external 任务及计划、审核、回滚 |
| 诊断主要呈现问题列表 | 同时记录已检查、被阻断、不适用、未运行、依赖不可用 |
| CLI 是一组入口 | CLI、GUI、Agent 共用 OperationRegistry、参数约束和输出契约 |
| 外部能力尚未展开 | 明确 Calibre 可选适配、运行时依赖、行为探测及导入/导出副作用 |

MyGo 0.2.0 带来的 v0.3 调整：

| v0.2 假设/设计 | v0.3 决策 |
|---|---|
| MyGo 主要作为 Web 前端桌面壳 | MyGo 0.2.0 新增纯 Go `ui` 原生 UI；Kepub 主编辑窗口仍采用 Web 页面 + WKWebView，原生 UI 先用于独立设置、诊断、检查器等辅助窗口的候选实现 |
| 网页能力直接挂在 `Window` | 按 0.2 breaking API 改为 `Window.Page()` 与 `WindowOptions.Page`；原生 UI 窗口的 `Page()` 为 nil |
| 页面加载可用通用 ready 状态判断 | 不用 `document.readyState` 单独判定目标章节已就绪；必须核对目标 generation / bookPath，并等待实际目标文档或元素完成握手 |
| CEF 可作为近期统一渲染后备 | MyGo 0.2.0 正式版未包含 CEF；当前公开 CEF PR 仍未合并且针对 Linux。Apple Silicon 首发继续只以 WKWebView 为基线 |
| dev reload 的进程重叠由应用自行规避 | MyGo 0.2 已改为先停止旧 build 再启动新 build；Kepub 的 workspace 单写者、journal 和进程回收仍由自身负责，不能依赖 dev 行为代替生产数据保护 |

保持不变：Mac-first、MyGo 可替换、纯 Go 核心、Amp CLI 作为执行底座、EPUBCheck 主校验、原书保护、候选审核、书籍预览与本机权限隔离。SDK 只参与 Agent 接入方案比较，不成为 EPUB 核心或只读 CLI 的依赖。

## 1. 产品定位与范围

Kepub 是面向 Agent 的 EPUB 阅读、制作预览与编辑工作台，不是完整 Sigil，也不是新的 Calibre 书库。

```text
打开 EPUB → 建立工作区 → 阅读/定位
                         ↓
              确定性操作 或 Amp 内容编辑
                         ↓
                 候选结果 ↔ 实时预览
                         ↓
              差异 + 检查覆盖 + 用户审核
                         ↓
                  接受 → 校验 → 导出
```

不登录 Amp 也能阅读、检查和执行已实现的确定性操作；不安装 Calibre 也能使用核心功能。模型是可选执行者，不是 EPUB 文件操作的必需依赖。

### 1.1 首发能力

- 仅提供 `darwin/arm64`。最低 macOS 暂以 15 为建议值，M0 根据 release 实测冻结；不同 macOS 的 WebKit 差异仍需测试。
- 未加密、可重排 XHTML 为主的 EPUB 2/3；解析 container、OPF、manifest、spine、EPUB 3 nav / EPUB 2 NCX。
- 原书样式的章节级滚动预览、目录、内部链接、前后章节、返回历史、阅读位置。
- 已有 XHTML/CSS 的 Agent 编辑，以及通过独立验收的确定性操作。
- 任务级差异、整任务接受/拒绝、冻结输入上的检查及独立 EPUB 导出。
- 无界面 CLI 不初始化 MyGo、AppKit 或 WKWebView。

固定版式、Media Overlays、书籍脚本、复杂音视频、签名、DRM、未支持字体混淆必须被识别。无法保证安全保真的输入进入明确的受限只读状态或拒绝编辑，不能删掉不支持内容后声称成功。

### 1.2 不做的事

MVP 不做书库数据库、OPDS/远程书库、多设备同步、邮件发送、设备管理、格式大全、完整分页、完整代码编辑器、内嵌终端、任意插件代码执行或完整阅读系统一致性认证。

项目名 Kepub 不等于 Kobo KEPUB 格式。默认输出普通 `.epub`；不能因为项目同名而自动注入 Kobo 标记、改变扩展名或执行 kepubify。Kobo 输入的识别、保留策略和可选转换另列能力，不把扩展名当充分证明。[研究 §2、§7]

## 2. 技术边界与核心架构

### 2.1 选型

| 层 | 首选 | 约束 |
|---|---|---|
| EPUB 核心 | 独立 Go package | 不依赖 MyGo、Amp、Calibre 或 Java |
| 应用服务 | Go | 命令调度、任务、操作、锁、审核与导出 |
| 桌面壳 | MyGo 0.2.0 候选基线 | 只存在于 desktop 适配；版本在 M0 后冻结，隔离不足时替换预览实现 |
| 主窗口 UI | TypeScript + Vite + React | Web 页面承载 Amp 面板、资源/目录导航和 Preview 容器；窄接口，不直接获得任意文件和进程权限 |
| MyGo 原生 UI | 可选辅助窗口 | `github.com/egoist/mygo/ui`；macOS 使用 Metal 绘制、Core Text 排版。适合设置/诊断/检查器，不是 EPUB 渲染引擎；当前不假设能与 Web Page 任意混排在同一窗口 |
| 预览 | WKWebView + MyGo `Page` API 的隔离出版物视图 | 页面加载、刷新、导航、崩溃恢复统一经过 `Window.Page()`；原书内容与可信 UI 不共享特权 |
| Agent | 并行验证 Go → CLI 与 Go → Node/TypeScript SDK → CLI | 相同任务/事件约定与故障矩阵；择一进入生产适配，不把 SDK 导入 WebView |
| 正式出版物检查 | EPUBCheck 独立进程 | 锁定版本、记录规则与输入哈希 |
| Calibre | 可选外部工具 | 不导入核心；内部 API/Qt 依赖需逐能力验证 |
| 历史 | 不可变 revision + 文件快照 | 不要求用户安装 Git；不以普通硬链接创建可写候选 |
| 本机会话 | 单写者 + Unix socket | 不照搬公网 Content server |

本版已重新核验 MyGo **v0.2.0**：tag `v0.2.0` 指向 commit `d51d2e28dff5b351bc67cf2280b5eef00f1267e8`，该 tag 的 `go.mod` 声明 Go 1.27.1。MyGo 0.2 是 breaking release：网页方法从 `Window` 移到独立 `Page`；新代码不得继续依赖旧的 `win.LoadURL()/win.Reload()/win.OnDOMReady()` 形式，而应先获取非 nil 的 `page := win.Page()`，网页配置放入 `WindowOptions.Page`。

MyGo 0.2.0 的官方 release 不含 CEF。公开的 CEF PR 目前是 Linux 可选 Chromium 方案且仍未合并，所以 Kepub 的 Apple Silicon 首发不把 CEF、Chromium bundle 或 Electron 兼容层写入必需架构。

Go、Amp、Java、EPUBCheck 的旧版本信息仍见 v0.1 来源记录；除本段已核验的 MyGo 外，不用旧信息冒充新版本认证。M0 分别冻结工具版本、commit/checksum、平台和协议样本。

### 2.2 分层

```text
CLI ───────────┐
MyGo GUI ──────┼─→ 应用用例 / 会话所有者
Amp 工具调用 ─┘            │
                    OperationRegistry
                           │
              Plan / Task / Review / Export
                           │
          ┌────────────────┼───────────────────┐
          ▼                ▼                   ▼
   Publication Core   只读 Preview        外部进程适配
   Archive / URL      Handler / Cache     Amp / EPUBCheck
   ReferenceGraph                         可选 Calibre
          │
   Revision / Candidate / Journal
```

CLI 与 GUI 不能互相调用对方的可执行程序来完成核心操作；它们调用相同用例。Amp 可通过 CLI 访问已暴露操作，但不能因此绕过任务审核。MCP 留待同一能力注册表稳定后再做。

### 2.3 拟定代码布局

```text
cmd/kepub/                   # headless CLI
cmd/kepub-desktop/           # MyGo 入口
internal/archive/           # 安全解包、归档文件清单
internal/publication/       # OPF、spine、nav、原始字节与索引
internal/bookpath/          # canonical path / URL / disk path
internal/references/        # 引用图、覆盖率与变更影响范围
internal/operations/        # 注册表、计划、确定性编辑
internal/workspace/         # revision、candidate、checkpoint、journal
internal/session/           # 锁、所有者、IPC、事件与幂等请求
internal/app/               # 统一应用用例、审核、导出
internal/preview/           # 只读资源服务、稳定代际、位置
internal/validation/        # 快检、策略、覆盖与 EPUBCheck
internal/agent/ampcli/       # Amp 协议与进程管理
internal/adapters/calibre/  # 可选、延后实现
internal/desktop/           # MyGo 专用代码
internal/platform/          # 进程组、系统路径、窗口启动
frontend/
testdata/
docs/verification/
```

先用一个 Go module。CI 验证 `cmd/kepub` 的依赖图没有桌面框架；有外部检查器子进程不等于核心需要导入它的运行时。

## 3. EPUB 内容模型：保存原文，重建索引

磁盘出版内容是事实来源，但不能因此放弃结构模型。索引可重建，编辑不能持有过期 DOM 再覆盖 Amp 刚写的新内容。

### 3.1 四类标识分开

- `BookPath`：容器根相对路径、POSIX 分隔符、精确大小写；不是 URL，也不是磁盘绝对路径。
- `Href`：原始引用字符串，包含编码、fragment 和可选 query；保留原值用于最小修改。
- `DiskPath`：仅后端使用，经 root containment 和 symlink 检查解析。
- `ResourceId`：工作区内稳定标识，记录与当前 BookPath 的映射；不要与可变 OPF manifest id 混用。

借鉴 Calibre 的 canonical name 与 URL 互转，而不机械复制其全部标准化行为。保留合法原始名字；NFC/NFD、大小写及编码折叠用于碰撞检查，不默认批量改名。Mac 文件系统能够打开某个错误大小写的 href，不代表该引用在 EPUB 中正确。[研究 §3]

### 3.2 导入与保真

从 `META-INF/container.xml` 解析 rootfile，不扫描到第一个 OPF 就选用。多 rootfile 要显式选择并记录；非标准目录、中文、空格、percent encoding、`linear="no"`、nav 不在 spine 都进入测试矩阵。

未改文件保留原字节。更改 XML/CSS 时优先进行带位置的局部修改；命名空间、未知属性、元数据 refinement、外部词汇和顺序不能被简化 struct 的重序列化吞掉。无法保真的操作应拒绝或报告扩大改写范围，不谎称只改一个字段。

安全解析 UTF-8/UTF-16 和受控合法实体；禁止联网获取 DTD、外部实体与无界展开。严格出版解析与可选容错诊断分离；不能用 HTML 容错解析后自动写回来掩盖 XML 错误。

### 3.3 引用图

记录每条边的源资源、源属性/语法位置、原始 href、解析目标、fragment、引用类别和解析器版本。首批覆盖 OPF manifest/spine、nav/NCX、XHTML `href/src`、CSS `url()/@import`、内联样式及已支持的 SVG 引用；`srcset`、SMIL、其他词汇不能未经实现就声称完整。

每类语法单独报告 coverage。未知脚本、未知 CSS 语法、解析失败或未支持引用只能标成未知；“没有找到引用”不是“已证明没有引用”。删除资源与移除 CSS 规则必须采用保守策略。

`resource.rename` 必须同时处理：文件移动、移动文件内部相对链接重定基准、其他文件的入站链接、OPF/nav/NCX 相关引用、保留 ID 与 fragment。只调用低层 rename 不足以完成此操作。源/目标重叠、循环改名、大小写改名和目标碰撞先做 preflight。[研究 §3.2]

第一版拒绝 OPF 文件本身改名、跨 rootfile 结构重写及引用覆盖不足的改名。安全子集通过测试后逐项扩展，不靠“尽量修复”隐藏遗漏。

## 4. 确定性操作与 Agent 的分工

### 4.1 四类能力不能混用

| 类别 | 作用 | 默认政策 |
|---|---|---|
| inspect | 元数据、目录、资源、引用与问题查询 | 只读，不调用模型 |
| edit | 明确字段/资源/链接的结构修改 | 计划 → 候选 → 检查 → 审核 |
| polish | 显式规则的整理和优化 | 声明副作用，不自动全书清理 |
| convert | 输入格式到另一出版工件 | 可选独立导入流程，不充当保存或修复后备 |

禁止保存时偷偷 EPUB→EPUB 转换。Calibre 转换经过中间表示和变换管线；它的 polish 是另一条尽量小改动的路径。Kepub 借鉴这个职责分离，不承诺 polish 必然字节无损。[研究 §3.1]

### 4.2 OperationRegistry

每项操作有稳定 `operationId`、独立 `operationVersion`、输入/输出 schema、读写范围、风险、网络/模型/GUI 需求、前置条件、支持子集、检查要求、幂等语义、实现状态和回归样本。

注册表生成 CLI help、capabilities 和参数文档；同一 schema 供 GUI 与 Agent 使用。不能把后端某函数存在推导为公共命令可用。未来 `capabilities` 应区分 `planned`、`available`、`unavailable`、`unsupported`，并解释原因。

首批只有：元数据/目录/资源/引用查询；现有唯一元数据字段的局部修改；引用覆盖充分时的单资源改名。自动删除、拆章合章、批量重命名、字体子集、整书 CSS 清理不塞进第一批。

### 4.3 Plan → Apply → Review

`plan` 是无出版内容副作用的预演，可写计划报告；不得趁预演调用模型或外部转换器修改原书。计划绑定工作区、baseRevision、出版文件树哈希、操作实现版本、配置摘要、外部输入摘要、预计写集合和所需授权。

`apply` 重新检查计划与输入，只创建并修改候选任务。它不是接受修改；不得直接替换 accepted revision。失败恢复该操作前检查点，保留失败报告。计划过期、操作版本变化、输入文件变化都必须拒绝。

Amp 可读 inspect/coverage，并提出操作计划；应用在确定的写入交接点执行。任意 shell 正在写候选时不得并行跑核心 mutator。MVP 的 raw CLI 模式需要先停止或结束 Agent 执行再应用结构操作；后续只有经过验证的独占工具调用协议才能在同一 Agent 轮次安全交接。

### 4.4 决策与不可自动化内容

正文改写、章节理解、脚注语义由用户与 Amp 决定。路径引用、字段修改、归档顺序和已支持的安全重命名由代码执行。

改标点、格式化 XHTML、移除软连字符、清理 CSS、字体子集、格式升级都可能改变内容或阅读结果；必须显式选择并展示差异。不能把它们打包成默认“修复全部”。

## 5. 工作区、历史与并发

### 5.1 数据布局

```text
workspaces/<workspace-id>/
  state.json
  original/book.epub
  revisions/<revision-id>/pub/
  plans/<plan-id>.json
  tasks/<task-id>/
    task.json
    work/AGENTS.md
    work/pub/
    checkpoints/
    artifacts/                 # 外部工具的独立输入/输出
    reports/
    changes.json
  preview/<generation>/
  journal/
```

`pub/` 才是 EPUB 根。任务说明、模型对话、Calibre/Java 日志、配置、凭证、检查点和预览辅助代码都不进入 EPUB。

源书包含 AGENTS、`.amp`、MCP 配置时作为不可信输入隔离，不能自动成为 Agent 指令；与合法出版资源冲突且无法安全保持时降级只读。指定 settings 文件不等于关闭上游所有配置发现，必须实测。

### 5.2 统一任务

`Task.kind` 为 `deterministic`、`agent` 或 `external`。所有类型共用 baseRevision、候选副本、状态机、检查点、diff、审核与导出规则。

```text
created → running → freezing → checking → review_required
                ↘ failed/cancelled          ↙       ↘
                                        accepted   rejected
```

失败候选可查看，但不能误标可接受。操作执行成功与出版检查通过是两个状态。原书已有错误不妨碍调查；正式接受默认阻止新引入的 error/fatal，已有错误需显式确认保留且状态仍为未验证有效。无法安全解析或检查范围不足时不得接受。正式导出另外要求最终归档完整检查通过。

每个变更操作先创建跨文件检查点。MVP 用独立复制，不能直接使用普通硬链接：Calibre 的克隆依赖其写 API 主动断开硬链接，Amp 的普通文件写入不会遵守这条约定。[研究 §3.3]

后续 APFS 文件克隆必须经实际内容隔离测试；不把它与硬链接混为一谈。历史淘汰按大小/数量策略进行，必须保护原书、当前 accepted、活动任务和用户固定的检查点。

### 5.3 单写者与共享会话

一个工作区一个会话所有者和可写任务。GUI/CLI 通过受限 Unix socket 共享状态；文件级只读命令不依赖常驻服务。socket 所在目录 0700、socket 0600，并检查会话 nonce/客户端身份和协议版本；不把 PID 存在当作锁。

读锁绑定 revision 或冻结 task generation。第二写入者返回冲突，不偷偷产生竞争副本。每个视图有独立 locator；不得以一个全局“当前章节”串扰多本书。

接受时停止受管写进程，冻结候选，重算哈希并校验 baseRevision。状态与 revision 指针用同步落盘、原子替换、journal 保证恢复。断网、取消、磁盘满和崩溃必须保留原书与可恢复的已接受内容。

这些是数据保护设计，不是针对同一用户权限下恶意进程的 OS 沙箱保证。

## 6. 预览与编辑上下文

### 6.1 MyGo 0.2 窗口与 Page 合约

第一阶段主编辑窗口仍采用 Web 页面：可信应用壳负责 Amp 面板、目录/资源导航、状态与审核；EPUB 正文由其内部的隔离出版物视图显示。MyGo 0.2 新增的原生 UI 以“另一个窗口的内容类型”进入设计，而不是替代 WKWebView 的 EPUB 渲染。

创建网页窗口后，所有网页操作统一走 `Page`：

```go
win := mygo.NewWindow(mygo.WindowOptions{
    Title: "Kepub",
    URL:   "/",
    Page: mygo.PageOptions{
        DevTools: mygo.DevToolsDisabled,
    },
})

page := win.Page()
if page == nil {
    return errors.New("Kepub main window has no web page")
}
page.OnDOMReady(func() {
    // 这里只表示当前顶层页面触发 DOMContentLoaded，
    // 不能单独证明目标 EPUB generation 已经可交互。
})
```

原生 UI 窗口通过 `WindowOptions.Content` 创建，其 `Page()` 返回 nil。MyGo 0.2 文档证明同一个应用可以同时拥有 Web 页面窗口和原生 UI 窗口；当前方案**不据此假定同一窗口中可以任意混排 native UI 与 WKWebView**。在上游出现稳定的同窗组合 API 并完成 Kepub 实测前，设置、诊断、检查器若采用原生 UI，均作为独立辅助窗口或后续替换实验。

Preview readiness 必须绑定 `workspaceId + taskId + generation + bookPath`。不得仅轮询 `document.readyState === "complete"`：MyGo 自己的 macOS GUI 测试已经出现“初始空文档先返回 complete，目标页面尚未 commit”的竞态。Kepub 应采用：

1. 后端生成目标 generation nonce，并先记录预期 `bookPath`。
2. 顶层可信壳等待自身 `Page.OnDOMReady`。
3. 出版物 frame/视图加载指定 generation；目标文档实际存在后发送最小 `PreviewReady{generation, bookPath}` 握手。
4. 前端和后端都核对 nonce、当前视图及 bookPath；旧 generation 的晚到事件直接丢弃。
5. 只有握手通过后才恢复 Locator、滚动位置、选区或允许自动化点击。
6. `Page.OnRenderProcessGone` 触发时，将当前预览标记 unavailable/stale，重新建立同一稳定 generation，而不是悄悄切换到候选最新文件。

release 构建默认关闭 DevTools；需要调试时显式开启。页面方法和事件只在非 nil `Page` 上调用，测试同时覆盖 Web 主窗口和无 Page 的 native 辅助窗口，避免将两种窗口类型混用。

### 6.2 可信壳与不可信出版内容

MyGo 自定义协议的顶层页面不能被当成天然安全沙箱。可信应用 UI 与出版物 frame/独立无桥 WKWebView 分离；禁书籍脚本、远程请求、表单、弹窗和顶层导航。每个 Go 服务仍校验窗口、工作区和操作授权。

M0 必须验证 release 下 frame IPC、`file:`/`about:` 导航、SVG/嵌套文档、伪造消息、开发 localhost 信任和 CSP。隔离不成立即更换适配，不允许带风险进入后续 UI 开发。

### 6.3 资源与稳定代际

GUI scheme 和 CLI loopback HTTP 共用只读 PublicationHandler。每次访问验证 workspace/generation/BookPath；真实缺失返回错误，禁止 SPA fallback，禁止读出版清单以外的文件及符号链接逃逸。

XHTML 使用正确媒体类型；资源 MIME、字体、SVG、图片与 range 请求通过样本验证。不要为显示破损书籍普遍改成 `text/html`。

目录树 watcher 处理原子替换、新目录、移除、事件合并和重扫。事件只是线索，以文件哈希决定变化，发布稳定只读 generation；OPF、nav、CSS 和媒体修改都参与失效。中间 XML 失败时保留上一代预览并明确显示过时，不闪出绿色状态。

300–500ms 合并窗口只是初始调优值。依赖不明确时保守刷新当前章节。只有当前 workspace/task/generation 的事件可影响视图。

### 6.4 定位与排版诊断

统一 Locator：`bookPath + fragment + progression`，并绑定 revision/generation；无法可靠定位时返回失效，不跳到另一文件的同名 ID。CLI 首期 `--at 'EPUB/Text/ch01.xhtml#note1'`，更复杂 CSS/CFI/text quote 定位后续实现。

点击预览给 Amp 的应是有限上下文：文件、位置、选区或明确元素，不把整棵 DOM 或本机路径自动发送给模型。选区能力需隔离脚本验证，未通过前只传章节。

制作模式保留原 CSS；阅读字号/夜间模式是用户覆盖层，不写回源书。后续可参考 Calibre 的样式来源检查，展示匹配规则/计算样式，但计算样式不反向覆盖原 CSS。[研究 §4]

### 6.5 浏览器预览服务

只监听 loopback；会话令牌不可预测，校验 Host/Origin，限制 CORS，日志脱敏且不加载远程追踪。GET 不能编辑，控制走独立本机 IPC。服务关闭撤销令牌。浏览器预览与 WKWebView 不作像素一致承诺。

## 7. 校验：问题列表之外必须有覆盖范围

### 7.1 分层

| 检查层 | 目的 | 权威性/依赖 |
|---|---|---|
| archive / parse | 安全容器、XML、文件清单 | Go 快检；不等完整 EPUB 一致性 |
| references / structure | href、ID、manifest、spine、导航 | 明确支持语法与覆盖范围 |
| policy / change review | 越界写、正文变化、任务权限 | Kepub 策略，不能冒称 EPUB 规范 |
| compatibility | 可选 Calibre/目标阅读器问题 | 单独来源，不覆盖 EPUBCheck |
| conformance | 冻结目录和最终 EPUB | EPUBCheck，记录版本、规则、原始报告 |
| rendering / accessibility review | 实际阅读与人工可访问性审核 | 不由工具零错误自动证明 |

每个 checker 返回 `passed / failed / blocked / unavailable / not_run / not_applicable`，以及输入哈希、规则版本、前置检查、耗时和诊断。顶层报告另给 `pass / fail / incomplete`。严重 XML 错误阻断引用检查时，不能把引用错误数零解读成引用正确。[研究 §3.4]

### 7.2 修复提案

诊断保留来源、上游 code、severity、BookPath、可得行列和稳定标识。修复建议指向明确 operation/version、参数、前置哈希、风险和需重跑的检查，而不是直接执行诊断中的自然语言。

可复用报告必须匹配内容树哈希、checker 二进制/规则版本和策略摘要。保留 baseline 与新增/消失问题，但“没有新增错误”不等于出版物有效。自动修复最多两轮，错误集合无进展则停止，不能无限消耗模型调用。

### 7.3 EPUBCheck 与规范

制作目标、OPF `package@version` 和检查规则分别保存。EPUB 3 package version 不因制作目标 3.3 而写成 3.3。选择实际工具支持的规则，无法证明规则版本时记录 unknown，不通过虚构参数伪装。

正式 export 对最终冻结归档运行 EPUBCheck；error/fatal 阻止，warning 提示，`--strict` 使 warning 也阻止。工具缺失/超时/不完整报告不能是 pass。草稿导出独立命名并附警告，不自动作为正常导出的降级路径。

Calibre 内部 CSS checker 在本次核查版本依赖 QtWebEngine；可选调用必须探测环境，不能把它放入轻量 headless CLI 的必需路径。[研究 §3.5]

## 8. Amp 与可选 Calibre 适配

### 8.1 Amp

首期以本机 Amp CLI 为执行底座，接入层按 §8.1.1 同时验证两个候选。Go 应用服务始终管理 workspace/task、写租约、候选冻结、审核和导出；不把这些责任交给 SDK。进入生产前必须核验并锁定 CLI、所选适配器及其运行时版本。prompt 经 stdin 或 SDK 消息输入，参数数组启动，不拼接任意 `sh -c`；同时读取 stdout/stderr，设置消息上限、兼容未知字段、测试分块 UTF-8/异常 EOF/终止事件。

线程 ID 显式绑定 workspace/task/cwd/revision，不继续“最近线程”。普通终端 `kepub amp` 保留原生 TUI，必须有 TTY；GUI 用结构化事件，不要求 Ghostty 或终端模拟器。

注册操作可以减少模型重复实现，但 raw Amp 仍可拥有当前用户 shell 权限。cwd、AGENTS 和候选副本不是 OS 沙箱。不得默认跳过权限，首次启用说明书籍片段可能发送服务端；本地进程不代表离线模型。

受管进程组统一取消、宽限、强杀和回收。接受前确认已无写入进程；关闭窗口不得悄悄删除活动目录。工具调用与候选数据都不自动拥有 accepted/export 权限。

### 8.1.1 两种接入方案用同一门槛比较

| 方案 | 原型目录 | 执行路径 | 需要证明 |
|---|---|---|---|
| A：Go 直连 | `experiments/amp-cli/` | Go → Amp CLI 的 execute/JSONL | 协议解析、stdin 流、错误分类、进程组取消及版本兼容可维护 |
| B：TypeScript SDK | `experiments/amp-sdk/` | Go 进程监督 → Node 辅助进程 → `@ampcode/sdk` → Amp CLI | SDK 减少的适配成本大于新增 IPC/运行时成本，取消可穿透辅助进程，干净机器可打包 |

TypeScript SDK 不是浏览器 SDK，也没有移除 CLI 依赖。Node 包、锁文件和测试留在原型目录；不把 Node 引入 `cmd/kepub` 的普通查询路径，不为比较重写 Go 出版物核心。方案 B 可以在自己的目录提供最小 Go 监督程序，不修改方案 A。两个原型均不注册生产 `capabilities`，也不提前开放 `kepub amp` 或 `task run`。

共同实验约定（版本 1，仅用于对照，不是新的公共 CLI）：

- 宿主通过 stdin JSONL 发 `start`：`schemaVersion:1`、`type:"start"`、`requestId`、`workspaceId`、`taskId`、`baseRevision`、显式绝对 `cwd`、`prompt`，续接时另带明确 `threadId`。上下文可附顶层字段 `bookPath/fragment/progression/selectedText/generation`；不自动读取或上传整书，也不把内容字符串解释为进程参数。
- 两个原型发送相同的显式上下文：模型文本先保留原 `prompt`，仅在提供上下文字段时追加 `\n\nKepub task context (JSON):\n` 和 JSON 对象。该对象只含上述五个已提供字段，不加入未知字段、内部绑定或自动文件内容。三项文本必须为字符串；`progression` 为 null 或 0～1，`generation` 为 null 或非负 int64；保留空字符串与显式 null，缺省字段不补入上下文。公共事件的缺省 generation 仍为 null。
- `cancel` 控制消息含 `schemaVersion:1`、`type:"cancel"`、同一 `requestId`。首批每个辅助进程只处理一个任务；不以会话池扩大比较范围。
- stdout 事件采用 CLI 契约的 envelope：`schemaVersion/requestId/workspaceId/taskId/sequence/generation/type/data`。`sequence` 从 1 严格递增，`generation` 未提供时为 null，日志只进 stderr。实验事件类型为 `started/assistant/tool/completed/failed/cancelled`；terminal 恰好一个。
- `completed` 只在协议成功结果、完整结束及成功进程退出均成立后发出，必须含 `data.reviewRequired:true`；不表示已接受或已通过出版物检查。缺失终止消息、异常退出或取消后仍可能写入都不能报告可冻结成功。正常执行结束后若仍有活写者必须终止，清理成功也应报告失败，而不是把被中断的工具结果升级为 completed。
- terminal 的 `data` 统一包含 `reviewRequired`、`code`（成功为 null，失败/取消为稳定字符串）、`threadId`（未知为 null）、`cleanup:{scope:"process-group",confirmed:boolean}`。无法确认受管进程组清理时报告 `failed` / `CLEANUP_UNCONFIRMED`，不发 `completed`；此范围不涵盖通过 setsid 等方式脱离进程组的同用户进程，不是 OS 沙箱或所有写者已停止的证明。
- 默认使用明确本机执行和 private 线程可见性；不使用 `continue:true`、不自动开启全权限。续接上下文由 Go 宿主复核，SDK 的消息 requestId 去重不能替代出版物事务幂等性。
- 原型宿主通过启动参数 `--bindings FILE` 提供可信续接绑定，文件形状为 `{"schemaVersion":1,"threads":[{"threadId":"…","workspaceId":"…","taskId":"…","cwd":"绝对路径","baseRevision":"…"}]}`。续接前逐项匹配，缺失/不匹配则拒绝；不能用 start 消息里的自我声明生成信任。新线程无需预存绑定，返回 ID 后由宿主登记；绑定文件是宿主输入，不从书籍或模型消息发现。
- 自动验证使用临时候选与 fake CLI/受控 SDK 测试替身，不调用付费模型、不发送用户书籍。SDK 原型须另证明实际固定版本 SDK 的加载和参数映射，不能仅测试自写 mock 后宣称 SDK 已联通。

两条线分别提供同名场景和预期结果：分块 UTF-8、流式输入、未知字段、stderr 并行输出、消息超限、无 result 的 EOF、success result 后非零退出、取消前/后及重复取消、带迟延写入的子进程清理、明确 threadId 续接。报告区分协议/进程已测、SDK 实包已测、真实 Amp 未测、Mac 未测；Linux fake 结果不能证明真实 Amp 子进程或 macOS 退出语义。

评选顺序为：数据保护和取消门槛 → 协议与权限可验证性 → Apple Silicon 打包可行性 → 维护成本。安全门槛相同且收益不明显时选 A；B 若显著减少可靠对话适配工作且通过运行时/回收门槛，可替换生产 Agent 适配层。初轮实验只给证据和建议，不默认同时长期维护两个生产后端。

### 8.1.2 借鉴编辑器边界，不复刻旧 VS Code 扩展

IDE 上下文桥与 Agent 调用适配是两个职责：前者传递当前文档、选区、诊断和受控操作，后者负责发起执行、收取事件及取消。Kepub 对应的上下文是 BookPath、Locator、generation、用户选区与检查覆盖；出版物视图不直接持有 SDK、凭证、进程或任意文件权限。

Amp 已宣布停用旧 VS Code/Cursor 侧栏扩展；其内部架构不能由旧商店链接推断。公开 `amp.nvim` 的本机鉴权桥可作历史设计参考，但也已停止维护。Kepub 不冒充 VS Code、不复制私有 IDE 发现协议。首期通过明确任务输入传上下文；OperationRegistry 和写入交接稳定后，再评估公开 MCP/插件 API，不能因收到工具调用事件就认定已取得候选写租约。

依据：[SDK 概览](https://ampcode.com/docs/sdk)、[TypeScript API](https://ampcode.com/docs/sdk/typescript)、[CLI IDE 连接](https://ampcode.com/docs/cli#connect-an-editor)、[扩展停用公告](https://ampcode.com/news/the-coding-agent-is-dead)、[已弃用 Neovim 实现](https://github.com/ampcode/amp.nvim/tree/01ede44322220da5dc0b73ad8ace328a5ec1f5bf)。以上支持职责划分，不是 Kepub 真实 Agent 联调或 Mac 实测证据。

### 8.2 Calibre：可选，按能力而非按安装状态

通过绝对路径、版本与对应命令 help 探测。macOS app bundle 内路径只是一个候选，不强依赖用户 shell PATH；用户可设置安装位置。

第一版不把 Calibre 作为必要依赖。后续适配先选择一个小功能做 fixture 验证，再开放下一项。`ebook-polish` 的输出是第二位置参数，`-o` 是 metadata OPF 输入；`ebook-convert` 的选项依赖输入/输出格式。不能统一给所有工具塞同一套 flags。[研究 §2、§3.1]

外部操作流程：

```text
冻结输入 → artifacts/input.epub → 指定外部工具与参数
 → artifacts/output.epub → 安全重新导入
 → 全文件清单/哈希差异 → 规则与范围审查
 → 候选结果 → 用户审核
```

工具只获得副本路径；退出 0 不代表格式正确，文档声称小改动也不豁免 diff。来源路径、工具版本、argv（脱敏）、输入/输出 hash 与报告都入任务记录。

`ebook-edit`/`ebook-viewer` 是 GUI 启动入口，不是 headless 操作；`calibredb check_library` 不是 EPUBCheck。内部 Python API 只能经独立 helper 和版本适配调用，不将其当稳定、纯 Python 的嵌入库。

Kepub 自己的 schema 吸收外部差异，不把私有函数名作为公开 ABI。分发前审查源码/二进制许可与依赖；本次不复制 Calibre 实现、不替项目选择许可证、不默认捆绑整个 Calibre。

## 9. CLI 与可复用工作流

详细定义见 [CLI_CONTRACT.md](CLI_CONTRACT.md)。保留 v0.1 的文件级、workspace、task、preview 命令，新增 `capabilities`、`inspect`、`plan`、`apply`；`info` 和 `toc` 作为同一读用例的便捷入口，而不是另一套逻辑。

操作层增长不等于每种能力增加一个顶层命令。metadata、rename、未来 polish 使用稳定 operation ID 和显式参数文件。GUI 菜单、CLI、Agent 请求统一经过应用服务。

机器输出为单 JSON envelope 或有界 JSONL 事件；stdout 不混日志。错误 code 不依赖中文/英文文本。可写请求要有明确 workspace/plan/task，不猜当前书、最新线程或最新计划。

后续工作流是版本化、有限步骤的声明式操作序列，不是任意 Python、JavaScript 或 shell 脚本。默认无网络；外部输入有哈希，允许的操作来自本机注册表。导入一份工作流文件不等于授权执行。

## 10. 打包、保真与安全

### 10.1 归档

独立归档器从批准的出版清单打包，不递归压缩整个 workspace。保留合法输入的未变资源及明确登记的非 manifest 文件；工具新生文件必须登记，不能靠文件名黑名单当全部防线。

`mimetype` 第一项、STORE、精确内容、无 BOM/换行、无加密和 extra field；用二进制 ZIP header 测试验证库行为。EPUB 3 修改时间只在确有已接受内容变更且策略要求时更新，纳入最终检查；不能每次 inspect/preview 都修改源书。

无改动 round-trip 要区分：出版条目内容字节保持不变，和 ZIP 压缩字节完全一致。需要原始归档字节时直接保留 original；重新打包不默认保证容器二进制一致。

输出同目录临时归档→校验→hash→原子发布。默认不覆盖输入及已存在文件；显式覆盖还要验证源文件未被外部修改并保留备份。

### 10.2 不可信输入防护

拒绝路径穿越、绝对路径、符号链接/特殊文件逃逸、重复 entry、大小写/Unicode碰撞和超限解压。边解边计数，不能信任 ZIP header 的大小声明。

初始防护阈值仍为 20,000 文件、单文件 256MiB、总展开 2GiB，属于待测产品策略，不是 EPUB 标准限制。XML 深度/展开、进程时间/输出、HTTP 大小和 JSON 消息同样设上限。

encryption.xml 存在不直接判为 DRM；识别算法，字体混淆与 DRM 分开处理。不支持算法、签名或必须保留但无法理解的结构时拒绝危险变更。书内文本、链接、配置和工作流不能自动升级成程序指令。

## 11. 开发阶段与通过条件

不承诺固定工期；M1-A 已完成首批只读查询与安全导入，其余按下列门槛推进。M0 的 Linux 协议实验与纯 Go 核心可并行，MyGo/WKWebView 的 Apple Silicon release 隔离仍是桌面集成门槛，不能被浏览器或交叉编译替代。

| 阶段 | 交付 | 必须通过 |
|---|---|---|
| M0 风险验证 | MyGo 0.2.0 release 隔离与 Page API、Amp A/B 接入实验、EPUBCheck、版本矩阵 | 主窗口按 `Window.Page()` 工作；目标 generation readiness 无空白页竞态；不可信书页不可调用 Go；两方案按同一矩阵比较，真实继续/取消另行实测；依赖与规则可追溯 |
| M1 纯核心 | 安全归档、Publication/BookPath、只读引用图、file CLI、注册表元数据 | 不起 GUI、不需 Amp/Calibre；无改动条目字节保持；partial coverage 显式 |
| M2 确定性编辑 | 工作区/锁/快照、plan/apply、局部 metadata、安全 rename 子集 | 原子跨文件变更；过期计划拒绝；API外写不污染基线；失败回滚 |
| M3 制作预览 | MyGo `Page` 生命周期、隔离视图、稳定代际、Locator、CLI serve | CSS/资源更新可见；页面 commit/readiness 可验证；render process 异常可恢复；中间坏状态不误导；本机服务无写越权 |
| M4 Amp 闭环 | 原生 TUI/JSONL、内容编辑、范围审查、计划交接、审核导出 | GUI/CLI共同闭环；无并发写竞态；停止后冻结检查；输出绑定报告 |
| M5 Mac 发布 | arm64 app/CLI、依赖发现、签名公证、干净机器验收 | 不要求 Go/Bun；缺可选依赖仍可用；无未解决数据破坏或越权 |
| M6 可选扩展 | 一项 Calibre adapter、polish操作、工作流等逐项开放 | 每能力锁版本、fixtures、隔离、diff、失败恢复；不能扩大MVP前置 |

首批任务编号延续 K-001～K-013 的职责，新增：

- K-014：BookPath 精确匹配与引用覆盖模型，纳入 M1。
- K-015：OperationRegistry、capabilities 与生成式 CLI 文档，纳入 M1/M2。
- K-016：metadata/rename 的 plan/apply 与并发交接，纳入 M2。
- K-017：检查依赖/coverage 与绑定式 FixProposal，纳入 M1/M4。
- K-018：可选 Calibre adapter spike，纳入 M6，不阻塞核心发行。
- K-019：MyGo 0.2.0 desktop spike：`Window.Page()`、`PageOptions`、readiness nonce、`OnRenderProcessGone`、native UI 独立辅助窗口和 release 隔离，纳入 M0/M3。

上述是任务标识，不代表已创建 Issues。不要一次性生成整套应用后才补安全测试。

### 11.1 第一轮并行工作与文件所有权

按用户最新要求，后续四条工作线全部由独立 Ultra orb 承担，覆盖此前的 Medium 分配。保留已经由 Medium 完成的 M1-A 代码和验证证据，不重复重写；集成线程维护计划、共同基线和验收结果。四个 orb 各自拥有明确模块，不是把整个应用复制四份同时开发。

| 工作线 | 本轮交付 | 独占修改范围 | 暂不进入本轮 |
|---|---|---|---|
| P1：M1-B1 只读结构 | EPUB3 nav / EPUB2 NCX、toc、只读引用图及逐语法 coverage | `internal/publication/`、`internal/references/`、必要的 `internal/bookpath/` 和 `internal/testfixture/`；`internal/app/`、`cmd/kepub/` 的对应入口；`docs/verification/M1_B1.md` | pack、EPUBCheck 适配、编辑和 GUI |
| P2：M2-A 工作区基础 | 原书保留、初始 revision、独立候选与检查点、内容树哈希、单写者锁和失败清理 | `internal/workspace/` 及其内部平台文件/测试；`docs/verification/M2_A.md` | 对外 workspace CLI、socket 服务、plan/apply、accept/export |
| P3：M0-A CLI 原型 | §8.1.1 的 Go 直连实验与故障证据 | `experiments/amp-cli/`、`docs/verification/AMP_CLI_SPIKE.md` | 生产 Agent 注册和真实书籍任务 |
| P4：M0-B SDK 原型 | §8.1.1 的 Node/SDK 辅助进程、监督及故障证据 | `experiments/amp-sdk/`（含独立 package/锁文件）、`docs/verification/AMP_SDK_SPIKE.md` | 生产 Node 依赖与 GUI 接入 |

P2 只依赖本轮冻结的 M1-A 读 API，不等待 P1 的新接口；P3/P4 使用相同文字契约和一次性候选 fixture，不等待工作区实现。新增代码可在独占目录内组织，不为并行人为抽出共享大框架。跨所有权目录、根 go.mod/go.sum、README、主方案、CLI 契约和 orb setup 的变更先由集成线程协调，避免分别改出不一致契约。

各 orb 从包含 M1-A 与本版方案的同一 Git 基线开始。远端尚未包含的提交通过文件传输工具传 Git bundle，再建立各自分支；线程消息中的 SHA 不等于代码已同步。每条线在自己的 checkout 中实现、修复和验证后回报路径、测试与限制；集成线程检查差异并运行组合测试。未通过验收的 prototype 不提升 capabilities，未推送的工作不声称远端已交付。

本轮已合入本地的核心成果：P1 的 nav/NCX、toc、引用图和逐语法 coverage；P2 的独立候选、内容树哈希、检查点/恢复与协作单写者锁。父 orb 已执行组合 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 及真实 CLI smoke。workspace 仍是内部库，未开放命令或审核/导出。P3/P4 保留为独立实验，不提升生产能力；协议和进程证据见各自报告。

下一轮在本轮验收后安排 M1-B2 的校验/正式打包、M2-B 的计划/审核用例，以及选定 Agent 适配。预览开发仍受 M0 Mac 隔离门槛约束；不把 Linux orb 的并行数量当作目标平台验证。

## 12. 验收矩阵

| 场景 | 预期证据 |
|---|---|
| MyGo 0.2 Web 主窗口 / native 辅助窗口 | Web 窗口 `Page()` 非 nil、native 窗口为 nil；不调用错误类型 API；release 下两者生命周期稳定 |
| 页面首开、连续切章、快速 reload、旧 generation 晚到、WebContent 崩溃 | 不以初始空文档 ready 冒充目标已就绪；只接受当前 nonce；崩溃后恢复相同稳定 generation |
| EPUB2/3、深层 OPF、多 rootfile、nav 不在 spine | rootfile选择准确，阅读顺序与元数据保留 |
| 中文、空格、`%`/fragment、错误大小写、Unicode碰撞 | 解析与落盘分离；不因Mac宽容而漏报 |
| 跨目录改名 | 入站引用、移动文件出站引用、nav/OPF/CSS同时正确；无范围外改动 |
| 循环/碰撞/OPF改名/未知引用 | 首批明确拒绝，不半执行 |
| 无改动、只读检查、仅改一个字段 | 不擅自格式化/更新时间；未变资源字节相同 |
| Agent直接写文件 | parse缓存失效；检查点和accepted不受普通写影响 |
| 严重XML错误 | 依赖检查显示blocked，不是虚假的零问题通过 |
| 未知CSS/动态内容 | unused仅候选，不自动删除 |
| 未退出进程、过期plan、两个CLI写入 | busy/conflict，有恢复日志，无盲覆盖 |
| 外部工具退出0但输出异常 | 重新导入/检查失败，不能接受或正式导出 |
| 无Calibre/Qt、无Amp、无Java | 按能力降级，unavailable与pass区分 |
| 存在书内AGENTS/远程URL/嵌套SVG | 不启用其权限，不加载远程追踪，不越过预览隔离 |
| 最终保存 | 标准ZIP顺序、清单、报告hash、原子发布和原书保护 |
| 文档与CLI | 注册表schema/help/JSON输出契约一致，示例覆盖错误路径 |

Go 单测/fuzz、fake Amp/Calibre进程、固定 EPUBCheck fixtures、前端状态测试分别运行。浏览器测试不代替 Apple Silicon release 实测；CI 不默认调用付费模型，公共 PR 不获得发布凭证或书籍原文。

性能沿用待测目标：固定20MiB/约200章样本首章显示≤3秒，稳定文件变化到预览完成P95≤1秒，常见取消≤3秒进入确定状态。分别统计GUI/WebKit/Amp/Java及可选Calibre，不引用空应用内存当产品指标。

## 13. 来源与维护

Calibre 相关事实仍定位在 [研究报告](research/CALIBRE_CLI_REVIEW.md)：官方 CLI 页面及说明文章、固定 Calibre commit `a1864306758688f62f386043a2b19cf3cbf38d6a` 的源码。在线手册与源码版本可能不同，均未据此宣称目标 Mac 已运行通过。

MyGo v0.3 文档修订依据固定在 2026-10-03 发布的 **v0.2.0**（tag/commit `d51d2e28dff5b351bc67cf2280b5eef00f1267e8`）：
- Release：<https://github.com/egoist/mygo/releases/tag/v0.2.0>，列出 native UI、Page API、dev reload 与 GUI test 修复。
- 文档入口：<https://github.com/egoist/mygo/blob/v0.2.0/docs/README.md>。
- Native UI：<https://github.com/egoist/mygo/blob/v0.2.0/docs/ui.md>；macOS 使用 Metal，文字使用系统 Core Text。
- Window/Page API：<https://github.com/egoist/mygo/blob/v0.2.0/docs/windows.md>；breaking 迁移说明见 PR #24：<https://github.com/egoist/mygo/pull/24>。
- 页面加载竞态修复：PR #26 <https://github.com/egoist/mygo/pull/26>，说明初始空白文档可先满足 `readyState === complete`，因此 Kepub 的 readiness 设计不能只依赖该状态。
- dev reload 生命周期：PR #22 <https://github.com/egoist/mygo/pull/22>。
- CEF 当前不是 0.2.0 正式能力；公开 PR #21 <https://github.com/egoist/mygo/pull/21> 仍未合并且目标为 Linux 可选 CEF。
- v0.2.0 `go.mod`：<https://github.com/egoist/mygo/blob/v0.2.0/go.mod>，声明 Go 1.27.1。

既有 Amp/EPUB 来源入口保留于 [v0.1 来源记录](history/DEVELOPMENT_PLAN_V0_1.md#18-来源与核验记录)，v0.2 的 Calibre 设计原文保留于 [历史文档](history/DEVELOPMENT_PLAN_V0_2.md)。EPUB 规范与检查器的实施基线仍需 M0 锁定，参考 [EPUB 3.3](https://www.w3.org/TR/epub-33/)、[Reading Systems](https://www.w3.org/TR/epub-rs-33/) 和 [EPUBCheck CLI](https://www.w3.org/publishing/epubcheck/docs/cli/)。

v0.3 修订只记录了上游 MyGo 0.2.0 事实。v0.4 的实际证据见 [M1-A](verification/M1_A.md)、[M1-B1](verification/M1_B1.md)、[M2-A](verification/M2_A.md)、[CLI 实验](verification/AMP_CLI_SPIKE.md) 和 [SDK 实验](verification/AMP_SDK_SPIKE.md)。不得据此宣称目标 Mac 实机、MyGo 预览、真实 Amp 集成、Calibre/EPUBCheck 或全部性能与安全矩阵已通过。
