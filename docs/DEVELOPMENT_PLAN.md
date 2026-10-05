# Kepub 开发方案

> 文档版本：0.7（开发计划；新接口暂定）· 更新日期：2026-10-05
>
> 路线：CLI + 外部 Amp 协作 → MyGo UI → UI 内集成 Amp。CLI 与 UI 共用 Go EPUB 核心；受管 Amp 不是 UI 前置条件。现进入 C0/C1/C2 开发，不自动启动真实模型调用或发布。
>
> 状态：只读核心与 CLI、工作区库及 Amp 对照实验已发布到默认分支；validate/pack、单字段元数据编辑、候选审阅、接受／拒绝与工作区导出已本地集成，尚未发布。GUI、生产 Agent、安装包和 Mac 实机验收未完成。实际支持范围以 README、capabilities 和验证记录为准。
>
> 目标平台：Apple Silicon Mac；Linux orb 用于开发验证。MyGo 与系统 WebView 已确认，当前接口基线为 MyGo 0.2.0，macOS 使用 WKWebView，不捆绑 Chromium。React + TypeScript + Vite 仍是前端计划。UI 暂缓不取消选型，也不让 WebView 验收阻塞纯 CLI 开发。

## 0. 本次修订与阅读顺序

v0.7 将当前方向拆成可验收批次：先完善独立 CLI，增加受限正文读写，让外部 Amp 通过 CLI 完成编辑；随后交付不依赖模型的 MyGo UI；最后接入 UI 内的 Amp 任务。受管 Agent 从 UI 的前置条件移到后续阶段。近期不引入 SDK、MCP、常驻 Agent 服务或完整资源改名作为 CLI 前置。

优先阅读 §1 的三方职责、§8.1 的两种 Amp 协作方向，以及 §11.3～§11.6 的批次、编辑边界、Medium 分工和验收。M0～M6 保留为技术工作包编号，不代表执行先后；§11.1／§11.2 保留已完成轮次的记录。本文不冻结新命令的最终 schema，也不代替实现、模型调用、推送或发布授权。

v0.6 的 Obsidian 官方 CLI、Amp External API v2 与 TypeScript SDK 核对记录继续保留：§9.1／§9.2 记录 CLI 借鉴，§8.1.3 区分工具调用、Agent 执行和工作区管理 API；参考文档不等于引入对应依赖。

v0.5 将第二轮目标限定为单字段元数据编辑闭环：安全打包与校验、局部修改与候选差异先并行，再接入审核接受和导出。该闭环现已本地集成；实际限制见 §11.2。第一轮 Ultra 分工仅保留作历史记录。

v0.4 在 v0.3 的 MyGo 0.2.0 与出版物安全边界上，增加 Amp CLI 直连 / TypeScript SDK 辅助进程的并行验证，以及互不重叠的 Ultra orb 开发分工。两种接入方式不是两套产品；最终只选一条程序化接入路径。

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

当前确认：Mac-first、MyGo + 系统 WebView、纯 Go 核心、Amp CLI 作为执行底座、EPUBCheck 主校验、原书保护、候选审核、书籍预览与本机权限隔离。MyGo 留在 desktop 适配层，不渗入出版物核心；隔离能力不足时补充或调整 MyGo／WKWebView 预览适配，不重新引入捆绑浏览器。SDK 只参与 Agent 接入方案比较，不成为 EPUB 核心或只读 CLI 的依赖。

## 1. 产品定位与范围

Kepub 是面向 Agent 的 EPUB 阅读、制作预览与编辑工作台，不是完整 Sigil，也不是新的 Calibre 书库。

三方职责固定为：**CLI 是可脚本调用的操作入口，UI 是阅读与审核入口，Amp 是理解需求和提出编辑的智能执行者。** 文件、计划、候选、检查和导出规则由共享 Go 核心负责；只有新增并通过验收的操作才能被任一入口使用。

首阶段用户直接在 Amp Code 中提要求，Amp 调用 Kepub CLI 并读取 JSON；Kepub 不启动 Amp。后续 UI 直接调用 Go 用例，不反向解析 CLI 文本。下面的实时预览和自由 Agent 文件编辑是完整产品目标，不是现有能力。

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

CLI 不登录 Amp 也能查询、检查和执行已实现的确定性操作；后续阅读 UI 同样不应要求模型登录。不安装 Calibre 也能使用核心功能。正式检查仍需要 Java／EPUBCheck；模型是可选执行者，不是 EPUB 文件操作的必需依赖。

### 1.1 CLI 优先的首批交付

- 整理并交付现有查询、安全解包、validate/pack、workspace、plan/apply、diff、accept/reject 和 export，不从头重写 CLI。
- 当前仅能修改一个现有 `dc:title` 或 `dc:creator` 简单文本；新 C2/C3 批次拟增加绑定版本的内容读取与已有 XHTML 的简单文本修改。正文编辑未实现，不能把当前元数据演示当成该目标完成。
- 稳定 JSON envelope、错误码、非交互行为、能力描述和依赖诊断；补齐 `doctor`、安装使用说明及对应测试。这些补齐项仍未实现。
- 先用现有元数据闭环做 Amp 小样本联调，再在 C3 通过后验证正文编辑；所有写入走 CLI 注册操作，用户明确决定 accept/reject/export。不依赖 SDK、MCP、常驻会话服务或桌面窗口。
- CSS 可纳入受限只读内容查询；CSS 写入、跨节点排版修改、资源新增／删除／改名、批量操作及任意候选文件写入不进入首个正文编辑版本，另行冻结支持子集并验收。
- Linux 验证可先进行；面向 Apple Silicon 的 CLI 正式发布仍须实机安装和运行验证。Darwin 交叉编译不算 Mac 验收，CLI 检查也不替代视觉排版审核。

### 1.2 后续桌面目标

- 仅提供 `darwin/arm64`。最低 macOS 暂以 15 为建议值，M0 根据 release 实测冻结；不同 macOS 的 WebKit 差异仍需测试。
- 未加密、可重排 XHTML 为主的 EPUB 2/3；解析 container、OPF、manifest、spine、EPUB 3 nav / EPUB 2 NCX。
- 原书样式的章节级滚动预览、目录、内部链接、前后章节、返回历史、阅读位置。
- 已有 XHTML/CSS 的 Agent 编辑，以及通过独立验收的确定性操作。
- 任务级差异、整任务接受/拒绝、冻结输入上的检查及独立 EPUB 导出。
- 无界面 CLI 不初始化 MyGo、AppKit 或 WKWebView。

固定版式、Media Overlays、书籍脚本、复杂音视频、签名、DRM、未支持字体混淆必须被识别。无法保证安全保真的输入进入明确的受限只读状态或拒绝编辑，不能删掉不支持内容后声称成功。

### 1.3 不做的事

MVP 不做书库数据库、OPDS/远程书库、多设备同步、邮件发送、设备管理、格式大全、完整分页、完整代码编辑器、内嵌终端、任意插件代码执行或完整阅读系统一致性认证。

