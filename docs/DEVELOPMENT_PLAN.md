# Kepub 开发方案

> 文档版本：0.10（开发计划；新接口暂定）· 更新日期：2026-10-06
>
> 当前路线：先补 Issue #3 的规范资产与差距矩阵 S0，再按 §11.7 完成独立 CLI 的 T1～T6，默认 EPUB3、显式 EPUB2 → EPUB3 转换，不新增嵌入字体。近期不集成外部编辑器、UI 或 Amp；真实预览、完整排版和无障碍验收保留在后续 S2～S4，不从最终目标删除。T6 完成不等于 Issue #3 关闭，阶段映射和关闭门槛见 §11.8。
>
> 状态：只读核心与 CLI、工作区库及 Amp 对照实验，以及 validate/pack、单字段元数据编辑、候选审阅、接受／拒绝与工作区导出、C1 命令框架、C2 正文查询、C3 单节点简单文本修改和独立审查修复，均已作为源码发布到默认分支 main。GUI、生产 Agent、真实 Amp 编辑联调、版本化安装包和 Mac 实机验收未完成。实际支持范围以 README、capabilities 和验证记录为准；文档版本不是软件发行版本。
>
> 平台顺序：Linux CLI 先功能测试，随后打包并实机验收 Apple Silicon Mac CLI。MyGo 与系统 WebView 选型保留给后续 UI；当前接口基线为 MyGo 0.2.0，macOS 使用 WKWebView，不捆绑 Chromium。前端与预览不作为 CLI 完成条件。

## 0. 本次修订与阅读顺序