项目名 Kepub 不等于 Kobo KEPUB 格式。默认输出普通 `.epub`；不能因为项目同名而自动注入 Kobo 标记、改变扩展名或执行 kepubify。Kobo 输入的识别、保留策略和可选转换另列能力，不把扩展名当充分证明。[研究 §2、§7]

## 2. 技术边界与核心架构

### 2.1 选型

| 层 | 首选 | 约束 |
|---|---|---|
| EPUB 核心 | Go 1.27.1 独立 package | 当前依赖标准库、x/text v0.29.0、x/sys v0.36.0；不依赖 MyGo、Amp、Calibre 或 Java |
| 应用服务 | Go | 命令调度、任务、操作、锁、审核与导出 |
| 桌面壳 | MyGo（已确认），0.2.0 为当前接口基线 | 只存在于 desktop 适配；M0 实测生命周期、隔离与打包，不能将框架选型当成验证通过 |
| 主窗口 UI | TypeScript + Vite + React（计划，未实现） | Web 页面承载 Amp 面板、资源/目录导航和 Preview 容器；窄接口，不直接获得任意文件和进程权限 |
| MyGo 原生 UI | 可选辅助窗口 | `github.com/egoist/mygo/ui`；macOS 使用 Metal 绘制、Core Text 排版。适合设置/诊断/检查器，不是 EPUB 渲染引擎；当前不假设能与 Web Page 任意混排在同一窗口 |
| 预览 | 系统 WebView（已确认），macOS 为 WKWebView | MyGo 页面操作通过 `Window.Page()`；出版物隔离容器待 M0 验证，不能把默认第二窗口当作无桥视图；原书内容不得获得可信 UI 权限 |
| Agent | 先 Amp → Kepub CLI；后续受管接入优先 Go → Amp CLI | 前者是工具使用，后者才比较 CLI／SDK 适配；均未完成真实 Amp 验证；SDK 暂不进入生产 |
| 正式出版物检查 | Java 17+ + EPUBCheck 5.3.0 独立进程 | 已接入；记录工具、规则与输入哈希，缺依赖不自动转草稿 |
| Calibre | 可选外部工具 | 不导入核心；内部 API/Qt 依赖需逐能力验证 |
| 历史 | 文件系统 + JSON + SHA-256 + 独立快照 | 不需要数据库或用户安装 Git；不以普通硬链接创建可写候选 |
| 本机会话 | 当前逐请求文件锁；后续评估受限 Unix socket | 常驻会话和写入交接未实现；不作为首批 CLI 的前置条件 |

本版已重新核验 MyGo **v0.2.0**：tag `v0.2.0` 指向 commit `d51d2e28dff5b351bc67cf2280b5eef00f1267e8`，该 tag 的 `go.mod` 声明 Go 1.27.1。MyGo 0.2 是 breaking release：网页方法从 `Window` 移到独立 `Page`；新代码不得继续依赖旧的 `win.LoadURL()/win.Reload()/win.OnDOMReady()` 形式，而应先获取非 nil 的 `page := win.Page()`，网页配置放入 `WindowOptions.Page`。

MyGo 0.2.0 的官方 release 不含 CEF。公开的 CEF PR 目前是 Linux 可选 Chromium 方案且仍未合并，所以 Kepub 的 Apple Silicon 首发不把 CEF、Chromium bundle 或 Electron 兼容层写入必需架构。

Go、Java／EPUBCheck 与实验 SDK 的实际版本及运行证据见 README 和各验证记录；生产 Amp CLI 版本仍待真实联调冻结。实验 SDK 的 Node／TypeScript 锁不等于桌面前端版本选择。orb setup/resume 负责 Linux 开发依赖，不是 Mac 安装器。

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

CLI 与 GUI 不能互相调用对方的可执行程序来完成核心操作；它们调用相同 Go 用例，GUI 不解析终端文本代替业务接口。Amp 可通过 CLI 访问已暴露操作；`apply` 不自动接受，但显式 `accept` 尚不鉴别人类与 Agent 调用者，权限边界见 §8.1。MCP 留待同一能力注册表稳定后再评估。

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

生产核心先用一个 Go module；SDK 对照实验保留现有独立模块。持续检查 `cmd/kepub` 的依赖图没有桌面框架；有外部检查器子进程不等于核心需要导入它的运行时。以上仍是目标布局，不为凑齐目录提前创建空实现。

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

当前仅支持 UTF-8 XML 及内建／数字实体，UTF-16、DTD 和自定义实体明确拒绝；扩展支持另行设计和验证。禁止联网获取外部实体与无界展开。严格出版解析与可选容错诊断分离；不能用 HTML 容错解析后自动写回来掩盖 XML 错误。

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

当前已实现元数据/目录/资源/引用查询和有限单字段修改。引用覆盖充分时的单资源改名仍是后续能力，不阻塞首批 CLI 交付。自动删除、拆章合章、批量重命名、字体子集、整书 CSS 清理不塞进第一批。

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

失败候选可查看，但不能误标可接受。操作执行成功与出版检查通过是两个状态。原书已有错误不妨碍调查；当前正式接受要求必需检查通过，error/fatal 阻止接受，没有既有错误豁免或草稿接受。未来是否支持保留已有错误另行决策，不能据旧设计绕过当前门槛。正式导出另外要求最终归档完整检查通过。

每个变更操作先创建跨文件检查点。MVP 用独立复制，不能直接使用普通硬链接：Calibre 的克隆依赖其写 API 主动断开硬链接，Amp 的普通文件写入不会遵守这条约定。[研究 §3.3]

后续 APFS 文件克隆必须经实际内容隔离测试；不把它与硬链接混为一谈。历史淘汰按大小/数量策略进行，必须保护原书、当前 accepted、活动任务和用户固定的检查点。

### 5.3 单写者与共享会话

当前 CLI 每次打开工作区取得排他锁，请求结束释放；同一工作区只有一个活动候选。后续 GUI／受管 Agent 才需要长期会话所有者：若宿主持锁，Amp 子进程不能再独立打开同一工作区作为第二写者。必须先选择并验证会话所有者转发请求或受控锁交接，不能直接套一层进程启动。

受限 Unix socket 是后续共享会话的候选接口，尚未实现；文件级只读命令不依赖常驻服务。若采用，socket 所在目录 0700、socket 0600，并检查会话 nonce/客户端身份和协议版本；不把 PID 存在当作锁。

读锁绑定 revision 或冻结 task generation。第二写入者返回冲突，不偷偷产生竞争副本。每个视图有独立 locator；不得以一个全局“当前章节”串扰多本书。

接受时停止受管写进程，冻结候选，重算哈希并校验 baseRevision。状态与 revision 指针用同步落盘、原子替换、journal 保证恢复。断网、取消、磁盘满和崩溃必须保留原书与可恢复的已接受内容。

这些是数据保护设计，不是针对同一用户权限下恶意进程的 OS 沙箱保证。

## 6. 预览与编辑上下文

### 6.1 MyGo 0.2 窗口与 Page 合约

进入 UI 阶段后，主编辑窗口采用 Web 页面：可信应用壳负责 Amp 面板、目录/资源导航、状态与审核；EPUB 正文由其内部的隔离出版物视图显示。MyGo 0.2 新增的原生 UI 以“另一个窗口的内容类型”进入设计，而不是替代 WKWebView 的 EPUB 渲染。此节不作为 CLI 开发的前置条件，但仍是出版物预览交付门槛。

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

MyGo 与系统 WebView 已确认采用，但出版物隔离容器尚未实现。可信应用 UI 与不可信书页必须分离；禁书籍脚本、远程请求、表单、弹窗和顶层导航。每个 Go 服务仍校验窗口、工作区和操作授权，不把原书 HTML 直接放入可信应用 DOM。

Bridge 是系统 WebView 的脚本通信适配，不是另一套渲染引擎，与 WKWebView 不存在这一意义上的冲突。需要验证的是谁能调用 bridge，以及 Go 服务允许调用者做什么；不能通过关闭整个可信 UI 的 JavaScript 来解决书页权限问题。

固定 v0.2.0 源码有以下边界，不能由“系统 WebView”或“另开窗口”推断安全：

- 公共网页窗口创建路径会[注入 MyGo bridge](https://github.com/egoist/mygo/blob/v0.2.0/window.go#L441-L449)，没有公开的关闭 bridge 选项。[macOS 实现](https://github.com/egoist/mygo/blob/v0.2.0/internal/darwin/window.go#L216-L240)默认只向顶层 frame 注入，并在原生消息入口拒绝非主 frame；不是向每个 iframe 注入。每窗独立 secret 和 `WKUserContentController` 不等于无桥，也不证明独立存储或进程隔离。
- [默认信任规则](https://github.com/egoist/mygo/blob/v0.2.0/window.go#L967-L995)包含 `file:`、`about:` 和应用注册的自定义协议。空 `TrustedOrigins` 不是拒绝全部；不能直接用注册的书籍 scheme 加载顶层不可信书页，再假定 IPC 会拒绝它。main-frame 限制是已有防护之一，不是整套预览隔离的证明。
- `Bind` 注册全局服务，不是每窗独立授权。服务可用 `CallerWindow(ctx)` 区分窗口，但同一可信窗口被替换为书页，或同源子 frame 借用 `parent.mygo` 时，仅检查窗口 ID 不足以授权；必须同时限制可信文档、导航和出版物容器。
- [`OnWillNavigate`](https://github.com/egoist/mygo/blob/v0.2.0/window.go#L1396-L1400)不覆盖应用主动的 `LoadURL`，也不是通用子资源网络拦截器。受控资源 handler、CSP 和导航策略须分别覆盖外部 CSS／字体／图片、嵌套内容及应用自身发起的导航。

已执行的集成边界实验见 [MyGo boundary](../experiments/mygo-boundary/README.md)。固定上游真实 core + fake backend 的开发／发布模式普通和 race 测试，复现了内建协议信任、第二窗全局绑定和同窗替换的风险，并验证外部 HTTPS、错误／跨窗 secret 拒绝及导航 context 取消。父 Linux orb 另在 WebKitGTK 4.1（2.50.6）中执行真实原生测试：上游七项 bridge/frame/CSP/生命周期 E2E 在开发与发布模式均通过；新增发布模式用例复现无 sandbox 的同源 `srcdoc` 经 `parent.mygo.call` 调用 Go，且调用者仍为宿主窗口。`sandbox="allow-scripts"` 且不加 `allow-same-origin` 的对照明确返回 `SecurityError`、调用计数不增加；无脚本 sandbox 观察到加载完成且无脚本执行。`script-src 'none'` 阻止内联脚本，但宿主 `Page.Eval` 仍可调用 bridge，不能把 CSP 当成卸载桥接。测试通过包含危险配置的成功复现，不是这些配置可用于生产。

M0 在已选框架内验证受限 frame 或专门配置的 WKWebView；若需要真正无桥视图或独立 data store，必须补充并验证相应原生适配，不能声称默认 MyGo API 已提供。隔离容器的同窗布局能力也须实测，不因另一个 Web 窗口可创建就视为完成嵌入。

Apple Silicon release 验收必须覆盖 frame IPC、`file:`/`about:` 导航、自定义协议信任、SVG/嵌套文档、伪造消息、开发 localhost 信任、CSP 和实际网络请求。隔离不成立就阻止不可信出版物预览交付，修复适配后再验收；浏览器测试或 Darwin 交叉编译不能替代。

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

先让 Amp 使用 Kepub，再让 Kepub 管理 Amp。两个方向的责任不同，不能把 §8.1.1 的适配器 A/B 比较误读成必须先实现受管 Agent 才能使用 CLI。

**近期：Amp → Kepub CLI。** 用户在 Amp 中提出要求，Amp 经 shell 调用同一环境内的 `kepub`，读取 JSON 和退出码。先 `capabilities` 确认能力，再 `inspect` 获取实际元数据；生成显式操作请求后，经 `workspace open → plan → apply → task diff` 形成可审阅候选。用户审阅后明确决定 `task accept/reject` 与 `workspace export`。请求／计划报告位于整个工作区之外，操作使用实际返回的 task ID，不猜最近任务。此路径不需要新增 SDK、MCP 或私有 IDE 协议；Amp、二进制、书籍及检查依赖必须位于同一可访问环境，orb 不会自动取得用户本机文件。

当前只开放有限 `metadata.set`；不能让 Amp 自由改写既有 deterministic 候选后沿用原执行记录接受。候选漂移可供 diff/reject，但不能绕过来源和输入绑定。C3 的受限正文修改将作为新的确定性操作走同一计划／候选／审核流程，即使参数来自模型也不必启动受管 Agent。任意 XHTML/CSS 文件编辑才需要后续 Agent 任务、写入交接和冻结协议，不以手工修改内部状态代替实现。

**后续：Kepub → Amp CLI。** Go 应用服务创建候选、绑定任务和线程、启动并监督 Amp，执行结束后进入差异与检查，仍不自动接受。优先将已有 Go 直连实验推进为受管适配；SDK 实验暂不进入生产。进入该阶段前先解决 §5.3 的锁交接和下述审批边界；`kepub amp`、`kepub task run` 均仍未实现，命令形态尚未冻结。将来 TUI 入口要求 TTY，程序化入口使用结构化事件，不要求内嵌终端。

受管接入需锁定真实 CLI 版本及协议样本。prompt 经 stdin，参数数组启动，不拼接任意 `sh -c`；并行读取 stdout/stderr，限制消息大小并测试分块 UTF-8、异常 EOF 和非零退出。线程 ID 显式绑定 workspace/task/cwd/revision，不继续“最近线程”。受管进程组统一取消、宽限、强杀和回收；停止写入后才能冻结，`result:success`、进程退出 0、检查通过和用户接受是四个不同状态。进程组回收不证明脱组或远程写者已停止。

**审批与权限边界尚未实现为强隔离。** 现有 CLI 的 `apply` 不自动接受，但 `task accept` 不能鉴别人类还是 Agent 调用者。同用户 shell 可以直接执行该命令；“请先询问用户”、工作目录、AGENTS 和候选副本都不是 OS 权限边界。近期流程按合作式使用记录限制；若要承诺 Agent 无法自行接受，必须另行设计受限执行接口及模型不可取得的审批授权，并约束其旁路文件／shell 权限，不能只隐藏一个工具名。

任务说明来自可信宿主目录，书内 AGENTS、MCP 和插件配置不得自动获得配置身份。不得默认开启全权限；真实模型调用及可发送内容先获明确授权，首轮仅使用合成 EPUB，不能用用户书籍替代 fixtures。本机进程不代表离线模型。官方接口依据：[execute mode](https://ampcode.com/docs/cli/execute-mode)、[Streaming JSON](https://ampcode.com/docs/cli/streaming-json)；官方文档不是 Kepub 真实联调通过的证据。

### 8.1.1 两种接入方案用同一门槛比较

本节保留已经完成的受管接入对照实验及晋级条件；它不阻塞先验证 Amp 调用现有 Kepub CLI，也不要求继续并行维护两个生产后端。

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

**本轮证据支持 A 进入下一轮验证，B 暂不晋级。** 固定 SDK 实包在 stderr 排空、exit 早于 EOF 和原始未结束消息上限处出现可复现缺口，AbortSignal 也不能单独回收工具写者。Go 监督能收敛这些失败，但没有消除 SDK 内部的缺口。A 对同名边界及时分类失败，不依赖任务超时；超过其公开 1MiB stderr 上限时主动失败，不能表述为无限量排空优于 B。两种路径均只证明受管 PGID，不证明任意 setsid/远程/外部写者已停止。

原型继续留在 experiments 作为对照证据，不注册生产能力。进入受管 Agent 阶段时优先锁定 A 的 CLI 版本与真实 JSONL、权限/配置发现、明确线程续接及 Apple Silicon 生命周期；真实执行必须另行授权，不发送用户书籍替代 fixtures。固定 SDK 的公共 `requestId` 会序列化成 CLI wire `request_id`，两原型现按此对齐；真实 CLI 接受/去重行为仍待实测，不能将对象层文档当成已验证的 wire 行为。

### 8.1.2 借鉴编辑器边界，不复刻旧 VS Code 扩展

IDE 上下文桥与 Agent 调用适配是两个职责：前者传递当前文档、选区、诊断和受控操作，后者负责发起执行、收取事件及取消。Kepub 对应的上下文是 BookPath、Locator、generation、用户选区与检查覆盖；出版物视图不直接持有 SDK、凭证、进程或任意文件权限。

Amp 已宣布停用旧 VS Code/Cursor 侧栏扩展；其内部架构不能由旧商店链接推断。公开 `amp.nvim` 的本机鉴权桥可作历史设计参考，但也已停止维护。Kepub 不冒充 VS Code、不复制私有 IDE 发现协议。首期通过明确任务输入传上下文；OperationRegistry 和写入交接稳定后，再评估公开 MCP/插件 API，不能因收到工具调用事件就认定已取得候选写租约。

依据：[SDK 概览](https://ampcode.com/docs/sdk)、[TypeScript API](https://ampcode.com/docs/sdk/typescript)、[CLI IDE 连接](https://ampcode.com/docs/cli#connect-an-editor)、[扩展停用公告](https://ampcode.com/news/the-coding-agent-is-dead)、[已弃用 Neovim 实现](https://github.com/ampcode/amp.nvim/tree/01ede44322220da5dc0b73ad8ace328a5ec1f5bf)。以上支持职责划分，不是 Kepub 真实 Agent 联调或 Mac 实测证据。

### 8.1.3 External API、CLI 与 TypeScript SDK 的适用边界

以下按 2026-10-05 官方文档及公开 [OpenAPI v2](https://ampcode.com/api/v2/openapi.json) 核对，属于接口研究，不是认证或真实模型联调。

| 接口 | 官方用途与约束 | Kepub 的选择 |
|---|---|---|
| Amp 调用 Kepub CLI | Agent 经 shell 使用本机已有工具；Kepub 输出自己的 JSON／错误码 | 第一阶段 C 批次首选；不需要调用 Amp External API 或安装 SDK，也不必先开发 MCP |
| Amp CLI／[TypeScript SDK](https://ampcode.com/docs/sdk/typescript) | 程序主动运行 Agent；SDK 的 `execute()` 返回 `AsyncIterable<StreamMessage>`，仍调用 Amp CLI，要求 Node.js 18+ | 第三阶段 A1 的执行适配；Go 直连优先，SDK 作为可重评的 Node 辅助进程，不放进 MyGo WebView |
| [Amp External API](https://ampcode.com/api/external) | 工作区范围的线程／消息／用量／成员查询、项目仓库检索及组／模型提供商等管理；目前仅开放 M2M 应用创建，Sign in with Amp 尚未普遍开放 | 不作为编辑执行后端；本次公开 schema 没有发送 prompt／启动 Agent 的端点。今后确有组织审计需求时再评估，不要求普通 CLI 用户申请工作区应用 |

External API 文档明确把 Amp 产品自动化指向 CLI／SDK。它的线程消息仅部分标识字段稳定，不能把历史消息列表当执行事件协议；工作区 M2M 凭据也不等同于 SDK 的用户访问令牌，不能嵌入桌面包或发给书页。未来只申请所需 scopes，不因能查全工作区线程就自动读取无关对话。

SDK 值得借鉴的部分是输入生成器、类型化流消息、显式线程续接和取消接口。实际接入还需以下限制：

- 本机编辑显式选择 `executor:'local'`、候选任务工作目录及 `visibility:'private'`；文档默认 visibility 是 `workspace`，private 也仍可能对工作区管理员可见，不等于离线或零服务端保存。凭据留在可信宿主，日志和 EPUB 不含凭据。
- `continue` 使用已经核对 workspace/task/baseRevision 的明确 thread ID；不使用 `true` 选择最近线程。`createUserMessage(..., {requestId})` 的重试去重只涉及同线程用户消息，不使 `apply`／接受／导出变成可任意重试的事务。`session_id` 到持久 thread ID 的对应关系须真实验证，不能凭字段名推断。
- `system/assistant/user/result` 由适配层转换为 Kepub 任务事件；`result` 是模型执行结果，不是出版物 JSON 报告或 EPUBCheck 证书。不照搬示例中看到成功 result 就 `break` 的完成判断，还须确认执行结束、受管写者停止及候选冻结。
- `AbortSignal` 是取消请求，不是所有工具后代停止的证明。首版本机任务仍由 Go 监督；远程 orb／runner 的本地进程只负责传回结果，杀死它不证明远端任务结束。
- SDK 文档中 orb／runner 只接受字符串 prompt，不能沿用本机流式输入；`enabledTools`、`skills`、`mcpConfig` 等本地选项在远程会被忽略并警告。`cwd` 只设置本地 CLI 子进程目录；runner 使用已服务的 `runnerDir`，续接保持既有目录。远程候选上传、版本绑定、取消确认、产物取回和重新校验必须另做协议，不随 `executor` 切换自动获得。
- 工具权限按官方插件／执行环境机制验证；`enabledTools` 是工具选择，不是文件系统沙箱。若以后依赖 execute 模式的插件生命周期，还须核对插件就绪时序，不能只配置名称就宣称授权已生效。

当前 SDK 实验证据只针对锁定的 `0.1.0-20260918210405-g81edbf0` 与对应 CLI。在线文档不是该发行包已经修复 stderr／退出时序／原始消息上限的证明，也不能据旧实验断言所有后续 SDK 均有这些缺陷。重评时固定 SDK／CLI／Node 组合并重跑 §8.1.1 同一矩阵；本次不升级依赖或重新执行模型任务。认证、版本兼容和权限说明另见 [SDK 概览](https://ampcode.com/docs/sdk) 与 [execute mode](https://ampcode.com/docs/cli/execute-mode)。

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

### 9.1 参考 Obsidian CLI，但保持真正 headless

[Obsidian CLI 官方说明](https://obsidian.md/help/cli)及[官方文档原文](https://raw.githubusercontent.com/obsidianmd/obsidian-help/master/en/Extending%20Obsidian/Obsidian%20CLI.md)描述的是运行中桌面应用的控制入口：需要启用 CLI，应用未启动时首条命令会启动它；另有 TUI。其独立 Headless Sync 不等于全部 CLI 命令均脱离桌面运行。这里参考公开行为，不声称核验了其闭源 CLI 的内部实现、IPC 或构建工具链。

| Obsidian 的公开设计 | 借鉴到 Kepub | 不照搬的部分 |
|---|---|---|
| `help`／`commands`，按文件、属性、链接、历史分组 | 可发现命令与能力，明确读／写／外部依赖；后续 help/schema 从同一命令描述产生 | 不为形式一致改成 `property:set` 或 `key=value`；保留现有子命令／`--option` 语法 |
| `vault=`、精确 `path=`，也允许当前 vault／active file 和名称解析 | 显式 BOOK、工作区目录、BookPath、task ID；阅读位置另有 Locator | 自动化不猜活动窗口／文件、不按模糊文件名写书；Obsidian workspace 是窗口布局，不是 Kepub 的版本／候选工作区 |
| 查询可选 JSON，`search:context` 返回带位置的匹配，链接可双向查询 | 保持统一 JSON envelope、稳定错误码、已有 incoming/outgoing 与 coverage；后续内容读取／检索需有范围和位置 | 不把所有命令都视为同一 JSON schema；正文检索目前未实现，不增加隐式整书上传 |
| diff／history 与属性修改能被脚本调用 | Agent 先查能力和元数据，再显式 plan/apply/diff，用户决定接受／拒绝 | 不把可直接修改 Markdown 的语义套到 EPUB；结构引用、清单及正式导出仍由核心校验 |
| `dev:screenshot`／`dev:dom`／`dev:css`／错误日志帮助 Agent 观察 UI | 桌面阶段评估受限的预览就绪、截图与排版诊断，并绑定 BookPath／generation | 不开放任意 `eval`、插件执行或照搬 Electron CDP；WKWebView 调试接口与安全策略需单独实现 |
| 应用注册 CLI 到 PATH；无参数可进入 TUI | 提供可查版本、安装路径与缺依赖说明；将来 GUI 可帮助注册独立 CLI | 首批不要求启动 MyGo、不做 TUI、不安装 Obsidian；orb/CI 无显示服务也能运行核心命令 |

因此目标是“应用能力可由命令调用”，不是“命令必须通过桌面应用运行”。未来只有预览／窗口上下文命令需要 GUI 或会话所有者；文件和确定性编辑入口仍可独立运行，共享 Go 用例和同一校验政策。

### 9.2 在现有 Go CLI 上增量构建并供 Amp 使用

当前 `cmd/kepub/main.go` 是自有参数解析、分派和 envelope 输出；工作区命令调用 `internal/app/workspace.go` 的共享用例。`internal/app/app.go` 提供 capabilities 和初步 schema，但 help 文本另行维护，尚未成为完整的生成式注册表。根模块没有 Cobra 等第三方 CLI 框架。先解决现有契约一致性，不为借鉴 Obsidian 重写解析器或加入 Node；若以后更换解析库，单独评估现有语法和错误优先级的兼容性。

第一阶段 CLI + 外部 Amp 的增量顺序见 C1～C4，构建原则为：

1. **命令与能力描述一致。** 在既有来源中补齐命令参数、结果、风险和已实现状态，让 help、capabilities 与实际验证规则可相互校验；不额外造另一份 Agent 专用能力表。版本化 JSON，错误路径同样能被机器读取。
2. **独立运行可诊断。** 补齐构建版本／安装说明及只检查不安装的 `doctor`；分清核心、Java／EPUBCheck 与可选 Amp。没有 Amp 登录不妨碍离线 EPUB 操作，缺 Java 不伪装正式检查通过；这些补齐项仍未实现。
3. **先交付一条可复现的 Amp 操作流程。** 复用 README 已有单字段闭环：`capabilities → inspect → workspace open → plan → apply → task diff`，展示真实 task ID 和差异后等待明确接受／拒绝／导出授权。JSON 请求文件放在工作区之外，正文不拼进 shell 参数；不循环盲重试写入，不读写内部 state 代替命令。可信使用说明以后可封装成 skill，但本次不创建 skill/MCP/插件，也不从书内加载指令。
4. **用反例验收自动化。** 不起 GUI；覆盖含空格／中文／短横线路径、缺检查器、无模型配置、过期计划、未知参数、并发 busy 和断流。真实 Amp 首测只用合成 EPUB，核对原书 hash、候选差异、接受决定及导出字节，不把模型回答“完成”当成通过。
5. **补正文操作，再做 UI，最后集成 Amp。** C2/C3 扩展读取与确定性文本修改；第二阶段 U 批次交付复用 Go 用例的 MyGo UI，第三阶段 A1 才在应用内管理 Amp。外部 Amp 在 GUI 打开期间调用 CLI 仍须服从 §5.3 的锁与状态刷新规则，不能借用当前窗口上下文绕过锁。

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

不承诺固定工期。当前顺序以 §11.3 为准；下表保留 M0～M6 技术工作包与目标门槛，编号不代表必须先完成所有 M0 才能交付 CLI，也不表示 M1/M2 的全部设想已经完成。MyGo/WKWebView 的 Apple Silicon release 隔离仍是桌面集成门槛，不能被 Linux WebKitGTK 或交叉编译替代。

| 工作包 | 目标交付 | 对应能力必须通过 |
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

第一轮四条工作线由独立 Ultra orb 承担，已经完成；这不是第二轮的模型要求。保留此前由 Medium 完成的 M1-A 代码和验证证据，不重复重写；集成线程维护计划、共同基线和验收结果。四个 orb 各自拥有明确模块，不是把整个应用复制四份同时开发。

| 工作线 | 本轮交付 | 独占修改范围 | 暂不进入本轮 |
|---|---|---|---|
| P1：M1-B1 只读结构 | EPUB3 nav / EPUB2 NCX、toc、只读引用图及逐语法 coverage | `internal/publication/`、`internal/references/`、必要的 `internal/bookpath/` 和 `internal/testfixture/`；`internal/app/`、`cmd/kepub/` 的对应入口；`docs/verification/M1_B1.md` | pack、EPUBCheck 适配、编辑和 GUI |
| P2：M2-A 工作区基础 | 原书保留、初始 revision、独立候选与检查点、内容树哈希、单写者锁和失败清理 | `internal/workspace/` 及其内部平台文件/测试；`docs/verification/M2_A.md` | 对外 workspace CLI、socket 服务、plan/apply、accept/export |
| P3：M0-A CLI 原型 | §8.1.1 的 Go 直连实验与故障证据 | `experiments/amp-cli/`、`docs/verification/AMP_CLI_SPIKE.md` | 生产 Agent 注册和真实书籍任务 |
| P4：M0-B SDK 原型 | §8.1.1 的 Node/SDK 辅助进程、监督及故障证据 | `experiments/amp-sdk/`（含独立 package/锁文件）、`docs/verification/AMP_SDK_SPIKE.md` | 生产 Node 依赖与 GUI 接入 |

P2 只依赖本轮冻结的 M1-A 读 API，不等待 P1 的新接口；P3/P4 使用相同文字契约和一次性候选 fixture，不等待工作区实现。新增代码可在独占目录内组织，不为并行人为抽出共享大框架。跨所有权目录、根 go.mod/go.sum、README、主方案、CLI 契约和 orb setup 的变更先由集成线程协调，避免分别改出不一致契约。

各 orb 从包含 M1-A 与本版方案的同一 Git 基线开始。远端尚未包含的提交通过文件传输工具传 Git bundle，再建立各自分支；线程消息中的 SHA 不等于代码已同步。每条线在自己的 checkout 中实现、修复和验证后回报路径、测试与限制；集成线程检查差异并运行组合测试。未通过验收的 prototype 不提升 capabilities，未推送的工作不声称远端已交付。

第一轮已发布的核心成果：P1 的 nav/NCX、toc、引用图和逐语法 coverage；P2 的独立候选、内容树哈希、检查点/恢复与协作单写者锁。父 orb 已执行组合 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 及真实 CLI smoke。当时 workspace 仍是内部库；第二轮才在本地开放命令和审核／导出。P3/P4 保留为独立实验，不提升生产能力；协议和进程证据见各自报告。

### 11.2 第二轮：Medium 并行实现，随后集成编辑闭环

第二轮从同一已发布的[第一轮基线](https://github.com/LeviTK/Kepub/commit/77821e6b163f45f81134394873d35f3ede8774ad)开始。阶段终点是：用户明确修改一个现有标题或作者字段，经独立候选、真实 diff、检查、显式接受与正式导出，原书和未修改资源的内容字节保持不变。计划、应用、接受和导出仍是四个动作，不能合并为自动保存。

| 工作线 | 第一批交付 | 独占修改范围 |
|---|---|---|
| Q1：M1-B2 | 安全目录快照、清单式打包、分层校验、固定版本 EPUBCheck 适配、`pack`/`validate` CLI | `internal/archive/`、必要的 `internal/publication/` 读取适配、`internal/validation/`、`internal/app/`、`cmd/kepub/`、`docs/verification/M1_B2.md` |
| Q2：M2-B | 局部 `metadata.set`、严格计划绑定与重算、独立候选、操作前检查点、失败回滚、真实树 diff | `internal/workspace/`、`internal/metadata/`、必要的 `internal/editing/`、`docs/verification/M2_B.md` |
| Q3：验证环境 | 将 Java 与固定 EPUBCheck 开发依赖加入幂等 setup；resume 仅快速验证，缺工具不安装 | `.agents/setup`、`.agents/resume`、`docs/verification/ORB_EPUBCHECK.md` |

先完成两条线可独立验证的库和命令，再将精确代码基线交给 Medium 接入工作区 CLI、检查、审核接受和导出。第一批 Q2 到 `review_required` 为止，不通过调用者提供的 `passed:true` 伪造接受门槛。集成阶段才开放相应 capabilities；共享 app/CLI 同一时间只有一个所有者。父线程负责方案、范围、差异复核和组合验收，不把前一批推送授权沿用为本批发布授权。

本轮库、validate/pack、环境脚本和公共编辑闭环均已本地集成，尚未发布。Q1 收尾后将 app/CLI 所有权移交给原 Q2 Medium orb，由其完成 accepted revision、公开命令与审核导出。父另用独立 EPUB3 样本执行 27 次真实 CLI 调用，通过连续接受、旧计划拒绝、候选隔离、缺依赖、外部改动后审阅／拒绝、no-op、FIFO 与输出边界检查；导出符合独立预期字节，原书未变。详细执行证据见 README 与各工作线验证记录。

集成首版用显式目录定位工作区，不加入全局注册表：`workspace open BOOK --output DIR` 创建新工作区，后续 `plan/apply/task` 使用 `--workspace DIR`，`task diff/accept/reject` 另需明确 task ID。报告与 EPUB 输出必须在工作区根之外且目标不存在；`workspace list`、历史任务浏览仍未纳入。首次导入的 initial 可作为基线，但不带合规通过状态；后续计划以当前 accepted revision 为基线，接受前真实检查，导出时重新检查最终归档。本轮不提供既有错误豁免或草稿接受，草稿仅是用户明确请求的导出选项。

第二轮限定：

- 每个计划恰好一个 `metadata.set` v1，仅支持现有 `dc:title` / `dc:creator` 的简单文本。选择器为 namespace、local name、可选明确 ID，另给预期旧值；必须唯一命中。复杂子内容、标识符、语言、版本升级、批量操作和资源改名不进入本轮。局部替换保留目标外原字节；no-op 不格式化 OPF，也不自动更新时间。
- 计划绑定 workspace、baseRevision、rootfile、精确出版树、操作实现和策略摘要。apply 不信任计划中的可执行标志或写集合，重新计算；候选里新增、删除和其他文件改动都必须进入实际 diff。失败不能更新已接受版本。
- `pack` 只读取明确的出版根与批准清单，不递归归档整个 workspace；新输出默认不覆盖。正式产物必须在同目录私有临时归档上完成 EPUBCheck，再原子发布。缺依赖、坏报告和超时不可冒充通过；草稿只在显式 `--draft` 时生成，绝不自动降级。
- 引用提取 coverage 与 EPUBCheck conformance 分开报告。部分 CSS 语法覆盖不等于允许 rename，也不能把缺失检查藏在 error 数量为零之后。正式导出的验证报告必须绑定最终归档，不能重用旧候选或源书报告。
- 真实 EPUBCheck 在本轮使用生成的 EPUB2/3 fixtures 验证；真实 Amp、用户书籍上传、GUI、资源改名、Calibre 和 Mac 发布不进入本轮。预览开发仍受 M0 Apple Silicon release 隔离门槛约束，Darwin 交叉编译不是实机验收。

验收需同时证明成功路径和拒绝路径：唯一文本局部修改、no-op、过期/被篡改计划、写集合越界、失败回滚、重开恢复、缺 Java/EPUBCheck、检查失败无正式产物、既有输出保护，以及正式导出后未修改资源的原字节比较。局部测试通过后，父线程重新跑组合测试与真实 CLI 流程；未完成的门槛继续明确记录为未完成。

### 11.3 v0.7 开发批次与依赖

截至本版：第一轮代码已在默认分支，第二轮闭环与环境更新已本地验证但未发布；MyGo 边界实验已在 Linux 验证，不是 Mac GUI 验收。2026-10-05 用户授权开始 CLI 框架及后续开发，当前进入 C0/C1/C2；具体接口见 CLI 契约 §2.1。其余批次按依赖推进，不重做已完成轮次，不把开始开发视为真实模型调用或发布授权。

三阶段产品终点分别为：**C：外部 Amp 可经 CLI 提交受限 EPUB 正文修改；U：用户不登录 Amp 也能阅读、修改、审阅和导出；A：用户可在 UI 中发起并控制 Amp 任务。** 发布各自有安装验收，不必等所有阶段完成才交付 CLI。

| 批次 | 依赖 | 交付与通过条件 | 不进入本批 |
|---|---|---|---|
| C0：共同开发基线 | 现有本地集成 | 核对本地／远端和未提交改动，记录准确基线、验证状态及文件所有权；进入开发时制作可传输的本地提交／bundle，新 orb 核对后再工作 | 自动推送、重新实现第二轮 |
| C1：CLI 可发现与可诊断 | C0 | 在现有 app/CLI 中统一 help、capabilities、schema 的命令与参数来源；补版本信息、只检查不安装的 doctor、构建／安装说明。成功与失败均有契约测试；无 GUI、Amp 或 Node 可运行核心，缺检查器明确报告 | 改解析框架、启动模型、自动安装依赖 |
| C2：内容读取与精确定位 | C0；CLI 接入接在 C1 后 | 读取显式 workspace 的 accepted revision 中指定 XHTML，提供有界文本查询与唯一定位信息、BookPath、revision 和资源 SHA-256；可增加 CSS 只读。无命中、多命中、超限和不支持语法有明确结果，不默认选择首个匹配 | 全书无界输出、活动候选读取、CSS 写入、把结构位置当可写偏移 |
| C3：受限正文编辑 | C1/C2 契约冻结并验收 | 增加一项版本化简单文本操作，复用 plan/apply/diff/accept/export；扩展执行来源记录与恢复检查，不是旁路写文件。元数据回归和正文完整闭环通过，只有预期文本字节发生变化；边界见 §11.4 | 任意 XHTML patch、混合内容重排、结构／样式修改、批量操作 |
| C4：外部 Amp 协作 | C1 后先做元数据试点；C3 后完成正文验收 | 提供可信使用说明／示例请求；显式授权后由真实 Amp 在合成 EPUB 上发现能力、读取目标、计划、应用、展示 diff，并等待用户决定。独立核对 task ID、原书 hash、检查结果和导出字节 | Kepub 启动 Amp、自动接受、读取用户书籍、SDK／MCP 前置 |
| U1：Mac 预览安全门槛 | CLI／应用服务契约稳定；可用 Apple Silicon 环境 | 用 MyGo 0.2.0 + WKWebView 验证隔离、导航／网络、bridge、readiness、旧 generation 与崩溃恢复；明确隔离容器和前端选型。Linux 实验和 Darwin 编译不能代替 release 实测 | 假定默认第二窗口无桥、以模型联调阻塞预览验证 |
| U2：无模型桌面闭环 | C3、U1 | 资源／目录、定位阅读、受限文本修改、候选预览、diff、检查、accept/reject/export 调用同一 Go 用例；不要求 Amp 登录。稳定 revision/generation 与锁冲突可见，CLI/UI 结果一致；代表状态实际渲染并验收 | 完整代码编辑器、Agent 聊天、长期会话服务的无验证接入 |
| A1：UI 内受管 Amp | C4、U2；先冻结锁交接／审批契约 | 优先 Go→Amp CLI，实现任务与线程绑定、显式选区上下文、流式进度、取消回收、冻结、恢复和可视审核；先在受控协议测试通过，再授权真实继续／取消联调。完成后仍由用户决定接受／导出 | 默认 SDK 双后端、External API 执行 prompt、任意同用户写者的沙箱承诺 |
| R：分阶段发布门槛 | 对应 C／U／A 批次通过 | CLI 先验收独立安装与检查器说明；桌面另验收依赖策略、签名公证、干净 Mac 安装／升级／卸载。最终用户不需 Go 或前端编译工具；发布另获授权 | 强迫 CLI 等待桌面／Amp、Calibre／跨平台／书库作为前置 |

C1 与 C2 的读核心可并行；C3 等待两者接口稳定。C4 的元数据试点可及早发现 CLI 易用性问题，但不能替代正文验收。U1 可在 C 阶段后半提前排除平台风险；U2 不依赖 A1，也不因暂缺真实模型授权停工。只有对应门槛通过才提升 capabilities；不要把文档版本当软件发行版本。

### 11.4 首个正文读写版本的边界

当前 `internal/workspace/edit.go` 的参数类型、计划重算和执行记录限定为一个 `metadata.set` v1；`lifecycle.go` 的接受／恢复也复核这些来源。正文写入必须一起扩展这些契约。`internal/publication` 的 `Element.Location` 是结构位置，不是可写字节偏移；不能把现有引用查询结果直接当安全编辑 API。

- **读目标：** C2 首版只读显式 workspace 的 accepted revision；从同一稳定输入计算路径、文本、定位信息和资源 hash。查询匹配多个位置时返回有界列表与歧义，不返回“默认第一个”；截断必须明确，不能把残缺文本冒充完整内容。单次大小／结果数上限、文本解码和匹配语义须在编码前写入 CLI 契约。
- **写目标：** 一个计划仍只含一个操作。正文操作暂定只改一个现有 XHTML 简单文本元素：明确 BookPath、读取时的 revision／资源 hash、唯一定位依据、预期旧值和新文本。定位依据由读取结果提供并绑定输入；不接受调用者指定原始字节区间作为写权限。命令名称、操作 ID 和字段 schema 在 C2/C3 启动时冻结，本文不提供假装已可执行的命令。
- **修改算法：** plan/apply 在持锁且验证基线后重新解析并唯一定位，再计算局部替换范围；新文本按 XML 转义并重解析检查。保留元素、属性、ID、href、目标外字节及其他文件；no-op 不重新格式化。现有 `metadata.set` v1 的含义和元数据流程保持不变。
- **明确拒绝：** 旧值／hash／revision 不匹配、定位歧义、非法 XML 字符、不支持编码或语法、子元素／混合内容、跨节点范围、脚本／样式目标和结构变更。新值是文本，不是可执行 HTML；包含 `<` 等字符应安全转义，不能解释成标签。全 CSS 写入、资源新增／删除／改名和任意文件补丁另立批次。
- **执行与持久化：** 操作分派、计划摘要、真实写集合、执行来源、diff 和 accept/reopen 的重算必须一起版本化。旧工作区、旧计划与旧执行记录要么按旧版本正确读取，要么在修改前明确拒绝；不得静默用新算法解释旧摘要。先做保持元数据行为的必要重构并回归，再加入正文操作，不引入动态插件系统。
- **验收终点：** 外部 Amp 只能生成该操作的参数，经 CLI 得到候选；候选仍需真实 diff、正式检查和明确接受，正式导出复查最终 ZIP。未注册操作不能通过直接写候选再伪造原执行记录获得接受。

### 11.5 Medium 分工与集成顺序

后续代码及其修复由 **Medium** 编写；父线程维护方案、冻结跨模块契约、复核差异和执行组合验收。用户已要求开始实施，先启动 C1 与 C2 读核心，不一次创建全部工作线程。

| 开发单元 | 文件所有权与交接 | 可独立验收的结果 |
|---|---|---|
| Medium CLI 线：C1 | `cmd/kepub/`、`internal/app/` 的发现／诊断用例及对应测试；不改 workspace 写入 | CLI 描述与行为一致、依赖诊断、版本输出、非交互错误测试 |
| Medium 内容线：C2 读核心 | 先限 `internal/publication/` 及其读取测试；workspace 快照读取由同线在明确移交后接入，不与 C3 同时改。app/CLI 等 C1 完成并移交后再接入 | 有界读取、精确定位、稳定输入 hash；随后是真实 CLI 查询 |
| Medium 编辑线：C3 | 接收 C1/C2 的精确集成基线后，负责 `internal/workspace/`、正文修改模块和必要 app/CLI 入口；共享文件单一写者 | 操作版本／来源校验、局部修改、恢复和完整审阅导出闭环 |
| 后续 C4／U／A | 按已完成批次逐个安排；U 的 desktop 模块与 headless 核心分开，A 的生产适配不得擅自改审批或锁契约 | 各批次的实测记录；仅交叉编译或 fake 测试不升级为真实平台／模型通过 |

README、主开发方案、CLI 契约和根依赖／setup 的跨线变动由父线程协调；工作线提交自己的验证记录，不各自修改总状态。确需新 package 时按责任划分，不为并行造多份解析器或重复注册表。

进入 C0 时先核对现有未提交内容并形成可复现基线，不覆盖先前工作。新 orb 默认只取得远端分支，须通过 bundle／文件传输接收本地未发布工作并核对；消息里的提交 ID 不能代替传输。每批回传差异、命令、失败路径和限制，父线程组合测试后才更新状态。推送、发布和真实模型调用各自取得授权，不继承历史轮次已使用的授权。

### 11.6 每批验收与需冻结的决策

| 门槛 | 必须提供的证据 |
|---|---|
| C1 契约一致 | help／capabilities／schema 与可执行命令的双向测试；未知参数／版本、中文／空格／短横线路径、缺 checker、无 Amp 配置有明确结果；doctor 不安装、不认证、不执行模型 |
| C2/C3 内容安全 | 用不对称样本验证精确路径、同文多处、旧值不符、hash 过期、无命中、大小边界、实体／中文／非 BMP、BOM／CRLF、混合内容拒绝；成功只改预期文本，no-op 和其余字节不变 |
| C3 持久闭环 | 元数据 v1 回归、新旧计划版本、篡改／额外文件、失败回滚、重开恢复、忙锁、旧 task ID、缺依赖、检查失败无正式输出、已有输出保护；由真实二进制完成至少一条正文接受与一条拒绝流程 |
| Go 组合验证 | 受影响单测／fuzz 后执行 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...`；配置真实固定 EPUBCheck 执行相关 fixture，不能把跳过外部检查当正式校验通过。仅修改文档时不重跑 Go 测试 |
| C4 真实协作 | 经授权仅用合成 EPUB；记录实际工具调用、JSON、task ID、用户决定与最终字节校验。不支持请求必须停止或提出替代方案，不改内部 state／不静默降级；模型口头“完成”不是证据 |
| U/A 平台与安全 | 实际渲染默认和错误／冲突／旧 generation 状态；Mac release 隔离、锁交接、真实 Amp 继续／取消、异常退出及冻结后无受管写入分别验证。fake 进程、Linux WebKitGTK、Darwin 编译分别标记，不能互相替代 |

尚需在对应批次开始前冻结的决策：C2/C3 的查询／匹配／定位 schema 和兼容策略；U1 的受限 frame 或专用 WKWebView、React + TypeScript + Vite 是否正式采用；U2 的逐用例锁与状态刷新方式；A1 的会话转发或受控锁交接、审批边界与生产 Amp 协议版本；R 的最低 macOS、Java／EPUBCheck 随包或外部安装策略、Amp 发现、许可及签名更新流程。Unix socket、SDK、MCP 均不是已决定必须新增的依赖。

首版外部 Amp 使用合作式审批，必须明确 `task accept` 无法鉴别人类与同用户 Agent。若要承诺强制人工批准，需另验模型无法取得的批准凭据及受限执行环境；不能通过提示词或隐藏命令名宣称实现。Mac 环境或模型授权暂缺时继续可独立验证的工作，阻塞门槛保留为未通过，不承诺固定完成日期。

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

既有 Amp/EPUB 来源入口保留于 [v0.1 来源记录](history/DEVELOPMENT_PLAN_V0_1.md#18-来源与核验记录)，v0.2 的 Calibre 设计原文保留于 [历史文档](history/DEVELOPMENT_PLAN_V0_2.md)。当前正式检查器已固定为 EPUBCheck 5.3.0，证据见 [M1-B2](verification/M1_B2.md)；规范参考 [EPUB 3.3](https://www.w3.org/TR/epub-33/)、[Reading Systems](https://www.w3.org/TR/epub-rs-33/) 和 [EPUBCheck CLI](https://www.w3.org/publishing/epubcheck/docs/cli/)。

v0.3 修订只记录了上游 MyGo 0.2.0 事实。v0.4 的实际证据见 [M1-A](verification/M1_A.md)、[M1-B1](verification/M1_B1.md)、[M2-A](verification/M2_A.md)、[CLI 实验](verification/AMP_CLI_SPIKE.md) 和 [SDK 实验](verification/AMP_SDK_SPIKE.md)。不得据此宣称目标 Mac 实机、MyGo 预览、真实 Amp 集成、Calibre/EPUBCheck 或全部性能与安全矩阵已通过。

v0.5 的本地编辑闭环证据另见 [M1-B2](verification/M1_B2.md)、[M2-B](verification/M2_B.md) 和 [检查器环境](verification/ORB_EPUBCHECK.md)；v0.6 纳入 [MyGo 边界实验](../experiments/mygo-boundary/README.md)与 CLI-first 暂定顺序。文档版本、代码已实现、本地已验证、默认分支已发布是四种不同状态，后续维护须分别更新。