v0.10 根据 [Issue #4：EPUB CLI 生态借鉴、安全修复与分层校验](https://github.com/LeviTK/Kepub/issues/4) 的完整正文（2026-10-05 核对，无评论）补充原生 FixProposal、ValidationDelta、上游角色与可选适配边界。沿用 v0.9 的 S0 → T1～T6 和 T4b，不另起路线，不要求安装全部研究工具。Issue #4 的上游结论来自静态研究，本次没有独立运行或审计这些工具，未锁定版本仍待实施前核验；规范资产、功能实现和开发暂停状态均未改变。

用户随后授权先由真实 Droid 审核计划，无阻塞问题后按 S0 → T1～T6 开发并循环 review。前三轮全文计划审核依次提出 9 项、6 项及 1 项问题，修订了 S0 退出清单、XML 语义与预算、字体混淆、EPUB2 资产、checker 覆盖和 CSS 阶段分工。R4 定向复核关闭索引问题并要求补明测试措辞，R5 已确认该补句关闭剩余问题；当前满足开始 S0 的计划门槛，不表示实现或规范归档已完成。

各轮真实模型、输入 hash、发现、失败、completion 和未测范围见 [v0.10 计划审核](verification/V010_PLAN_REVIEW.md)。R4／R5 是增量确认，不冒称全量复审；实现仍须逐批通过测试、真实 CLI／EPUBCheck、Droid review 和父独立验收。此前“暂停”记述是历史状态，S0 通过前仍不恢复 T1 产品实现。

**当前审计者变更（用户 2026-10-06 指令）：使用 Amp 的 DeepSeek V4.1 Flash（`deepseek-v4.1-flash`），Droid 待用量恢复后使用。** 本规则适用于下文开发审查流程；保留以往 Droid 要求、实际结果和失败，不伪造 Droid 证据。当前独立审计与父验收、完整范围、固定输入、实测和修复复审门槛均不变。不得因额度不足直接放行，也不自动充值或监控额度。

S0 已本地建立原文归档、官方用例来源／工件索引、上游版本与许可记录，以及离线矩阵生成和校验脚本。P1～P7 已独立复验；首轮固定树 Droid 实现审查结束时未批准，发现门禁／证据核验、列表条款、官方用例关联及 NVDL 归档遗漏，父另复现算法说明误标的 P8。各项修复、固定新树审查、阅读限制及实际放行结论持续记录在 [S0 父验收](verification/S0_PARENT_ACCEPTANCE.md)，不改写原失败；实际 checker 增量见 [S0 EPUBCheck 核验](verification/S0_EPUBCHECK_2026.md)。离线 `gate` 核验资产、派生物、来源，以及绑定同一输入的父验收和真实独立审计完整批准，不以布尔字段、普通样本通过或审查进程 exit 0 放行。归档／生成不算官方用例执行，不提升 CLI 功能状态；实施验收与推送／发布分别记录。

v0.9 对齐 [Issue #3：EPUB 3.3 全规范兼容与 XHTML／CSS 全面排版](https://github.com/LeviTK/Kepub/issues/3) 的完整正文（2026-10-05 核对，当前无评论）。该 issue 是跨 CLI、核心、预览和验收的总目标；本版补齐规范归档、五维能力证据、阶段映射和关闭条件，不把它误缩为六批终端功能，也不把后续 GUI 强行放进当前 CLI 批次。编码 Orb 报告的 T1 初稿尚未编译／测试／集成，仍待 S0 验收后恢复；旧基线普通／race／vet 通过不是新功能证据。

v0.8 按用户最新范围将近期目标调整为独立终端制书：以官方 EPUB3 为默认新建和编辑目标，优先完成其 XHTML 语法处理、内置编辑操作、书籍 CSS 和本机系统字体发现；新增 EPUB2 → EPUB3 转换，不并行建设完整 EPUB2 编辑语法。已有 EPUB2 读取、校验与受限编辑保持，不因计划修订移除。不向 EPUB 新增字体本体，不集成外部编辑器。基础图片插入与简单排列纳入，复杂图文排版、固定版式等进阶制作能力后置。现有工作区和审核机制继续复用，不把计划修订当作代码已实现。

优先阅读 §0.1 的范围对应、§3.4～§3.9 的内容边界（含按标题拆章）、§7.2～§7.5 的提案与验证、§8.3 的生态取舍，以及 §11.7～§11.9 的实施顺序。§11.1～§11.6 保留既有交付和 v0.7 路线，不改写历史证据；其中 C4／U／A 依赖不约束新 CLI 主线。除 CLI 契约已单列的 T1 语义外，新命令/schema 尚待冻结；计划不代替实际实现、验证或发布授权。

v0.7 已完成 C1/C2/C3 受限读写；其“先外部 Amp 协作，再 UI”的推进顺序由本版取代。当前二进制仍限制为单操作、简单文本和 UTF-8，并拒绝 DOCTYPE。用户上传的 EPUB3 已实际通过 EPUBCheck，但因 38 处简单 `<!DOCTYPE html>` 被 Kepub 阻断；去除声明后的副本才跑通编辑闭环。这是标准兼容缺口，不是私有格式，也不能作为原书直接通过的证据。修复后的首要验收必须直接使用未经预处理的原书；私有书籍不提交到公共测试仓库。

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

当前确认：独立 Go CLI 优先、默认官方 EPUB3、EPUB2 显式转换到 EPUB3、EPUBCheck 主校验、原书保护、候选审核与无字体嵌入的系统字体选择。MyGo 和 Amp 保留在后续适配层，不渗入出版物核心；SDK、浏览器与外部编辑器不是本阶段依赖。后续预览仍须完成独立隔离验收。

### 0.1 Issue #3 的最终目标与近期范围

“全面支持”分别证明保留、解析、编辑、渲染、验证，不能压成一个 `valid:true`。EPUBCheck pass、引用 complete、CSS 属性探测或使用 WKWebView，都不等于完整支持。目标是固定 EPUB 3.3 及其适用 XHTML/CSS 基线，不是所有未来 CSS 草案或跨阅读器像素一致。

| 决策来源 | 本计划执行方式 |
|---|---|
| 用户已确定的近期产品范围 | 默认 EPUB3；EPUB2 显式转换；不新增嵌入字体；内置编辑；基础图片；Linux 先测试、Mac 后打包；UI／外部编辑器／Amp 后置 |
| Issue #3 的总目标 | S0 规范归档与条款映射、S1 内容与编辑、S2 作者原样预览、S3 完整排版／媒体交互、S4 官方测试与 Mac／无障碍验收，全部保留 |
| 字体读取与字体写入的区别 | 不新增字体本体，不妨碍保留和验证原书 `@font-face`、TTF／OTF／WOFF／WOFF2；规范字体混淆的读取与预览另有明确后续任务，不等于绕过 DRM |
| 开发验证流程由用户指定 | 父 Orb 主导，Medium 子 Orb 编码，真实 Droid demo／代码审查与修复复测；这不是 Issue #3 原文指定的工具，也不能代替官方／实机测试 |

关联 [Issue #1](https://github.com/LeviTK/Kepub/issues/1) 的 Source／Semantic Diff 与样式影响说明：CLI 先提供真实数据，后续审核面板复用，不重复另造 Pierre Diff UI。关联 [Issue #2](https://github.com/LeviTK/Kepub/issues/2) 的 Revision／Tree／Blob：沿同一版本事实来源演进，预览与结构编辑不新建平行版本库；不能把关联 issue 的设计当作已经落地。

Issue #4 补充上述路线的实现来源、修复提案、诊断对照和外部工具接入条件，不替代 #3 的总体符合性目标。只复用成熟方法、可核验接口与许可明确的样本，不把全部上游变成产品依赖，也不把其自述的兼容性或性能作为 Kepub 验收证据。

## 1. 产品定位与范围

Kepub 当前目标是独立终端 EPUB 创建、修改与维护工具，后续再扩展阅读／制作 UI 和 Agent。完整制书闭环不等于完整阅读系统，不扩展为 Calibre 式书库。

三方职责固定为：**CLI 是可脚本调用的操作入口，UI 是阅读与审核入口，Amp 是理解需求和提出编辑的智能执行者。** 文件、计划、候选、检查和导出规则由共享 Go 核心负责；只有新增并通过验收的操作才能被任一入口使用。

首阶段用户通过终端命令与声明式操作输入完成制作；不接入 Amp、不启动外部编辑器，也不通过直接覆盖内部候选文件绕过审核。后续 UI 直接调用 Go 用例，不反向解析 CLI 文本。当前目标流程如下，其中新增能力仍待实现。

```diagram
新建／导入 EPUB → 工作区 → 查询／定位
                            ↓
              内置内容／结构／样式操作
                            ↓
                  计划 → 候选 → 差异
                            ↓
                正式检查 → 接受／拒绝
                            ↓
                   最终 ZIP 检查 → 导出
```

CLI 不登录 Amp 也能查询、检查和执行已实现的确定性操作；后续阅读 UI 同样不应要求模型登录。不安装 Calibre 也能使用核心功能。正式检查仍需要 Java／EPUBCheck；模型是可选执行者，不是 EPUB 文件操作的必需依赖。

### 1.1 独立 CLI 的当前交付目标

- 整理并交付现有查询、安全解包、validate/pack、workspace、plan/apply、diff、accept/reject 和 export，不从头重写 CLI。
- 元数据操作可修改一个现有 `dc:title` 或 `dc:creator` 简单文本；C2 已本地实现绑定 accepted revision 的单资源 XHTML 内容读取，C3 已增加 `content.text.set` v1 的单节点简单正文修改。每计划一个操作，不代表支持任意 XHTML 编辑。
- C1 已本地补齐命令帮助／schema 同源、构建版本、只检查不安装的 `doctor`、源码构建／安装说明及测试；保留 JSON envelope、错误码与非交互行为。发行安装包和 Mac 安装验收仍未完成。
- 待实现：默认新建 EPUB3、EPUB2 → EPUB3 转换、完整常用元数据、章节增删和阅读顺序、目录生成与维护、资源与引用更新、全书查询／替换、多操作事务、任务／版本历史及恢复；通过这些能力形成“创建—修改—维护”闭环。新建 EPUB2 和 EPUB3 → EPUB2 降级不纳入本阶段。
- 网络小说按章节标题自动拆章纳入 T4b：从 XHTML 标题或明确规则识别候选边界，审阅后生成独立章节并同步目录／引用；不是仅生成目录，也不扩展为 TXT 导入、爬取或模型猜测章节，具体边界见 §3.9。
- 完整 EPUB3 XHTML 语法处理与内置混合内容／结构编辑进入主线，不再以“只有简单文本节点”作为最终目标。EPUB2 补足安全读取和迁移所需语法，不另做同等完整的编辑器；需要新编辑能力时先显式转换。CSS 管理和系统字体发现是正式能力，不只是后续 UI 设置。
- 新书和新增样式只声明字体族与回退，不复制字体文件；已有书籍字体资源及引用保留。基础图片增删替换、替代文本、尺寸、居中／简单并列与图注纳入；复杂图文绕排、固定版式与精确分页后置。
- 不集成外部编辑器、资源编辑往返或任意脚本入口。用户提供的文本／XHTML 片段／CSS／图片作为内置操作的明确输入，冻结并绑定 hash；不是对 workspace 内部文件的写权限。
- Linux 验证可先进行；面向 Apple Silicon 的 CLI 正式发布仍须实机安装和运行验证。Darwin 交叉编译不算 Mac 验收，CLI 检查也不替代视觉排版审核。

### 1.2 后续桌面目标

- 仅提供 `darwin/arm64`。最低 macOS 暂以 15 为建议值，M0 根据 release 实测冻结；不同 macOS 的 WebKit 差异仍需测试。
- 未加密、可重排 XHTML 为主的 EPUB 2/3；解析 container、OPF、manifest、spine、EPUB 3 nav / EPUB 2 NCX。
- 原书样式的章节级滚动预览、目录、内部链接、前后章节、返回历史、阅读位置。
- 已有 XHTML/CSS 的 Agent 编辑，以及通过独立验收的确定性操作。
- 任务级差异、整任务接受/拒绝、冻结输入上的检查及独立 EPUB 导出。
- 无界面 CLI 不初始化 MyGo、AppKit 或 WKWebView。

固定版式、Media Overlays、书籍脚本、复杂音视频、签名、DRM、未支持字体混淆必须被识别。无法保证安全保真的输入进入明确的受限只读状态或拒绝编辑，不能删掉不支持内容后声称成功。

这是当前限制而非永久排除：Issue #3 的 S2／S3 仍须实现并实测相应字体、SVG／MathML、固定／混合版式、媒体与安全交互 profile；MUST 缺口不能靠标注“后期”视为关闭。

### 1.3 不做的事

MVP 不做书库数据库、OPDS/远程书库、多设备同步、邮件发送、设备管理、格式大全、完整分页、完整代码编辑器、内嵌终端、任意插件代码执行或完整阅读系统一致性认证。

本阶段不做 UI、Amp 接入、外部编辑器集成、字体下载／安装／嵌入／子集化、厂商私有协议或非 EPUB 格式转换。章节合并、任意嵌套位置拆分、按字数强制切分、复杂图片排布、CSS 自动清理、图片压缩、媒体叠加、音视频与脚本交互制作列入后期；按标题的安全边界自动拆章已纳入 T4b。其中不少后置项是官方 EPUB 能力，只是制作操作后置，不能把它们统称非标准。

项目名 Kepub 不等于 Kobo KEPUB 格式。默认输出普通 `.epub`；不能因为项目同名而自动注入 Kobo 标记、改变扩展名或执行 kepubify。Kobo 输入的识别、保留策略和可选转换另列能力，不把扩展名当充分证明。[研究 §2、§7]

## 2. 技术边界与核心架构

### 2.1 选型

| 层 | 首选 | 约束 |
|---|---|---|
| EPUB 核心 | Go 1.27.1 独立 package | 当前依赖标准库、x/text v0.29.0、x/sys v0.36.0；不依赖 MyGo、Amp、Calibre 或 Java |
| 应用服务 | Go | 命令调度、任务、操作、锁、审核与导出 |
| 系统字体发现 | 计划评估 Linux Fontconfig／macOS Core Text | 仅平台适配读取字体清单和属性，不把字体文件、系统路径或 GUI 依赖带入出版物核心；后端尚未实现或冻结 |
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

当前仅支持 UTF-8 XML 及内建／数字实体，UTF-16、DOCTYPE／DTD 和自定义实体明确拒绝；v0.8 将标准语法兼容列为首批，目标与安全边界见 §3.4。禁止联网获取外部实体与无界展开。严格出版解析与可选容错诊断分离；不能用 HTML 容错解析后自动写回来掩盖 XML 错误。

### 3.3 引用图

记录每条边的源资源、源属性/语法位置、原始 href、解析目标、fragment、引用类别和解析器版本。首批覆盖 OPF manifest/spine、nav/NCX、XHTML `href/src`、CSS `url()/@import`、内联样式及已支持的 SVG 引用；`srcset`、SMIL、其他词汇不能未经实现就声称完整。

T2 放行标准混淆书时，`META-INF/encryption.xml` 的 `CipherReference` 必须进入依赖核验，按 OCF 定义的 URI 基准寻址，不按该文件所在目录猜路径。T4a 改名被混淆字体须原子同步该引用，否则拒绝；本阶段拒绝删除被混淆字体，保持既有字体保留范围，不新增混淆资源清理功能。T2 尚无此操作时直接拒绝路径／声明变化，不以字体字节未变判安全。`signatures.xml` 引用同属不能忽略的来源，但本阶段签名书整书只读，不尝试改引用、删除签名或伪造有效签名。

每类语法单独报告 coverage。未知脚本、未知 CSS 语法、解析失败或未支持引用只能标成未知；“没有找到引用”不是“已证明没有引用”。删除资源与移除 CSS 规则必须采用保守策略。

`resource.rename` 必须同时处理：文件移动、移动文件内部相对链接重定基准、其他文件的入站链接、OPF/nav/NCX 相关引用、保留 ID 与 fragment。只调用低层 rename 不足以完成此操作。源/目标重叠、循环改名、大小写改名和目标碰撞先做 preflight。[研究 §3.2]

第一版拒绝 OPF 文件本身改名、跨 rootfile 结构重写及引用覆盖不足的改名。安全子集通过测试后逐项扩展，不靠“尽量修复”隐藏遗漏。

### 3.4 标准 XHTML：语法、编辑与渲染分开验收

“完整 XHTML 支持”本阶段以 EPUB3 的 HTML XML 内容文档要求为准，不把任意 HTML 标签拼接或浏览器容错解析称为 XHTML 合规。默认制作目标对齐 EPUB 3.3，OPF `package@version` 仍为 `3.0`，不是 `3.3`；规则与固定检查器的覆盖范围如实记录。EPUB2 的 OPS／XHTML 只扩展安全迁移所需解析，不并行新增完整 EPUB2 编辑能力，不为兼容增加厂商私有标记。

| 层次 | 本阶段目标 | 不能据此声称 |
|---|---|---|
| 解析与保留 | UTF-8／UTF-16、命名空间、合法 XML 声明和 DOCTYPE、目标规范允许的实体、注释／CDATA／处理指令、空元素、混合内容；保留合法未知属性和未修改内容 | 任意 DTD、非法 XML 或超限输入均可接受 |
| 内置编辑 | EPUB3 混合文本、段落／标题、强调、链接／锚点、列表、表格、ruby 等结构；安全插入、替换、删除与属性修改 | 原 `content.text.set` v1 自动拥有结构写权限，或把任意文件覆盖当结构操作 |
| 规范与显示 | 版本化结构规则、引用完整性和真实 EPUBCheck；生成可重排文档 | CLI 实现阅读器、字体塑形／排版引擎，或保证所有阅读设备视觉一致 |

简单 `<!DOCTYPE html>` 不能继续被当作危险实体声明一概拒绝。EPUB3 按 [§3.9](https://www.w3.org/TR/2026/REC-epub-33-20260113/#sec-xml-constraints)／[附录 B](https://www.w3.org/TR/2026/REC-epub-33-20260113/#app-identifiers-allowed) 判断声明，**不读取外部 DTD 子集，包括本地 catalog 中的外部子集**。这是共享核心选定的处理方式，与 [Reading Systems §3.6](https://www.w3.org/TR/2024/REC-epub-rs-33-20241017/#sec-epub-rs-conf-xml) 的非验证处理一致；后者的 MUST 适用于阅读系统，不冒充 CLI 作者工具的独立符合性规则。T3 EPUB2 迁移输入才使用固定白名单离线 DTD／实体，输出进入 EPUB3 规则；任一路径都不访问任意外部 URL 或本机实体文件。

编码支持必须贯穿 publication 读取、定位、编辑、执行来源重算与重新打开，不只是前端转码。未改资源保持原字节；修改资源保留编码与声明，确需转换或重序列化时作为显式操作列入 diff。新结构操作按版本扩展请求和持久格式，旧元数据及简单文本操作语义不变。先修复原始上传 EPUB 的 DOCTYPE 兼容，用合成 EPUB3 补齐编码、语法和拒绝矩阵；EPUB2 保留既有回归，迁移所需语法在转换批次补齐。

T1a 只通过 §11.7 的首增量门槛；T1b 必须在统一资源预算内完成下表，**不能仅给必需项标 unsupported 就宣布 T1b 通过**。T2 在 T1a、T1b 都通过后开始。开发中的未覆盖输入暂报能力不足，但不改变必需集，也不把 EPUB3 合法语法全推给 T3。

| T1b 必需类别 | 处理与验收 |
|---|---|
| 必须解析／保留 | XML 1.0／命名空间、合法声明、UTF-8／UTF-16、无外部标识符的 DOCTYPE、注释／PI／CDATA；内部子集的合法 ELEMENT／ATTLIST／内部通用及参数实体／NOTATION 声明，按非验证处理器规则处理属性规范化、默认值和实体替换；不把“不做 DTD 有效性验证”误当可忽略默认属性 |
| 必须接受但不解析外部子集 | 附录 B 的精确 public／system 标识符与 manifest 媒体类型组合：NCX、SVG 1.1、三个 MathML 媒体类型；声明原字节保留，外部内容零读取，不能把白名单套到 XHTML／OPF。完整元组由 S0 从固定附录登记 |
| 必须诊断非法 | XML 非良构、非法字符／声明／命名空间、递归实体；EPUB3 内部子集的外部实体声明、XInclude，以及不符合附录 B 的外部标识符（包括 EPUB3 XHTML 的外部 DOCTYPE）；不得用 catalog 将非法声明“修成通过”，也不把 EPUB3 的版本约束套到 EPUB2 |
| 解析来源与写权限 | 内部实体的字符及标记替换、参数实体与 ATTLIST 默认／固定属性必须有展开语义及来源标记；保留原声明和引用。实体生成节点／默认属性没有独立原始可写字节时，读取仍正确，直接局部写入必须拒绝或走未来显式结构操作；不伪造 byte offset，不把虚拟节点映射为第一个相似源节点 |
| 已识别但无法展开的实体 | 按 XML 1.0 §4.1／§5.1 区分 WFC 与 VC：外部子集／参数实体引用且非 standalone=yes 时，未取得声明不一定是非良构。记录实体名、原字节区间及来源，保留引用，内部语义和依赖 coverage 为 partial，不用空串或猜测值代替；缺失参数实体后声明的处理遵守 §5.1，不能继续套用可能已被覆盖的默认值。所在元素及依赖它的操作不得直接局部写入；这项有意不展开的处理须实现，不是以笼统 unsupported 免除必需解析 |
| 预算与政策 | 每资源 XML 8 MiB 按原始字节计，含 BOM；每资源先解码的 UTF-8 流及实体展开后的 UTF-8 流分别不得超过 16 MiB，均计入标记、属性、文本与保留的声明，不把两个表示长度相加，也不对每个祖先重复计文本。实体替换每次产生的字节另累计到同一资源的 16 MiB 展开工作预算，防止最终输出小但中间工作无界；不跨资源累计该限额。DTD 声明最多 4096、实体嵌套最多 16、替换次数最多 100,000；沿用元素深度 128、tokens 200,000、索引 32 MiB，实体生成内容也计数。循环与非法语法是输入错误，超限是预算错误；固定上限内的必需解析不能以泛称“DTD 不支持”拒绝 |

**32 MiB 索引是独立上限，保留既有计量，不承诺 16 MiB 正文均可索引。** 字符数据按 UTF-8 字节先计两次，每向上一层祖先汇入子树文本再计一次，另加各元素 location 的 UTF-8 字节；这是索引预算口径，不是实际驻留内存测量。单段 `html > body > p` 文本约计四倍，计入 location 后有效文本上限略低于 8 MiB；更深嵌套更早触限。原始／解码／展开流预算中的“不按祖先重复计文本”不改变这个索引计量，所有独立限额须同时满足，不借支持转码放宽旧索引预算。

读取、局部编辑后的目标资源、来源重算和历史重开采用同一计量；不因 UTF-16 转为 UTF-8 膨胀就重新套用原始 8 MiB 限额。T1a 先落实 raw／decoded／索引计量，T1b 增加实体预算与反例；UTF-16 LE／BE 的 ASCII／CJK／代理对及原始 8 MiB 两侧均独立测试。转码后跨 8 MiB 的成功样本明确使用大段 CJK 注释和小段可编辑正文，使索引仍在预算内；LE／BE 各一份，均须验证读取／定位／局部编辑／来源重算／历史重开一致，不能仅验证解析。另用正文为主的 CJK 及不同嵌套深度，验证 raw／decoded 未超限但索引超限时仍拒绝，不能用注释样本宣称大正文可读。虚拟来源的写入限制不意味着解析或保留可以缺失；该区分是五维能力的组成部分。

无法展开实体使用专用 `XML_ENTITY_UNRESOLVED` 能力诊断，位置不丢；真正违反 Entity Declared WFC（如 standalone=yes）的输入仍报非良构，不与未读取外部声明混淆。可表达 coverage 的读取／inspect 返回 partial；现有 `content/search` 要求完整文本与匹配数，遇到应读取的未展开内容明确 exit 3，不返回成功的残缺文本／匹配数，不跳过资源。T1b 对附录 B MathML 外部实体、内部参数实体及 standalone=yes 反例做无外部读取、保留、诊断和写入拒绝测试。checker 若使用其自有 DTD 而产生不同文本，记录处理 profile 差异，不为匹配它而修改 Kepub 的零外部读取策略。

以上附录 B 约束只用于已确定为 EPUB3（package version 3.0）的上下文；解析器须携带资源类型和版本 profile，不能从 DOCTYPE 猜书籍版本。EPUB2 非迁移读取路径对其合法外部 DOCTYPE／尚未支持的实体报能力不足（exit 3），不称 EPUB3 意义的非法；实际 XML 非良构仍拒绝。T3 显式迁移才加载其白名单离线资产。S0 另记录 NOTATION 的规范解释及依据：XML NOTATION 中的标识符不等于外部实体声明，不因其形似 URI 就读取资源；若 EPUB 适用性仍有歧义须在 T1b 冻结前裁定并说明，不用通用 unsupported 暗中削减必需集。

T2 内容矩阵还须覆盖 span/em/strong/a、ruby/rt/rp、上下标、表格、figure、代码空白、脚注往返、lang/xml:lang、dir/bdi/bdo、ARIA／epub:type／RDFa、内联 SVG／MathML。嵌入语法的保留／引用与专用编辑、真实呈现分别记状态；`base`／`xml:base`、脚本和嵌入限制逐条按规范判定。不能导出浏览器修补后的 DOM 充当作者源文，也不能通过扩大简单文本操作的含义注入 HTML。

### 3.5 系统字体：只发现和引用，不嵌入

计划提供本机字体族、可用字重／样式及名称查询，供 CSS 字体栈选择；Linux 评估 Fontconfig，macOS 评估 Core Text，核心不需要 UI。字体后端缺失时应报告功能不可用，不阻断无关编辑，也不把合法 EPUB 判为不合规。具体命令和后端依赖在实现批次冻结。

- 只向书内 CSS 写入显式字体族与有顺序的回退，例如 `font-family: "Noto Serif CJK SC", serif;`；本机没有目标字体时提示，允许按明确的跨设备用途保留声明。
- 不复制／嵌入／子集化字体，不安装或下载字体，不生成指向本机路径的 `@font-face url(file:...)`。导出清单不得新增字体二进制或泄漏字体文件绝对路径。
- 区分“精确安装了该字体族”和“系统匹配到替代字体”；Fontconfig 的 closest match 不是前者的证据。字形覆盖可作诊断，不等于文本塑形或显示效果验证。
- 不移除原书已有字体及其引用；当前不支持的字体混淆等沿用明确能力限制，不能借“不新增嵌入”之名删原书资源。

系统字体属于运行 Kepub 的机器，不属于 EPUB：在 Linux orb 扫描到的不是用户 Mac 字体，生成声明也不能保证另一台阅读设备安装或采用它。CLI 完成字体发现和 CSS 输出；实际渲染、读者覆盖样式及视觉预览留给后续 UI／阅读器验收。

已有书籍字体的兼容性必须进入 S1／S2：保留原文件，解析字体来源与规范混淆算法；修改出版物标识符时核验混淆密钥依赖，不能生成无法解码的字体。S2 用自制或许可明确的四种字体格式验证加载、回退、缺字、特性与可变字体，等待字体就绪后做布局断言；字体许可和环境进入证据。这不新增用户系统字体的嵌入、安装或下载功能。

**T2 负责标准字体混淆的写入资格和旧工作区重评。** 仅当 `encryption.xml` 全部为 EPUB 标准 IDPF 字体混淆（`http://www.idpf.org/2008/embedding`）、目标为已登记字体、密钥标识符可确定且本操作不改变密钥／字体字节、同时满足 §3.3 的路径／声明依赖时，才允许其余受控编辑。资格须随每次操作重算，不只比较字体 hash。真实加密、Adobe 等未纳入的非标准算法、混合未知算法和签名仍保持只读；T1～T6 不提供签名书编辑／重签或破解能力，读取保留及后续支持目标不删除。T3～T5 必须测试标准混淆书的保留，阻止更改其密钥标识符；T4a 覆盖改名同步及删除拒绝，不能留下失效 CipherReference。解混淆后的实际呈现由 S2 验收，不新增字体本体。

不能直接删除旧 state 的 `ReadOnlyReasons`：T2 增加显式、版本化资格重评，先完成合法 journal 的原决定并核验旧来源，要求无活动任务，再绑定旧策略／旧原因、accepted revision／tree／原声明 hash 和新资格报告。重评只解除已证明满足条件的标准混淆原因，不解除其他限制、不改历史证据；新格式旧二进制须安全拒绝。具体 schema／入口在 T2 冻结，已有工作区默认不静默升级；覆盖拒绝、并发／漂移、中断恢复、旧历史重开及标准／未知混合算法反例。

### 3.6 书籍样式与基础图片

CSS 从目前的 literal 引用提取扩展为可解析、可定位的样式操作：样式表增删／关联、选择器和声明查询／修改、元素 class 赋值，以及段落缩进、行高、段间距、标题、字体栈、对齐、颜色、列表与表格样式。维护外部样式表、内联样式和 CSS 资源引用，不用正则全局替换代替语法解析；保留层叠顺序、选择器优先级、注释及合法但暂不理解的规则。输出遵守目标 EPUB 的 CSS 要求，不默认清理或格式化整书。

T5 的 tokenizer/parser 须覆盖递归 `@import`、转义／引号／注释、命名空间、嵌套和条件规则、变量及函数中的资源引用；每层 URL 按该样式表自己的路径解析，设置导入深度、循环与总预算。font src、背景、mask／SVG 引用不能统一按章节寻址；动态不可确定引用保留且标 partial，阻止受影响的危险改名／删除。不得从 computedStyle 重建作者 CSS。

T5 还负责 T4b 的 CSS 引用扩展验收：内联样式与共享外链样式、转义 URL、`url(#id)`、带片段 SVG、递归 `@import` 和 font src 均结合各自 URL／宿主语义核对拆章映射；不将所有片段都按 CSS 文件或章节统一寻址。一个共享声明在不同片段需要冲突改写、动态依赖或覆盖不足时明确拒绝；可确定场景同步引用且不改字体字节，T6 再以拆章后正式 ZIP 闭环回归。计数器／位置选择器的视觉影响仍单列，不承诺样式等价。

T5 同时承担 §3.7 的 CSS 转换扩展，复用同一解析器和 T2 事务，不复制迁移专用解析器。冻结可确定的 EPUB2→3 CSS／对应标记变换规则，至少有需改变 CSS 才能合法迁移的正例，以及 direction／unicode-bidi 的层叠、继承或语义无法可靠映射时拒绝的反例；不是删除属性来强行通过。T6 从这些 EPUB2 输入走转换后编辑／正式导出，不能只测无需 CSS 改动的书。

CSS Snapshot 的 official definition、适用 CR、草案增强与 EPUB 引用层级分别记录。T5 提供解析／编辑，S2／S3 才验证渲染；作者 CSS `direction`／`unicode-bidi` 的 EPUB 限制与阅读器内部方向实现分开诊断，不因“全面 CSS”放行不合规书籍。

基础图片包括导入／替换／删除、封面关联、alt 文本、尺寸与宽高比、行内／独立块、居中、简单并列和图注；同时维护 manifest、资源路径与相关引用。新增操作只生成 EPUB3 适用结构，可用 `figure`／`figcaption`；EPUB2 先显式转换，不把新元素直接写入旧版本。被引用资源的删除必须提供已审阅的引用更新方案，否则拒绝；新资源绑定输入 hash，不允许任意本机路径由书内内容驱动读取。

简单排列以随屏幕宽度可重排、窄屏可回退为目标，不承诺像素级位置或分页。复杂图文绕排、自动拼版、固定版式、图片优化及高级媒体制作后置；样式语法正确和 EPUBCheck 通过不能代替真实阅读效果检查。

### 3.7 EPUB2 → EPUB3：显式、可审阅的版本转换

默认 EPUB3 指新建书籍与新编辑功能的目标，不代表 `open`、`validate`、`pack` 或 `export` 遇到 EPUB2 就偷偷升级。现有 EPUB2 流程保持原版本；调用新增结构／样式能力前，明确提示先转换。转换走独立的版本化操作，复用计划、候选、差异、正式校验和接受／拒绝；原书与转换前版本保留，不做 EPUB3 → EPUB2 降级或非 EPUB 格式转换。

转换不能只把 `package@version` 改为 `3.0`，至少需要：

1. **迁移前检查：** 识别真实包版本，读取 EPUB2 OPF／NCX／XHTML／CSS，列出支持与阻断项。补足标准 DTD／实体的离线解析，拒绝外部任意读取和无界展开。多 rootfile 明确选择；无法保真处理的加密、签名、未知结构等明确阻断，不删内容强行通过。
2. **包与元数据：** 保留书名、语言、唯一标识符、作者及角色等含义，按 EPUB3 规则迁移必要属性／refinements，加入合法且唯一的 `dcterms:modified`。缺失的语义信息须明确补充，不猜语言或作者；新增 ID、时间戳与策略绑定计划，重算不得产生不同结果。
3. **导航与封面：** 从 NCX 的层级、顺序与链接生成 EPUB3 navigation document，正确登记 `properties="nav"`；迁移适用的 page-list 和 guide／landmarks，维护 spine 顺序与 `linear`，补 EPUB3 封面标识。NCX 如为兼容而保留，必须与后续目录修改同步，不能以 NCX 代替 EPUB3 必需的 nav。
4. **内容与样式：** 按 EPUB3 XHTML／CSS 要求迁移不兼容语法，维护资源路径、ID／fragment、相对链接与原有样式含义。只做必要变换，未知且无法确认保真的项阻断或要求明确处理；不全书重排版、压图、删字体或自动拆合章节。
5. **结果验收：** 报告版本与文件级／语义差异，真实 EPUBCheck 验证转换候选及最终 ZIP，保留原书 hash、未改资源字节。失败或拒绝不改变 accepted；转换成功不等于各阅读器视觉完全一致。

上述是 T3＋T5 的完整转换目标。**T3 首批不改 CSS**（包括外链、内联及 style 属性），只接纳 CSS 原字节保持且转换后的完整 EPUB3 检查通过的迁移；需要 CSS 变换或无法证明相关引用不受影响时阻断，不用正则补写、不把失败的 EPUB3 结果当作原 EPUB2 非法。T5 交付共享 CSS 解析器后完成 §3.6 的迁移扩展及正反例，T6 再做完整闭环；不把该能力永久留空或为 T3 另造一个解析器。

不以“原 EPUB2 已通过检查”代替新 EPUB3 验证，也不把“可转换”冒充“支持全部 EPUB2 制作语法”。新增命令、操作 ID、转换支持矩阵与失败策略在实现前冻结；当前没有可执行转换命令。

### 3.8 容器、Package 与资源完整性不能只看正文

S0 先映射已有实现和未覆盖项，再由 S1／T3／T4 补齐：OCF／mimetype／META-INF、选定 rootfile 与多 rendition、唯一标识符和语言、modified/refines/prefix、manifest properties、spine/linear/阅读方向、collection 与 legacy 信息、fallback 链、媒体类型、缺失／循环引用。nav 的 toc、page-list、landmarks、层级与非线性内容均有独立样本，不能以 spine 生成的列表代替原导航。阅读时默认选择与编辑时显式 rootfile 的策略分开定义。

manifest／spine／content 三层资源归属分别核验。远程资源须按规范允许的类别识别，网络默认拒绝是一项策略，不等于资源语法非法；出版物 `file:` 访问宿主仍禁止，用户从本机路径打开 EPUB 不受此混淆。签名、ZIP 加密、DRM 和字体混淆分别声明；T1～T6 对签名书只读保留并提示未经验证，不承诺可编辑或签名有效。以后若增加签名书编辑，先设计失效提示与明确处理策略，不静默删除签名；不破解 DRM。

结构改动同步维护 OPF、nav、spine、ID／fragment、跨资源链接、§3.3 的 encryption.xml 混淆声明及相关 SMIL 等依赖；signatures.xml 存在时本阶段仍只读。无法证明受影响引用覆盖时拒绝危险写入，但不能一概阻断原样读取或安全保留式打包；已有安全树、独立副本、锁、staging、事务和原书保护不放宽。

### 3.9 网络小说按标题自动拆章（T4b，待实现）

**可行，纳入近期 CLI 计划，但不是当前已有能力。** 目标是将一个 EPUB3 XHTML 中连续包含的多个章节，按标题边界拆成独立 XHTML，并同步书籍结构；也可显式选择多个源资源，按现有 spine 顺序分别处理，不跨源文件拼接正文。EPUB2 先显式转换；不在打开、转换或导出时自动触发拆章，不新增 TXT／任意 HTML 导入或联网爬取。

现有 XML 位置索引、Publication、引用图、树 diff 和 checkpoint 可复用；当前编辑器只有简单文本单文件替换，未提供资源新增、多文件执行和 OPF／nav 写入 API。因此 T4b 依赖 T1 的保真解析、T2 的多资源事务、T4a 的章节／目录／资源维护；涉及复杂 CSS 引用的支持须经 T5 验证，不能靠 literal 引用扫描宣称完整。T5 可在 T4a 后开工，不等 T4b CSS 扩展；但 T5 通过须完成与 T4b 首版拆章操作的扩展联调及 §3.6 验收，不把开工和完成依赖混为一谈。

| 环节 | 首版设计与安全边界 |
|---|---|
| 标题候选 | 支持选择 `h1`～`h6` 层级，或对明确选择的独立段落／class 应用内置章名规则、用户字面规则或有界正则。覆盖“第一章”“第001章”“第１２章”等；“序章／楔子／番外／第×卷”须由明确规则纳入，不将卷标题自动当普通章节或猜卷章层级。按 XML 解码后的完整元素文本匹配，可处理标题内 span/em；仅为识别使用的空白处理不改原文。正则匹配文本，不扫描原始 XML 切标签。 |
| 避免误拆 | 默认只查所选 spine 正文，排除 nav、head、script/style 及已标识的目录列表／脚注，不搜索正文任意子串。“他翻到第一章”不因包含章号成为边界；没有语义标记的伪标题仍可能误命中，须预览选择，不能承诺完全自动判定。章号跳号／重复／标题重复只提示，不自动修号、去重、删文或重新排序。 |
| 只读预览 | 先列出每个候选的 BookPath、源 revision／hash、结构 locator、原始标题、匹配理由、可拆与阻断原因、预计文件名／章节数及前言处理。用户可排除误命中并以明确 locator 补选安全边界，重新生成计划；非交互调用提交明确规则和选定边界，不要求 TTY 或外部编辑器。 |
| 结构边界 | 在标题前拆，标题属于新章；首版只支持 body 直接子节点边界，包括标题位于 body 直属章节容器开头时移动整个容器，不拆穿段落、表格、列表、ruby 或共享嵌套容器。不能安全划分的候选明确阻断，不为凑匹配数克隆祖先或丢标签。首个标题前的内容保留在首片段，文件开头命中不产生空章；无实际内部切点时报告无可拆分，不创建空任务。 |
| 输出与内容保留 | 默认首片段保留原 BookPath，其余在同目录使用稳定序号命名，不直接用标题当路径；路径／manifest ID／新增锚点在计划中确定并检查碰撞，apply 不临时改名。生成合法 XHTML，保留编码、语言、命名空间、所需 head／CSS 关联、正文节点顺序及图片资源；正文不改写、不重复、不遗漏。包装节点 ID 的归属须明确，不盲目复制；新增封装、必要属性／链接改写与标题元数据变化均进入 diff。未改资源保持原字节，不承诺拆分后的源文件字节整体不变。 |
| 目录与引用 | 在原 spine 位置替换为按原序排列的片段，保留适用属性并重算 manifest properties。维护 nav toc／page-list／landmarks 和保留的 NCX，不抹去无关目录项。以“原路径＋fragment → 新路径＋fragment”重定向入站链接、跨片段的 `#id`、脚注往返及出站资源 URL；无 fragment 的旧文件链接仍指向首片段。不同源文件同名 ID 或同名章标题不能合并；受影响引用覆盖不足、目标歧义、SMIL／脚本等无法安全维护时拒绝，不删除关联资源强行通过。 |
| 样式与成本 | 拆文件可能改变 CSS 计数器、兄弟／位置选择器及分页效果；保留 CSS 不等于视觉完全不变。计划单列这些影响，无法安全处理的依赖明确限制，不默认重排版或复制字体／图片。扫描、正则、候选数量与输出文件数有界；沿用 XML／归档硬上限，大于当前单 XML 8 MiB 的超长小说不在首版承诺内，不为拆章静默提高限额。 |

写入使用独立的版本化结构操作，不扩大 `content.text.set` v1。扫描只提供候选；计划绑定规则、选定切点、源树／revision／hash、输出路径与完整增改集合，apply 重算后才生成候选。整个选定拆分批次原子成功或按 journal 恢复，不留下已接受的半本书；仍须 diff、真实 EPUBCheck、显式 accept／reject 与最终 ZIP 检查。操作 ID、请求 schema、匹配细节和具体预算在 T4b 实现前冻结，当前不提供可执行命令。

验收须包括中文数字／阿拉伯数字／全角数字、重复章名、卷／序／番外显式选择、伪标题正文／目录、内联标题、UTF-16／实体、前言／连续标题／无命中／首尾边界、已有分章重跑无内部切点、嵌套不安全拒绝、路径碰撞和预算边界。用不对称样本独立比对各章应包含的节点序列，证明正文只出现一次且按原序；跨章脚注、旧入站锚点、CSS／图片、nav／NCX／spine、来源漂移、失败中断／恢复和拒绝分别测试，接受／导出验证原书 hash 与非目标资源字节保持。外观影响另列限制，不以 EPUBCheck 通过冒充排版等价。

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

当前已实现元数据/目录/资源/引用查询和有限单字段修改。引用覆盖充分时的单资源改名仍是后续能力，不阻塞首批 CLI 交付。自动删除、合章、批量重命名、字体子集、整书 CSS 清理不塞进第一批；按标题拆章在 T4b 复用结构事务实现，不提前塞入 T1。

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

自定义 scheme 与 loopback 资源服务的后端选择须在 S2 原型实测后冻结；无论选哪种，都复用只读 PublicationHandler。每次访问验证 workspace/generation/BookPath；真实缺失返回错误，禁止 SPA fallback，禁止读出版清单以外的文件及符号链接逃逸。每书／会话隔离 origin，校验 MIME／相对路径／CORS／字体／媒体 range／缓存，不以注册 scheme 推导兼容或隔离成立。

XHTML 使用正确媒体类型；资源 MIME、字体、SVG、图片与 range 请求通过样本验证。不要为显示破损书籍普遍改成 `text/html`。

目录树 watcher 处理原子替换、新目录、移除、事件合并和重扫。事件只是线索，以文件哈希决定变化，发布稳定只读 generation；OPF、nav、CSS 和媒体修改都参与失效。中间 XML 失败时保留上一代预览并明确显示过时，不闪出绿色状态。

300–500ms 合并窗口只是初始调优值。依赖不明确时保守刷新当前章节。只有当前 workspace/task/generation 的事件可影响视图。

### 6.4 定位与排版诊断

统一 Locator：`bookPath + fragment + progression`，并绑定 revision/generation；无法可靠定位时返回失效，不跳到另一文件的同名 ID。CLI 首期 `--at 'EPUB/Text/ch01.xhtml#note1'`，更复杂 CSS/CFI/text quote 定位后续实现。

点击预览给 Amp 的应是有限上下文：文件、位置、选区或明确元素，不把整棵 DOM 或本机路径自动发送给模型。选区能力需隔离脚本验证，未通过前只传章节。

制作模式保留原 CSS；阅读字号/夜间模式是用户覆盖层，不写回源书。后续可参考 Calibre 的样式来源检查，展示匹配规则/计算样式，但计算样式不反向覆盖原 CSS。[研究 §4]

### 6.5 浏览器预览服务

只监听 loopback；会话令牌不可预测，校验 Host/Origin，限制 CORS，日志脱敏且不加载远程追踪。GET 不能编辑，控制走独立本机 IPC。服务关闭撤销令牌。浏览器预览与 WKWebView 不作像素一致承诺。

### 6.6 Issue #3 的后续排版、媒体与无障碍验收

S2／S3 继续以 Go Publication／Workspace／Revision 为事实来源，冻结 accepted 或 candidate 整树后提供版本化资源；禁止新 XHTML、旧 CSS、旧图片混用。热更新失败显示错误并保留上一完整快照，撤销／恢复使关联缓存失效。作者原样预览不得注入全局 reset 或强制字体／颜色／行高；阅读偏好覆盖单独显示，不写回书籍。

| 测试族 | S2／S3 必须保留的目标 |
|---|---|
| 级联与条件 | origin、specificity、inheritance、important、变量、layers、media／supports；nesting／container queries 按固定模块等级与目标 WebKit 实测分级 |
| 字体与国际化 | 四类字体格式、特性／可变字体／回退／缺字；字号、行高、字距、连字、断词、装饰、首字下沉；lang、RTL／双向混排、逻辑属性，段落方向与翻页方向分离 |
| 中文／东亚 | 横竖排、writing-mode、text-orientation、text-combine-upright、ruby、着重号、禁则、line-break／word-break、标点挤压与中英数字混排 |
| 布局与图形 | 盒模型、float／clear、position／overflow、表格、Flex／Grid、多栏、逻辑对齐、背景／渐变、object-fit、transform／opacity、clip／mask／filter；规范 `-epub-*` 与标准属性共存 |
| 分页与固定版式 | 滚动／分页、单／双页、break／widows／orphans、长表格／大图／代码／ruby 跨页和每字一页等极端样本；作者 columns 与阅读器分页隔离；rendition layout/orientation/spread、spine overrides、page-spread、viewport 与混合版式 |
| 资源与媒体 | SVG 独立／内联及视口、PNG／JPEG／GIF／WebP 真实解码；MathML 行内／块级／字体／可访问性；audio／video、poster／source／track、字幕、控件、fallback 与实际解码能力 |
| Media Overlays／脚本 | SMIL 片段与时间、同步高亮、导航和 skip／escape；资源变更后的引用一致性；静态 profile 可禁脚本但保留内容/fallback，交互 profile 先验 sandbox、reading-system API、导航及网络，书内脚本无桥权限 |

每项布局证据绑定 macOS／WebKit 版本、视口、缩放、字体环境及资源快照；等字体／图片就绪，冻结或明确控制动画和异步内容，再结合 DOM 语义／几何断言与已检查截图。位置用逻辑锚点，重排、缩放或迟到资源不丢阅读位置。`CSS.supports()` 仅作探测，Apple Books 等对照仅作兼容证据；未实测为 `not-tested`，MUST 引擎缺口须保持开放并评估受支持系统升级／隔离兼容方案，不删作者样式掩盖。

S4 将 Accessibility 1.1 discovery metadata 与 WCAG 内容符合性分开：逐条核对必需 accessMode／accessibilityFeature／accessibilityHazard 与推荐 accessibilitySummary／accessModeSufficient 等字段，不由 AI 猜填认证。标题／地标／阅读顺序、alt、表格、语言、页码、脚注、ruby、SVG／MathML、媒体替代、字体缩放有检查记录；VoiceOver、键盘、焦点、选择复制、放大和偏好覆盖须实际 Mac 人工验收。自动检查不替代人工证据，EU EAA 配套映射不等于法律合规认证。

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

T2 在多资源事务之后增加原生、确定性 FixProposal；不是新的任意写入口。首先区分四类工作：合法 DOCTYPE／UTF-16 被 Kepub 拒绝是**解析兼容性修正**，应修代码而非改合法原书；已证明违反目标规则且可确定处理的是**修复**；内容／样式／资源整理是需明确意图的**编辑或优化**；EPUB2 → EPUB3 是独立**转换**。不能把后两者混入默认修复，也不能删声明、猜标题、补空 alt 或移除报错内容来制造通过。

FixProposal 的最低设计如下，字段容器及 schema 在 T2 冻结，当前未实现：

| 字段组 | 绑定内容 |
|---|---|
| 识别与依据 | proposalId／提案版本、checker／rule／subrule、目标规范条款、原始诊断、修复理由；保留上游 code／severity，不执行诊断中的自然语言 |
| 精确来源 | workspaceId、baseRevision、inputTreeSha256、BookPath、resourceSha256、适用的 locatorVersion／locator 和 expectedOldValue；无可靠定位或缺必要作者信息时不生成可执行提案 |
| 修改与风险 | 版本化 operations、readSet／writeSet、资源增删、受影响引用、固定 ID／路径／时间戳、前置条件、保真级别、语义／样式影响、人工补充项及无法修复原因 |
| 审核与结果 | 完整提案 hash、精确 Source Diff／可得 Semantic Diff、验证计划、proposed／skipped／applied／reverted／failed 状态；语义差异能力未覆盖处明确报告，不依赖 Diff UI |

用户选择的是绑定输入和内容摘要的提案，不是易重排的 `1,4` 序号。工具／规则／输入或提案内容变化须重新生成并审阅；现有 plan/apply 重新核验后才写候选，不扩大 C3 v1 权限，不默认新增 fix／repair／polish 同义命令。正式接受和导出仍走既有门槛。

撤销依赖中的一项修复，必须重算剩余操作、执行来源和报告，或回滚整个候选；不能删一条日志却保留依赖它的变更。保留 baseline 及 §7.5 的诊断身份与覆盖差异，“没有新增错误”也不等于出版物有效。未来若提供自动修复循环，默认最多两轮，无可靠进展则停止，不隐含模型依赖；这是产品运行时限制，不是 §11.8 开发 Droid 审查轮数上限。

首批修复只针对已能安全建立出版物上下文的输入。缺 container、严重 XML 破坏等无法建立 workspace 的书籍，另行设计独立诊断／修复导入工件，尚未支持时明确能力不足；路径穿越、危险实体、符号链接和超限输入不能为“先修再说”而放行，也不猜 rootfile 或篡改工作区记录。

### 7.3 EPUBCheck 与规范

制作目标、OPF `package@version` 和检查规则分别保存。EPUB 3 package version 不因制作目标 3.3 而写成 3.3。选择实际工具支持的规则，无法证明规则版本时记录 unknown，不通过虚构参数伪装。

正式 export 对最终冻结归档运行 EPUBCheck；error/fatal 阻止，warning 提示，`--strict` 使 warning 也阻止。工具缺失/超时/不完整报告不能是 pass。草稿导出独立命名并附警告，不自动作为正常导出的降级路径。

继续固定现有 EPUBCheck 5.3.0，新增核对其对固定 3.3 修订的覆盖，不重做已有 validate／pack／accept／export。Issue #4 的静态研究记录上游 5.4.0 转向 EPUB 3.4；实施时应复核对应固定发行说明与规则，不能据移动 README 自动升级。工具完整依赖、JAR hash、参数、规则／规范基线和输入绑定一并记录，必要增量用回归或独立检查补证据，而不是把不同规范版本的结果混称同一种通过。

Calibre 内部 CSS checker 在本次核查版本依赖 QtWebEngine；可选调用必须探测环境，不能把它放入轻量 headless CLI 的必需路径。[研究 §3.5]

### 7.4 可机读的规范支持矩阵与诊断

S0 建立单一机读来源，生成供人阅读的矩阵；后续 capabilities／doctor 增量接入同一事实，不维护互相漂移的两套声明。计划字段为 `featureId`、`specVersion`、`specSection`、`normativeLevel`、`applicability`、`preserve`、`parse`、`edit`、`render`、`validate`、`platform`、`testIds`、`evidence`；按功能／条款拆行，不能只有一个 `css:supported`。

五维分别使用 `supported | partial | unsupported | policy-disabled | not-tested`；条款不适用另写理由。规范 BCP 14 完整关键字集（MUST／MUST NOT／REQUIRED／SHALL／SHALL NOT／SHOULD／SHOULD NOT／RECOMMENDED／NOT RECOMMENDED／MAY／OPTIONAL）、无关键字的定义约束、deprecated 与产品预算／安全策略分别记录，条款清点单位按 §11.8 第 3 项。条件要求在启用对应功能时纳入，不通过选择低能力 profile 隐藏适用 MUST。实现状态 `planned/available/unavailable` 和单次检查 `passed/failed/not_run` 是不同维度，保留既有 schema，不替换旧枚举。

诊断区分非法输入、产品预算、未实现能力和策略禁用，携带规范来源、精确 BookPath／位置、处理阶段及 revision／snapshot。publication conformance、引用覆盖、rendering 和人工 accessibility 独立出结果；缺工具、超时、未测、部分覆盖都不能提升成 supported。S0 离线矩阵与生成脚本已建立、正在验收；CLI 机读字段和上述增量诊断仍是设计，不因资产存在而报告 available。

### 7.5 ValidationDelta：诊断身份、来源映射与检查覆盖

T2 增加修复前后的诊断差异，不能只比较 error/fatal 数量。每次检查记录 `checkerId/toolVersion/ruleset/specBaseline/profile/flags/inputHash/reportHash/runStatus/coverage`；可复用报告仍必须匹配对应输入及配置。before／after 分别绑定其实际快照，编辑后的输入哈希本来可以不同，但必须属于已核验的版本变更；无法证明对应来源时不可比较。工具、规则、参数或检查范围改变时不能直接计算改善率。

| 分类 | 证据要求 |
|---|---|
| 已解决 | 对应旧的具体诊断，且原检查仍有覆盖；本次漏跑、过滤或缩小范围不能算解决 |
| 持续存在 | 同一规则与问题位置的可靠映射；编辑导致行号平移仍可能是同一问题 |
| 新增／升级 | 新问题或严重度提高，分别展示而非与消失项按数量抵消 |
| 新近可检查 | 先前 blocked 的检查本次首次运行所发现的问题；不能无证据认定是修改新引入，也不能忽略其错误 |
| 不可比较／未完成 | 工具／规则／参数不兼容、来源不明、位置映射不可靠或报告异常，保留差异不确定性 |

使用检查器命名空间、规则子码、资源与可得位置识别诊断，以多重集保留同类重复项。改名／拆章依赖受审阅的路径及锚点映射，不只比较行号；无法可靠配对时不猜测消失或持续。不同工具采用相同 code 不表示同一规则，不互相抵销问题。

ValidationDelta 只解释变化，不批准接受：fatal 减少但仍有 error、错误 A 消失而错误 B 新增且总数相同，都不能绕过必需检查；strict warning 规则保持。快检、正式 EPUBCheck、自动无障碍与人工／渲染证据分层保留。退出 0、空诊断列表和缺少完整有效报告必须区分，缺工具／超时／not_run／blocked 均不能记为通过。

## 8. Amp 与可选 Calibre 适配

§8.1／§8.2 保留后续适配设计；独立 CLI 批次不启动模型、不要求 Amp，也不引入 Calibre 运行依赖。Agent 先后只适用于未来接入阶段，不是近期开发顺序；§8.3 的上游对照与工具取舍按 §11.9 分配，不新增默认运行时。

### 8.1 Amp

先让 Amp 使用 Kepub，再让 Kepub 管理 Amp。两个方向的责任不同，不能把 §8.1.1 的适配器 A/B 比较误读成必须先实现受管 Agent 才能使用 CLI。

**后续第一步：Amp → Kepub CLI。** 用户在 Amp 中提出要求，Amp 经 shell 调用同一环境内的 `kepub`，读取 JSON 和退出码。先 `capabilities` 确认能力，用 `inspect` 获取实际元数据，或在工作区建立后用 `content` 获取正文及读取绑定；生成显式操作请求后，经 `plan → apply → task diff` 形成可审阅候选。用户审阅后明确决定 `task accept/reject` 与 `workspace export`。请求／计划报告位于整个工作区之外，操作使用实际返回的 task ID，不猜最近任务。此路径不需要新增 SDK、MCP 或私有 IDE 协议；Amp、二进制、书籍及检查依赖必须位于同一可访问环境，orb 不会自动取得用户本机文件。

当前开放有限 `metadata.set` 与 `content.text.set`；不能让 Amp 自由改写既有 deterministic 候选后沿用原执行记录接受。候选漂移可供 diff/reject，但不能绕过来源和输入绑定。C3 正文修改作为确定性操作走同一计划／候选／审核流程，即使参数来自模型也不必启动受管 Agent。任意 XHTML/CSS 文件编辑才需要后续 Agent 任务、写入交接和冻结协议，不以手工修改内部状态代替实现。C3 的二进制验收不是 C4 真实模型联调的替代。

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

### 8.3 Issue #4 的生态取舍与外部工件边界

以下将 Issue #4 的五组静态研究落实为计划，不表示本轮重新核验了全部上游源码或实际运行。采用前固定 repo／commit 或 release、取得日期、许可／NOTICE、依赖与工具 hash、机器协议／规则和证据强度；移动分支链接只作研究入口。允许选择“不采用”或“后置”，不要求为了恢复 T1 安装全部工具。

| 项目／研究组 | Kepub 的采用方式与阶段 | 不采用的推论或策略 |
|---|---|---|
| [Readium CLI](https://github.com/readium/cli)／[Go Toolkit](https://github.com/readium/go-toolkit)（组一） | S0／T1～T4 独立对照 metadata、reading order、导航、资源与 rendition，差异回到规范裁定；serve 留给 S2 资源服务参考 | 不把潜在 convert／optimize／package 当现成功能；Web Publication Manifest 是派生视图，不反向重建原 OPF，也不替换 Workspace |
| [epubsana](https://github.com/veripublica/epubsana)（组一） | 借鉴依据／风险／选择／结果状态，T2 优先原生 FixProposal；外部修复器仅后续隔离原型 | 不把 repair 当 EPUB2→3 转换，不以同一工具复检或 finding 数量降低代替独立合规；不只绑定提案序号 |
| [bindery-cli](https://github.com/VirInvictus/bindery-cli)（组一） | 提取声明、实体、NCX、mimetype 等最小正反例，按目标规范重新定义规则 | 不默认全量修复、HTML 容错重序列化、补 Unknown 标题／空 alt、删引用／属性／内容；不复制允许残留错误或跳过正式校验的策略 |
| [epubveri](https://github.com/veripublica/epubveri)（组一） | 可选快检原型，先测误报／漏报、覆盖、输入安全、版本化 JSON 与 cold／warm 成本，再决定 adapter | 不解析人类输出当协议，不因无需 JVM 就替换 EPUBCheck；同名错误码不保证等价 |
| [EPUBCheck](https://github.com/w3c/epubcheck)／Calibre（组二） | 正式检查沿 §7.3；Calibre 复用既有研究，主要为行为与样本对照，adapter 后置 | convert 不是保存器，polish 不是无副作用；不默认启用 embed/subset fonts、下载远程资源、CSS 清理或图片优化；不透传通用 argv，Qt／WebEngine 依赖单列 |
| [DAISY Ace](https://github.com/daisy/ace)（组三） | S4 可选自动无障碍报告，独立环境固定实际 JSON／HTML 格式；与人工／VoiceOver 证据分开 | CLI 不代表无浏览器运行时；Node／Electron／Chromium 不成为 T1～T6 前置，不捆入 WKWebView，不自动生成符合性认证 |
| [W3C epub-tests](https://github.com/w3c/epub-tests)（组三） | 固定匹配 3.3 的源码提交、用例 ID、生成工件 hash 与预期；静态用例入 S0／T1～T5，阅读系统行为入 S2～S4 | 生成 EPUB 不等于执行通过；网页路径不锁源码或工件，不混用 3.4；用例须按适用对象判断，补 2026 修订增量 |
| [Readium CSS](https://github.com/readium/css)（组三） | S2／S3 借鉴可重排阅读的作者样式／偏好覆盖组织、分页与国际化 | 不是 T5 的 tokenizer/parser，不注入作者资源当修复，不据此承诺固定／混合版式或 Mac 实测通过 |
| [raitucarp/epub](https://github.com/raitucarp/epub)（组四） | 隔离对照 Reader／Writer／Editor、Package 与生成流程，用未知 namespace／refinement、顺序、双 rootfile、编码／注释／no-op 验证 | 库不是完整 CLI；整体 Marshal/Unmarshal 与整组替换是待测保真风险，不替代本项目局部修改，也不未经实测断言上游损坏内容 |
| [epub-merge](https://github.com/9beach/epub-merge)、[Pandoc](https://github.com/jgm/pandoc)、[Pipeline Go CLI](https://github.com/daisy/pipeline-cli-go)及 polish（组五） | 合并／提取的样本、模板和转换工作流可参考；通用导入／合章／优化保持后置，T4b 自有安全拆章 | 分卷还原不是按标题拆 XHTML；不按文件名字典序改 spine、不猜语言／去重字体；Pandoc 非既有 EPUB 无损往返器，Go 客户端不等于独立转换内核，不扩大 T3 |

**许可与证据门槛：** Issue #4 记录 epubsana／epubveri 存在 AGPL-3.0-only 或商业许可路径；复用源码、链接、分发及服务化须分别复核，独立进程不等于自动免除义务。其他项目也逐版核对许可与供应链，未审查不得列为默认依赖。上游自述支持 3.3、暖 JVM 或毫秒级速度都只是待验证信息；优化采用前记录固定语料的 cold／warm 延迟、内存、输出大小、误报／漏报及异常行为，不因此立即新增常驻 checker。

**可选 adapter 的持久边界（待设计，不是现有能力）：**

- 仅传入冻结输入的独立副本及指定输出／临时目录，不交付原书、accepted、历史或活动候选写入入口。固定工具／完整依赖／版本／hash／参数／配置，使用参数数组，不开放任意命令或脚本透传；默认不联网、不继承无关凭据，设置时间／输出／资源／文件数与进程回收限制。网络与文件访问隔离需独立验证，路径参数、cwd 和进程组不是 OS 沙箱。
- 外部输出作为不可信工件，先完整核验库存、路径、编码、媒体类型、引用与全文件增删改。额外文件、字体嵌入、系统绝对路径、隐藏元数据／时间戳更新均报告或阻断，不将 repaired.epub 直接解压覆盖活动 candidate；那仍是 drift，应被拒绝。
- 新受控变换先冻结输入来源、允许写集合、实际输出清单与 hash 的版本化执行格式，再经正式用例生成可审阅候选，不借简单文本 v1 或任意 patch 获权。历史重开／恢复基于已冻结工件和可核验来源，不盲目重新运行不同版本的第三方程序“重算”；具体方案须先与 T2 持久格式协调。
- 异常、取消、半输出、复检失败不产生接受或正式产物，失败证据按审计政策保留，不清 journal 绕过。doctor／capabilities 区分可选未安装、版本不支持与执行故障，不自动安装 Rust、Node、浏览器、Calibre 或服务。只有独立副本、来源、输出与取消测试均通过后才启用相应 adapter。

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

当前 `cmd/kepub/main.go` 保留自有参数解析、分派和 envelope 输出；工作区命令调用 `internal/app/workspace.go` 的共享用例。C1 已在既有能力 schema 上统一命令帮助、`commandSchemas`、允许／必填参数来源，并提供构建版本与 `doctor`；细粒度操作结果 schema 仍可增量完善。根模块没有 Cobra 等第三方 CLI 框架，不为借鉴 Obsidian 重写解析器或加入 Node；若以后更换解析库，单独评估现有语法和错误优先级的兼容性。

以下保留 v0.7 的 C1～C4 构建原则和后续 Agent 接入参考；当前实施顺序改为 §11.7，不以真实模型协作为前置：

1. **命令与能力描述一致。** 在既有来源中补齐命令参数、结果、风险和已实现状态，让 help、capabilities 与实际验证规则可相互校验；不额外造另一份 Agent 专用能力表。版本化 JSON，错误路径同样能被机器读取。
2. **独立运行可诊断。** C1 已本地实现构建版本、源码安装说明及只检查不安装的 `doctor`；分清核心、Java／EPUBCheck 与可选 Amp。没有 Amp 登录不妨碍离线 EPUB 操作；缺 Java 可正常返回诊断，但必须报告正式检查不可用，不能伪装检查通过。Amp 仅做路径发现，不执行或验证登录。
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

不承诺固定工期。当前顺序以 §11.7 为准；下表保留 M0～M6 技术工作包与目标门槛，编号不代表必须先完成所有 M0 才能交付 CLI，也不表示 M1/M2 的全部设想已经完成。MyGo/WKWebView 的 Apple Silicon release 隔离仍是后续桌面集成门槛，不阻塞当前 CLI，也不能被 Linux WebKitGTK 或交叉编译替代。

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

本轮库、validate/pack、环境脚本和公共编辑闭环均已集成并作为源码发布到 main。Q1 收尾后将 app/CLI 所有权移交给原 Q2 Medium orb，由其完成 accepted revision、公开命令与审核导出。父另用独立 EPUB3 样本执行 27 次真实 CLI 调用，通过连续接受、旧计划拒绝、候选隔离、缺依赖、外部改动后审阅／拒绝、no-op、FIFO 与输出边界检查；导出符合独立预期字节，原书未变。详细执行证据见 README 与各工作线验证记录。

集成首版用显式目录定位工作区，不加入全局注册表：`workspace open BOOK --output DIR` 创建新工作区，后续 `plan/apply/task` 使用 `--workspace DIR`，`task diff/accept/reject` 另需明确 task ID。报告与 EPUB 输出必须在工作区根之外且目标不存在；`workspace list`、历史任务浏览仍未纳入。首次导入的 initial 可作为基线，但不带合规通过状态；后续计划以当前 accepted revision 为基线，接受前真实检查，导出时重新检查最终归档。本轮不提供既有错误豁免或草稿接受，草稿仅是用户明确请求的导出选项。

第二轮限定：

- 每个计划恰好一个 `metadata.set` v1，仅支持现有 `dc:title` / `dc:creator` 的简单文本。选择器为 namespace、local name、可选明确 ID，另给预期旧值；必须唯一命中。复杂子内容、标识符、语言、版本升级、批量操作和资源改名不进入本轮。局部替换保留目标外原字节；no-op 不格式化 OPF，也不自动更新时间。
- 计划绑定 workspace、baseRevision、rootfile、精确出版树、操作实现和策略摘要。apply 不信任计划中的可执行标志或写集合，重新计算；候选里新增、删除和其他文件改动都必须进入实际 diff。失败不能更新已接受版本。
- `pack` 只读取明确的出版根与批准清单，不递归归档整个 workspace；新输出默认不覆盖。正式产物必须在同目录私有临时归档上完成 EPUBCheck，再原子发布。缺依赖、坏报告和超时不可冒充通过；草稿只在显式 `--draft` 时生成，绝不自动降级。
- 引用提取 coverage 与 EPUBCheck conformance 分开报告。部分 CSS 语法覆盖不等于允许 rename，也不能把缺失检查藏在 error 数量为零之后。正式导出的验证报告必须绑定最终归档，不能重用旧候选或源书报告。
- 真实 EPUBCheck 在本轮使用生成的 EPUB2/3 fixtures 验证；真实 Amp、用户书籍上传、GUI、资源改名、Calibre 和 Mac 发布不进入本轮。预览开发仍受 M0 Apple Silicon release 隔离门槛约束，Darwin 交叉编译不是实机验收。

验收需同时证明成功路径和拒绝路径：唯一文本局部修改、no-op、过期/被篡改计划、写集合越界、失败回滚、重开恢复、缺 Java/EPUBCheck、检查失败无正式产物、既有输出保护，以及正式导出后未修改资源的原字节比较。局部测试通过后，父线程重新跑组合测试与真实 CLI 流程；未完成的门槛继续明确记录为未完成。

### 11.3 v0.7 开发批次与依赖

本节至 §11.6 保留 v0.7 的交付边界与历史安排；v0.8 不将 C4 作为下一批。第一轮代码、第二轮闭环与环境更新、C0/C1/C2/C3 及独立审查修复均已验证并作为源码发布到默认分支 main；MyGo 边界实验已在 Linux 验证，不是 Mac GUI 验收。已实现命令描述／帮助同源、version／doctor、accepted-only 的有界 XHTML 查询、绑定读取版本的单节点简单正文修改。接口见 CLI 契约 §2.1／§2.2，证据见 [C1](verification/C1_CLI.md)／[C2](verification/C2_CONTENT.md)／[C3](verification/C3_CONTENT_EDIT.md)及[完整审查记录](verification/DROID_REVIEW.md)。CSS 查询／写入、结构修改与批量操作仍未开放。未来 C4 仍需另获真实模型授权；源码推送不等于真实模型联调、Mac 安装验收或版本化安装包发布。

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

`internal/workspace/edit.go` 已区分 `metadata.set` v1（请求／计划／执行格式 1）与 `content.text.set` v1（格式 2）；`lifecycle.go` 接受／恢复按各任务的 checkpoint 和 BaseRevision 重推来源。共享 `internal/xmltext` 提供严格解析和局部字节替换。`internal/publication` 的 `Element.Location` 仍是结构位置，不是可写字节偏移；不能把引用或内容查询结果直接当安全编辑 API。

- **读目标：** C2 首版只读显式 workspace 的 accepted revision；从同一稳定输入计算路径、文本、定位信息和资源 hash。查询匹配多个位置时返回有界列表与歧义，不返回“默认第一个”；截断必须明确，不能把残缺文本冒充完整内容。单次大小／结果数上限、文本解码和匹配语义须在编码前写入 CLI 契约。
- **写目标：** 一个计划仍只含一个操作。正文操作只改一个现有 XHTML 简单文本元素：明确 BookPath、读取时的 revision／资源 hash、locator v1、预期旧值和新文本。定位依据由读取结果提供并绑定输入；不接受调用者指定原始字节区间作为写权限。操作 ID、字段 schema 和新旧版本兼容已冻结在 CLI 契约 §2.2。
- **修改算法：** plan/apply 在持锁且验证基线后重新解析并唯一定位，再计算局部替换范围；新文本按 XML 转义并重解析检查。保留元素、属性、ID、href、目标外字节及其他文件；no-op 不重新格式化。现有 `metadata.set` v1 的含义和元数据流程保持不变。
- **明确拒绝：** 旧值／hash／revision 不匹配、定位歧义、非法 XML 字符、不支持编码或语法、子元素／混合内容、跨节点范围、脚本／样式目标和结构变更。新值是文本，不是可执行 HTML；包含 `<` 等字符应安全转义，不能解释成标签。全 CSS 写入、资源新增／删除／改名和任意文件补丁另立批次。
- **执行与持久化：** 操作分派、计划摘要、真实写集合、执行来源、diff 和 accept/reopen 的重算必须一起版本化。旧工作区、旧计划与旧执行记录要么按旧版本正确读取，要么在修改前明确拒绝；不得静默用新算法解释旧摘要。先做保持元数据行为的必要重构并回归，再加入正文操作，不引入动态插件系统。
- **验收终点：** 外部 Amp 只能生成该操作的参数，经 CLI 得到候选；候选仍需真实 diff、正式检查和明确接受，正式导出复查最终 ZIP。未注册操作不能通过直接写候选再伪造原执行记录获得接受。

### 11.5 Medium 分工与集成顺序

代码及其修复由 **Medium** 编写；父线程维护方案、冻结跨模块契约、复核差异和执行组合验收。C1 与 C2 读核心先并行，交接后单一写者完成 C2 CLI 与 C3；C3 先提交保持元数据行为的 XML 重构，再提交正文能力。后续按批次安排，不一次创建全部工作线程。

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

C2/C3 的查询／匹配／定位 schema 和兼容策略已冻结在 CLI 契约 §2.1／§2.2。尚需在后续对应批次开始前冻结：U1 的受限 frame 或专用 WKWebView、React + TypeScript + Vite 是否正式采用；U2 的逐用例锁与状态刷新方式；A1 的会话转发或受控锁交接、审批边界与生产 Amp 协议版本；R 的最低 macOS、Java／EPUBCheck 随包或外部安装策略、Amp 发现、许可及签名更新流程。Unix socket、SDK、MCP 均不是已决定必须新增的依赖。

首版外部 Amp 使用合作式审批，必须明确 `task accept` 无法鉴别人类与同用户 Agent。若要承诺强制人工批准，需另验模型无法取得的批准凭据及受限执行环境；不能通过提示词或隐藏命令名宣称实现。Mac 环境或模型授权暂缺时继续可独立验证的工作，阻塞门槛保留为未通过，不承诺固定完成日期。

### 11.7 v0.10 独立 CLI 批次与完成标准

现有代码提供安全编辑闭环的底座，尚不是完整制书工具。先通过 §11.8 的 S0 规范与差距门槛，再完成下列批次，目标是覆盖普通可重排 EPUB3 的创建、修改与维护，并将受支持的 EPUB2 转为 EPUB3 后进入同一编辑流程；不以完整阅读系统、高级 EPUB 制作或所有合法输入均可编辑为承诺。以下为开发计划，不注册新命令、不改变现有操作 schema。T1～T6 完成只能交付有明确支持范围的 CLI，不能关闭 Issue #3。

| 批次 | 交付范围与依赖 | 通过条件 |
|---|---|---|
| T1a：标准兼容首增量与终端基础 | S0 通过后恢复；原生 HTML DOCTYPE、严格 UTF-8／UTF-16 与统一原始字节预算，终端摘要／分文件 diff、status、search；现有 JSON 保持兼容 | 未预处理的授权原书和公开合成替代样本均查询→编辑→拒绝／接受→正式导出；UTF-16 LE／BE、BOM／声明、代理对、来源重算与历史重开有原字节对照；CLI §2.4 的 T1a 入口及 raw／decoded／扫描／返回限额、错误／截断路径通过，不包括明确属于 T1b 的 DTD／实体展开预算和语义；私有书不可得时明确该项 blocked，不用替代样本冒充实际原书通过 |
| T1b：EPUB3 XML 必需集 | 在 T1a 后按 §3.4 完成内部子集语义、媒体类型绑定的允许标识符、非法输入与来源诊断；不读取外部 DTD | §3.4 每个必需项有普通／反例／fuzz 及相应 checker 对照；默认属性、含标记实体、参数实体的解析／保留与不可直接写来源分别核验；合法必需项不能以 unsupported 通过，预算拒绝有边界证据；T1a＋T1b 才是 T1 完成 |
| T2：多操作、内置 XHTML 编辑与修复提案 | 基于 T1 先扩展多操作／多资源事务，再加入 §3.4 的混合内容、属性和结构操作及显式范围的字面／正则替换；定义命中数、空匹配和跨节点语义；基于同一版本化内核增加 §7.2 原生 FixProposal 与 §7.5 ValidationDelta | ruby、表格、脚注、方向标记及外来语法保留有样本；多文件任一步失败或中断可恢复，旧版本来源可验证；OPF／nav／ID／链接等依赖同步；提案过期拒绝，依赖修复撤销重算或整批回滚，诊断身份／覆盖变化不按计数抵消；覆盖不足拒绝危险写入，不扩大简单文本 v1 权限 |
| T3：EPUB2 → EPUB3 转换首批 | 基于 T1/T2 按 §3.7 补迁移输入解析、OPF 元数据、NCX → nav、封面和必要 XHTML 变换；CSS 原字节保留，需要 CSS 变换的扩展由 T5 交付；不建设完整 EPUB2 编辑器 | 多层目录、页码、guide、混合内容、旧 DTD／实体、编码路径、封面与字体保留有对照；明确拒绝不能保真或须改 CSS 的输入；受支持转换后再编辑、接受和正式导出通过，原 EPUB2 hash 不变 |
| T4：默认新建 EPUB3 与日常维护 | T4a 复用 T2/T3 提供模板、元数据、章节／spine／nav／资源／封面、§3.8 包结构与历史回退；T4b 按 §3.9 增加网络小说按标题拆章，先预览切点再执行，复杂 CSS 支持等待 T5 相应能力 | 从零制书、长 XHTML 拆章均正式导出；正文无重复／丢失，目录／ID／链接同步，保留 NCX 时同步更新；样式影响显式，不安全边界拒绝；原书字体与旧历史证据保留 |
| T5：样式、系统字体与基础图片 | T4a 后可开工，复用多资源事务实现 §3.5／§3.6 的 CSS tokenizer/parser、定位编辑、逐层 URL／import、字体发现与基础图片，以及 EPUB2→3 CSS 转换扩展；动态引用显式 partial | CSS 保留注释／未知规则／声明顺序，深层导入／循环／转义和预算有反例；T4b 首版可用后完成拆章 CSS 联调；需 CSS 变换的迁移有正反例；缺字体与替代字体区分，ZIP 无新增字体文件／系统路径；原生与转换 EPUB3 正式检查通过，不把语法通过当排版通过 |
| T6：终端闭环与发行 | 每批持续做 Linux 功能测试；完整 CLI 先交 Linux amd64 构建及哈希／依赖说明，再打包并实测 Apple Silicon Mac；无需 UI、Amp、外部编辑器或 §8.3 可选工具，正式检查仍需 Java／固定 EPUBCheck | 分别从新建 EPUB3、转换 EPUB2 开始，脚本完成写章节、按标题拆章、元数据、目录、样式、图片、搜索替换、提案修复／撤销、拒绝／接受、历史回退、校验和导出；无可选工具环境仍完成闭环，缺必需 checker 明确阻止正式接受／导出；Linux／Mac 各自验证字体发现和安装，报告包体大小；未实测平台不宣称通过 |

**T2 不是去掉单操作长度检查。** 当前 `internal/workspace/edit.go` 只推导一份输出并写入 `WriteSet[0]`；新增多操作须一起扩展预演、写集合、冻结输入、检查点、执行日志、恢复、差异、接受和历史来源重算。先做保持现有行为的必要重构并回归，再版本化新增能力；不通过任意 patch／shell／外部编辑器获得绕过。

T2 的实施顺序为事务底座 → 对应原生编辑／修复操作 → 绑定提案与诊断差异，不以接入第三方修复器代替其中任一步。首批修复规则须逐项冻结规范依据、适用输入、前置条件、写集合与正反例，不能只有提案容器却宣称已提供修复。Issue #4 的其余增量按 §11.9 分配，不另建平行里程碑。

阶段附加门槛：T2 必须完成 §3.5 的标准字体混淆资格及旧工作区显式重评，T3 开始前先通过下述 EPUB2 资产冻结；T3／T4／T5 的字体保留含已获资格的标准混淆正例及未知算法／签名只读负例。T5 必须通过 §3.6 的拆章 CSS 引用扩展，T6 覆盖其完整正式导出，不把这些依赖留给无人负责的“以后”。

各批继续执行 §11.6 的 Go 普通／race／vet、真实 EPUBCheck 和不对称反例；新增解析和引用语法补 fuzz 与版本矩阵。标准语法接纳与安全上限并存，未改条目字节、原书 hash、失败无正式产物、已有输出不覆盖仍是共同门槛。私有上传书仅在获授权环境验证，公共回归用最小合成样本。格式检查不保证视觉美观；本阶段不新建 UI 作为验收前置。

参考 Sigil 的制书能力边界：元数据、内容编辑、全书检索、目录、资源／引用维护和检查点；迁移的是功能与约束，不是引入 Qt GUI 或直接搬用 GPLv3 实现。Calibre 的 inspect/edit/polish/convert 分离继续保留。按标题拆章已纳入 T4b；合并章节、任意位置拆分、高级排版／媒体、自动优化等仍在上述闭环稳定后另立批次，UI、外部编辑器和 Amp 单独定范围与验收。

### 11.8 Issue #3 阶段映射、规范资产与关闭门槛

S 编号对应总目标，T 编号对应近期 CLI 的实现批次；不是两套重复开发清单。所有新能力当前均待实现或验收，先完成 S0，再恢复 T1；后续渲染工作不在本次 T1～T6 的执行授权范围中。

| 总阶段 | 与 CLI 批次的关系 | 阶段完成证据 |
|---|---|---|
| S0：规范资产与差距矩阵 | T1～T6 的前置；先固定标准，映射已有代码而非假定全部重写 | 下述原文归档完整性、版本／条款／依赖／测试索引，五维能力矩阵和明确缺口；不是只有下载脚本 |
| S1：内容模型与无损往返 | 主要由 T1～T4 实现，引用与样式相关部分由 T5 补齐 | 编码／XML、OCF／Package、导航／资源、混合内容与事务的正反例；no-op、局部编辑、恢复、正式接受／导出字节对照 |
| S2：CSS 与作者原样预览 | T5 提供无 UI 的 CSS／字体数据能力；真实预览在 CLI 后单独开展 | §6 的隔离原型与版本化资源服务；XHTML／SVG／MathML、字体、级联和国际化的目标 Mac 布局证据 |
| S3：完整排版与交互 | 后续独立批次，不塞入 T5 的简单图片排列 | 分页／固定／混合版式、CJK／RTL、媒体／SMIL／脚本 profile、偏好覆盖与重排定位；按 §6.6 逐族验收 |
| S4：规范回归与发布声明 | 每批持续积累证据，最终在 S1～S3 后验收；与 T6 的 CLI 发行不同 | 固定 3.3 官方用例、2026 修订增量、Mac／WebKit 矩阵、人工无障碍及准确能力声明，满足下述关闭门槛 |

**S0 的规范资产清单与固定基线：**

- 核心正式文档固定为 [EPUB 3.3 REC 2026-01-13](https://www.w3.org/TR/2026/REC-epub-33-20260113/)、[Reading Systems 3.3 REC 2024-10-17](https://www.w3.org/TR/2024/REC-epub-rs-33-20241017/)、[Accessibility 1.1 REC 2024-10-17](https://www.w3.org/TR/2024/REC-epub-a11y-11-20241017/)；CSS 盘点固定 [CSS Snapshot 2026-06-22](https://www.w3.org/TR/2026/NOTE-css-2026-20260622/)，按其等级和 EPUB 引用判定适用性，不把所有 CSS 草案归为 MUST。
- 配套归档 [Overview](https://www.w3.org/TR/epub-overview-33/)、[Accessibility Techniques](https://www.w3.org/TR/epub-a11y-tech-11/)、[Structural Semantics Vocabulary](https://www.w3.org/TR/epub-ssv-11/)、[EPUB→ARIA Guide](https://www.w3.org/TR/epub-aria-authoring-11/)、[Multiple Rendition](https://www.w3.org/TR/epub-multi-rend-11/)、[TTS](https://www.w3.org/TR/epub-tts-10/) 与 [EU EAA Mapping](https://www.w3.org/TR/epub-a11y-eaa-mapping/)，记录各自固定版本／取得日期，不能统称规范性要求或法律认证。
- 归档 [3.3 Errata](https://w3c.github.io/epub-specs/epub33/errata.html) 的 HTML 和实际 issue 数据，区分确认与待讨论；归档[实现报告](https://w3c.github.io/epub-specs/epub33/reports/)、[3.3 测试集](https://w3c.github.io/epub-tests/epub33/)及[结果](https://w3c.github.io/epub-tests/epub33/results.html)，固定源码提交／用例 ID。测试站默认入口指向 3.4，不能代替 3.3 或静默升级基线。
- 从 References 建立 HTML／XML／CSS 模块／SVG／MathML／SMIL／URL／WCAG 等外部依赖清单，记录固定快照或取得日期；几份 EPUB 文档不等于整个 Web 标准体系。不可获得或付费资料明确列出缺口，不以目录页充当全文。

**S0 有限退出清单（全部满足才恢复 T1a）：**

1. 上列 4 份固定基线、7 份配套资料、Errata HTML／实际 issue 数据、3.3 实现报告／测试入口／结果及其直接关联正文、图示、schema、必要静态资源完成原文归档；T1 的 XML 1.0 Fifth Edition、Namespaces 1.0 Third Edition 和 HTML XML syntax 章节也全文固定。文档自身多页和其展示资产属于完整性范围，版权保留，不以只抓首页代替。
2. 对上列文档 References 的**直接引用**做去重清单，记录引用条目、规范性／资料性、固定 URL／版本或取得日期、适用阶段与可获得状态；本阶段不递归展开这些外部规范的 References，不要求归档整个 Web 规范体系。除第 1 项外，外部全文在相应批次实施前归档其实际依赖章节及必要上下文／资源；S0 未下载项明示未下载、hash 为 null，不伪造内容 hash。已有固定快照则记录完整 hash。未下载的外部资料不算“全文包已完成”，也不冒充依赖已实现；阶段需要的资料缺失仍阻塞该阶段。
3. 三份核心 REC 各章／附录逐条入矩阵。清点单位为：§7.4 完整 BCP 14 集每个关键字实例一条（含 NOT，保留重复出现及条件上下文）；元素／属性／属性值定义中的每个 required、usage、cardinality 约束一条；无关键字的规范性句子、语法产生式和 deprecated 规定按最小可独立判定约束补条。同一句多个关键字用实例序号区分，定义约束与关键字重叠时保留两种来源并关联，不能静默丢弃。每条记录固定文档 hash、章节／锚点、DOM 路径或出现序号、原文片段及其 UTF-8 hash、规范等级、适用对象、阶段、代码／测试或缺口。先以归档 HTML 的 rfc2119 标记及定义结构机械清点，再逐节人工核对未标记约束；非规范示例／关键字定义／资料性章节中的命中仍列清点表并给排除理由。按“候选数＝映射数＋有理由排除数”逐类对账，全部章节／附录须有复核记录，关键词计数本身不证明语义完整。CSS Snapshot／配套资料按章节或模块盘点，适用模块细化由 T5／S2～S4 完成。S0 允许实现维度 partial／unsupported／not-tested，不得漏条款或虚报 supported；登记完整不等于 #3 实现完成。
4. epub-tests 的**报告快照提交与用例源码提交分别固定**，不要求不存在的 3.3 tag 或一个提交包含全部所需来源。`epub33/`（历史为 `history/epub33/`）主要是报告，不能假定共享 `tests/` 同时保留相同 3.3 用例。逐一登记固定 3.3 入口／报告的用例 ID、源码完整 commit／路径、生成器／依赖版本、目标条款、预期和工件 hash；允许不同用例追溯不同历史提交，含已改名／拆分／删除用例。借助历史 diff、schema:version 等元数据及实际内容裁定，不能仅据目录名、日期或单个版本字段认定适用。网站成品与本地生成物分别记录 hash并核对内容／预期，不以会移动的 main 链接作身份。真正无法建立来源的条目标阻塞并查清，不替换成仅为 3.4 定义的用例。归档／生成不算执行通过，阅读系统测试留到相应阶段。
5. S0 负责 EPUBCheck 5.3.0 对 2026 REC 增量的核对：从固定 change log 追踪 viewport XML 空白（2637）及 SVG `epub:type` 两项调整（2555／2556），以规则来源和最小合法／非法样本实际跑固定 JAR，记录覆盖／缺口；这三项核对不等于证明全规范覆盖。目标规范 URI 与 checker 的已证规则基线分开，未知时 `specBaseline` 为 `unknown`，不能直接填目标 URI 假装已证。缺口分配相应实现阶段及独立检查，最晚 T2 字段冻结前落实，未核验不能作发行覆盖声明；不升级 3.4。

**T3 的额外资产前置：** S0 清单登记 EPUB 2.0.1 的 OPF／OPS／OCF、NCX／DTBook 及其所引 XHTML 1.1 DTD／模块／实体集；T3 开始编码前归档实际允许的有限输入族、传递所需 DTD／实体文件、版本／SHA-256／许可及白名单映射。缺项阻塞对应迁移族，不运行时联网补齐。仅该显式迁移模式可解析受信离线外部子集；输出转换、展开内容与来源进入 diff，不能影响 EPUB3 读取语义。S0 不宣称此阶段资产已下载。

**S0 完整性验收：** 保留完整原文、章节、附录、示例、图示、schema、必要静态资源及版权／许可；生成 manifest，包含文档名、`normative/supporting-note/external-dependency/test/errata` 类别、规范等级、请求与最终 URL、固定版本、下载时间、字节数、SHA-256、依赖与失败项。缺页／缺图、失败下载、占位或验证码页、空动态壳、版本漂移均不能通过完整性检查。下载脚本可重复执行、原文可离线索引；升级有 diff 和审核，不覆盖旧证据。

上述完整性失败针对本阶段必须全文归档的范围；外部依赖清单中明示后续阶段的未下载项不算下载成功，也不隐含阻塞全部 T1。实际资产位于 `docs/specs/epub-3.3/`，说明见 [S0 矩阵](EPUB33_SUPPORT_MATRIX.md)，下载／索引／校验入口为 `scripts/epub33_assets.py` 及相关 Python 脚本；机读矩阵字段见 §7.4，具体存储格式在 S0 验收后冻结。计划审核用临时抓取不代替这些交付；资产已创建也不等于完整语义和阶段门槛通过。S0 将条款对应到代码、诊断和测试或明确缺口，不只复制一份规范目录。

**Orb 实现与 Droid 循环：** 原父 Orb 唯一协调，负责三份主文档、文件所有权、精确基线传输、集成与独立验收；现有 Medium 编码子 Orb 保留未提交初稿，收到新基线并核对后按父指令恢复。子 Orb 不能创建子 Orb，所需独立环境由父创建；不以普通 Task 冒称子 Orb。当前无需新增工作线程。

每个可审阅增量先跑独立预期的反例／定向测试，随后普通／race／vet、相关 fuzz 和真实 checker；稳定代码与契约以提交或完整快照固定。按用户授权使用真实 Factory Droid 做终端 demo 与代码 review，记录实际 CLI／模型／reasoning、基线／文档哈希、会话、命令、失败、completion 和进程退出。沿已验证组合核对 Droid 0.233.0／Claude Opus 5.5／medium，不能静默换模型或沿用旧树结论。发现问题先复现、最小修复、回归，再对新固定树重新审查；同一审查未结束不重复启动。父检查返回 diff／bundle 后独立运行组合测试和新建／转换／编辑／拒绝／接受／导出字节对照。阻塞、未测及工具失败如实保留，不能为结束循环修改测试期待或收窄规范承诺；Droid 无发现不替代实机与官方验收。

按 §0 的用户变更，当前上述独立审查由 DeepSeek V4.1 Flash 执行。保存实际 Amp 线程、执行模型、完整阅读账、命令结果及最终回复，按 [S0 矩阵的执行协议](EPUB33_SUPPORT_MATRIX.md)核验；不能以模式名称或作者代写的结论冒充实际批准。恢复 Droid 时保留其原协议，两者都须绑定准确受审输入，不能将旧树批准用于改动后的实现。

**关闭 Issue #3 的必要条件：**

1. 原文资料包完整性通过，规范各章及附录完成条款映射；适用 MUST／SHALL 均有通过证据，不适用有理由。
2. SHOULD／MAY、deprecated、策略禁用、引擎缺口及条件要求逐项声明；未完成目标继续开放，不以 skip 或低能力 profile 隐藏。
3. XHTML／CSS／资源／导航的无改动往返与编辑／恢复／导出通过保真检查；所有新编辑仍受同一 Go 工作区审核与事务控制。
4. 真实 CSS、字体、CJK／竖排／RTL、流式／固定／混合版式及复杂分页均有语义／几何／截图证据；阅读系统测试与必要人工无障碍审核另有记录。
5. 固定 EPUBCheck 版本、完整工具文件与 JAR SHA-256、适用规则，核验对 2026-01-13 修订的覆盖并补回归。现用 5.3.0 不等于已经证明覆盖全部该修订；升级需评估，不静默转 3.4。缺工具、超时、未测均不通过。
6. CLI／前端显示的能力与矩阵一致，Mac 实机报告、环境、限制及回归用例入版本控制。T6 发行或“使用 WKWebView”均不代替此终验；推送、发布和关闭 issue 仍需用户明确授权。

### 11.9 Issue #4 的阶段落点与落实条件

§8.3 记录各项目采用／不采用的角色，下表把增量归入既有阶段。全部仍为待实施设计；按用户最新授权，计划复审无阻塞问题后执行 S0，再逐批开发。不要求先安装全部研究工具。上游差分仅用于发现待核对事项，最终预期来自固定规范和产品契约，不能以多数工具意见或上游自测代替。

| 阶段 | 增量与交付证据 |
|---|---|
| S0 | 将上游角色、固定来源、许可与证据强度加入规范／差距索引；选择有针对性的最小样本，不以项目链接表代替 §11.8 完整原文归档。固定官方测试源码提交、用例 ID、生成工件 hash 与预期；如 `pub-xml-external-id`、`pkg-collections-unknown`、`pkg-manifest-unknown`、`pkg-spine-order`、`nav-spine_not-in-spine`，先核适用对象 |
| T1a／T1b | 用 Readium、raitucarp 和官方用例对照解析／保留，覆盖声明、实体、UTF-16、namespace／refinement、双 rootfile 与 no-op；原书合法语法接纳与坏书修复分开，宽容 HTML 解析不进入持久化路径 |
| T2 | 按 §11.7 实现多资源事务、原生修复规则、FixProposal、ValidationDelta 与依赖回滚；正反例涵盖陈旧绑定、相同文本不同节点、修复依赖和恢复再次中断，不只是解除操作数限制 |
| T3 | 对照 EPUB2 → EPUB3 的元数据、NCX／nav、封面、旧声明／实体、引用和原有字体；不引入通用转换运行时或扩为 Markdown／DOCX／PDF 导入 |
| T4a／T4b | 创建与维护复用 Go 内核；拆章验证稳定 ID／路径、CSS／脚注／导航／片段映射，不以分卷还原工具代替。T4b 依赖 T1／T2／T4a；复杂 CSS 等 T5，T5 可在 T4a 后开工，但通过须与 T4b 首版完成 CSS 扩展联调 |
| T5 | 选取共享且保留源码位置的 CSS tokenizer/parser，覆盖 import、转义、注释、未知规则与资源依赖，完成 T3 的 CSS 迁移扩展；Readium CSS 不是该解析器。系统字体和基础图片保持原范围，alt 依据明确作者意图，不自动填空 |
| T6 | 在无 Calibre／Node／浏览器／Rust／Amp 等可选依赖环境完成核心流程；另验缺 Java／EPUBCheck 的失败路径。平台实测、包体与来源分别记录，不引用旧树或上游性能作本版证据 |
| S2／S3／S4 | 后续参考 Readium 资源服务与 CSS，仍按冻结快照、受限本机访问、作者原样／偏好覆盖及真实 Mac 渲染验收；可选 Ace 自动报告、固定官方用例与人工／VoiceOver 分开，不自动认证，不以研究工具收窄 #3 |

**交付物与后续增量：** S0 上游研究索引已登记 repo、commit／release、取得日期与许可记录；继续补齐实际采用所需的依赖／工具 hash、机器协议、规则及行为证据。跨项目采用决策可另落 `docs/research/EPUB_CLI_ECOSYSTEM_REVIEW.md`，避免复制规范矩阵。对应 fixtures／验证记录在实施时新增，私有书籍和未授权字体不公开。研究库存不是采用或运行验证；当前仍有一个已披露的许可缺口，阻止相应后置 adapter 的采用。CLI 新机器 schema 尚未实现；可选 adapter 只有在 §8.3 的隔离副本、完整输出、来源及取消测试通过后才启用。

**Issue #4 落实门槛：** 五组角色和取舍可追溯，实际采用的版本／协议／许可及供应链完成核验；T1 的合法输入接纳与 repair 分开，T2 有真实提案／事务／诊断差异／恢复证据，T3／T4b／T5 具备相应对照反例；正式门槛、旧 schema 来源和无新增字体约束不退步；按 §11.6 执行受影响测试和实际 CLI／平台验收。后置或不采用项目明确记录理由，不以“未集成全部工具”阻塞 T6。只改计划不能勾选实现完成或关闭 issue；#4 落实也不等于 #3 的规范、渲染、无障碍目标完成。

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
| 提案绑定与定位 | 输入／旧值／工具／规则改变、序号重排、多个相同文本节点均不能误用批准；精确目标与提案 hash 重验 |
| 修复前后诊断 | A 消失而 B 新增且总数相同、fatal 减少仍有 error、位置平移／拆章、过滤参数变化、同 ID 不同引擎、blocked 后首次可查均有不同预期；不得抵消或伪称解决 |
| 依赖修复与多资源失败 | 第二／第三文件写失败、撤销被依赖修复、取消／崩溃及恢复再次中断，原子执行或按 journal 恢复；重算或整批回滚，历史来源可验证 |
| 未退出进程、过期plan、两个CLI写入 | busy/conflict，有恢复日志，无盲覆盖 |
| 外部工具退出0但输出异常 | 重新导入/检查失败，不能接受或正式导出 |
| 外部输出和限制 | 额外文件、字体替换／嵌入、系统绝对路径、ZIP 穿越、隐藏时间戳、写集合不符、试图联网、半输出／截断 JSON／缺报告均有拒绝或限制证据；恢复不盲跑新工具版本 |
| 无Calibre/Qt、无Amp、无Java | 按能力降级，unavailable与pass区分 |
| 无可选工具的终端闭环 | 无 Node／浏览器／Rust／Calibre／Amp 仍可完成原生编辑；正式检查需要 Java／固定 EPUBCheck，缺失明确阻止，不自动降级 |
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

v0.8 独立 CLI 范围参考：[Sigil 官方功能](https://sigil-ebook.com/sigil/)、[EPUB 3.3 内容文档要求](https://www.w3.org/TR/epub-33/)、[HTML XML 语法](https://html.spec.whatwg.org/multipage/xhtml.html)、[CSS Fonts 的字体族与回退](https://www.w3.org/TR/css-fonts-4/)、[Fontconfig 用户文档](https://www.freedesktop.org/software/fontconfig/fontconfig-user.html)。这些规范和参考实现用于冻结后续支持矩阵，不是当前 Kepub 已实现字体／完整 XHTML／CSS 的证据。

v0.9 依据 [Issue #3](https://github.com/LeviTK/Kepub/issues/3) 在 2026-10-05 的完整正文（无评论）完善当前路线；固定规范、配套资料与下载门槛见 §11.8。这里只确认计划追踪范围，没有完成规范原文包、能力矩阵实现、官方测试或 Mac 验收，也没有修改远端 issue 状态。

v0.10 依据 [Issue #4](https://github.com/LeviTK/Kepub/issues/4)（正文更新于 2026-10-05T20:25:57Z，无评论），其 Kepub 输入固定为 [v0.9 提交](https://github.com/LeviTK/Kepub/commit/a8920d3d69fb15ebcc1d33bb37c0a41ed7eb3bee)。本轮只读取 issue 并更新计划，不重做其部分上游文档／源码静态研究，也未运行上游工具、性能或平台测试。该研究记录 epubsana `docs/USAGE.md` blob `666d35625f8a7390ece1437c3e5c3265e3ebf19c` 与 raitucarp `editor.go` blob `d515d40c986d707309a69eecc61c9c131305dac5`，仅锁定所读文件，不等于锁定完整工具与依赖；其余移动入口及规范归档仍按 §8.3／§11.8／§11.9 补齐证据，不伪称已完成。
