# Kepub CLI 与操作契约

> 设计版本：0.6 · 日期：2026-10-06 · 状态：分阶段实现；对齐 Issue #3／#4 与开发方案 v0.10，补修复提案、诊断差异及可选适配设计，不改变已实现 schema。真实 Droid 计划审查已关闭已知问题，S0 与后续批次按固定输入和独立证据逐批验收，T1 启动须先通过 S0；实际可用命令以 README 与 `capabilities` 为准。
>
> 本文定义 Kepub 自己的命令和协议，不是 Calibre 或 Amp 的使用手册，也不表示命令已经能运行。架构见 [开发方案](DEVELOPMENT_PLAN.md)，设计依据见 [Calibre 研究](research/CALIBRE_CLI_REVIEW.md)。

## 1. 设计目标

CLI 是与 GUI 平级的产品入口，也是 Agent 的确定性工具层。不要求用户安装桌面端来查询或打包 EPUB；不要求运行模型来改一个已明确指定的字段。

一个子命令对应一个应用用例。GUI 不拼 shell 字符串复用 CLI；GUI/CLI 都调用 Go 服务。Agent 使用 CLI 时仍面对相同任务、校验与审核语义。

Obsidian CLI 的参考取舍见 [开发方案 §9.1／§9.2](DEVELOPMENT_PLAN.md#91-参考-obsidian-cli但保持真正-headless)：借鉴可发现能力、精确目标、结构化查询和差异，不引入桌面运行依赖、当前活动文件默认值或任意 eval。保留现有 `--option` 与 JSON 请求文件语法；Amp 经 shell 使用本 CLI 不需要 SDK 或 External API。反向运行 Amp 的接口边界见 [§8.1.3](DEVELOPMENT_PLAN.md#813-external-apicli-与-typescript-sdk-的适用边界)。此补充不改变现有 schema 或注册新命令。

## 2. 命令分组与实施次序

下表 M0～M6 是技术工作包编号，不再表示执行先后。当前先完成 [S0 规范资产与差距矩阵](DEVELOPMENT_PLAN.md#118-issue-3-阶段映射规范资产与关闭门槛)，再按 [开发方案 v0.10 §11.7](DEVELOPMENT_PLAN.md#117-v010-独立-cli-批次与完成标准) 完成独立终端制书：默认 EPUB3、EPUB2 → EPUB3 转换、完整 EPUB3 XHTML、样式和系统字体发现；本阶段不接入 UI、Amp 或外部编辑器。C1 发现／诊断、C2 有界内容读取／定位、C3 单个 XHTML 简单文本确定性修改已实现并作为源码发布到 main，尚无版本化安装包。C3 操作 schema 在 §2.2 冻结；支持边界见 [§11.4](DEVELOPMENT_PLAN.md#114-首个正文读写版本的边界)。T1a／T1b 已验收，源码已同步到 GitHub main，编码／内部子集／来源及 search／status／终端输出见 §2.4；§2.3、§2.5～§2.6 的其余能力仍待实施。CLI 完成不等于 Issue #3 的真实排版／无障碍目标完成。

| 命令形态 | 语义 | 阶段 |
|---|---|---|
| `kepub version --json` | 实际构建版本／Go／平台／可用 VCS 信息，不联网推测版本 | C1 本地已实现 |
| `kepub doctor --json` | 环境/架构/外部工具发现，不调用模型、不修改书籍 | C1 本地已实现 |
| `kepub capabilities --json` | 版本化能力和可用性；区别 planned/available/unavailable/unsupported | M1 |
| `kepub info BOOK --json` | 出版信息摘要，含规范/渲染能力警告 | M1 |
| `kepub toc BOOK --json` | 读取导航，不改变 spine | M1 |
| `kepub inspect BOOK --section manifest --json` | 指定范围的只读结构查询 | M1 |
| `kepub content --workspace DIR --resource BOOK_PATH [--query TEXT] [--limit N] --json` | 只读 accepted revision 的单个 manifest XHTML，返回有界文本／结构定位／资源哈希 | C2 本地已实现 |
| `kepub search --workspace DIR --query TEXT [--limit N] --json` | 按 manifest 顺序检索 accepted 的全部 XHTML，返回完整匹配元素数 | T1a 本地已验收 |
| `kepub unpack BOOK --output DIR` | 解到新目录，安全检查，不覆盖 | M1 |
| `kepub pack DIR --output OUT.epub` | 清单式归档及正式检查，不是convert | M1 |
| `kepub validate BOOK_OR_DIR --json` | 分层诊断及覆盖报告 | M1 |
| `kepub workspace open BOOK --output DIR --json` | 仅新建工作区，返回持久ID；已有路径不覆盖 | M2 已实现 |
| `kepub workspace list --json` | 全局工作区发现/注册表尚未实现 | planned |
| `kepub plan --workspace DIR --operations FILE --output PLAN.json` | 从当前accepted基线生成单字段操作计划，不写出版内容 | M2 已实现 |
| `kepub apply --workspace DIR --plan PLAN.json --json` | 核对计划，创建候选任务；不接受/不导出 | M2 已实现 |
| `kepub task diff TASK --workspace DIR --json` | 实际完整文件增删改及元数据／正文目标的old/new文本 | M2／C3 已实现 |
| `kepub task status TASK --workspace DIR --json` | 精确活动或已结算 taskId 的执行、检查尝试与决定；不产生批准 | T1a 本地已验收 |
| `kepub task accept TASK --workspace DIR [--strict --timeout SECONDS]` | 冻结候选，真实正式检查，显式接受新revision | M2 已实现 |
| `kepub task reject TASK --workspace DIR` | 保留审计，不删除原书或revision；随后可再编辑 | M2 已实现 |
| `kepub workspace export DIR --output OUT.epub [--draft --strict --timeout SECONDS]` | 仅从当前accepted导出，重新检查最终ZIP | M2 已实现 |
| `kepub preview --workspace ID --at 'EPUB/Text/ch01.xhtml#note1'` | 启动GUI预览并定位；不启动Amp | M3 |
| `kepub serve --workspace ID` | 受限本机只读预览服务 | M3 |
| `kepub amp --workspace ID` | 建候选后启动原生Amp TUI，要求TTY | M4 |
| `kepub task run --workspace ID --prompt-file FILE --jsonl` | 程序化Amp任务，结束后仍需审核 | M4 |

`inspect --section` 首批枚举 `metadata`、`manifest`、`spine`、`navigation`、`references`、`capabilities`。引用查询可加 `--resource BOOK_PATH` 和 `--direction incoming|outgoing`；查询不完整时在数据中返回 coverage，不能把空列表当全书无引用。

XHTML 的 href／src（含 EPUB3 nav）在解析目标前，仅去掉属性值两端的 ASCII 空白；引用边与导航结果仍保留 XML 解析后的原属性值，出版资源字节不变。不去掉 NBSP 等 Unicode 空白，不把百分号编码后的文件名空格当作分隔空白；此规则不全局应用于 BookPath、OPF、NCX、SVG 或 CSS。

`info` 和 `toc` 是同一读用例的便捷入口。暂不增加一串同义顶层命令；改名、metadata和未来polish由操作注册表表达。`convert`、任意Calibre透传、MCP、书库、邮件、批量删除不属于首批命令。

上述命令按阶段逐步实现，`capabilities` 不能把表中所有设计都提前报告为 available。

当前 M2 通过显式目录定位，不接收用户填的 workspaceId 代替实际状态，不查询全局注册表或“latest”。除 `workspace open` 新建外，各命令 Open 已有目录并验证持久身份、来源与锁。Plan 报告与 export 输出统一要求在整个 workspace 根之外、父目录已存在且为真实目录、输出此前不存在；不存在的新文件也不得放入 original、revision 或候选 pub。操作/计划输入拒绝符号链接、硬链接及特殊文件，打开使用非阻塞 regular-file 检查，FIFO 不等待 writer。

`workspace open` 的创建目标已存在时返回 exit 2／`OUTPUT_EXISTS`，包括已有文件、目录和符号链接；源文件被确认不是稳定的单链接普通文件时返回 exit 2／`INVALID_ARGUMENT`，不跟随链接或等待 FIFO。源／父目录缺失、权限失败等真实文件系统故障仍为 exit 6／`IO_ERROR`；不将任意内部文件存在错误全局映射成参数错误。

本文允许审阅／拒绝的候选漂移，限于安全、可完整散列的普通文件与真实目录树的字节或文件增删变化。符号链接（包括树内链接）、硬链接、特殊文件、非规范路径、大小写／Unicode 碰撞仍拒绝；文件不可读取也不能伪装成完整 diff。任务、来源记录或检查点损坏不是候选字节漂移。即使 accepted 本身未变，工作区 `content`／export 也须通过 Open 的完整核验，不提供绕过不安全 active 候选的入口。这沿用 [M2-B 的安全树边界](verification/M2_B.md#持久状态和提交恢复语义)，不是新增不安全条目审计／归档能力。

外部写者造成不安全候选后，应先停止写者、保留现场证据，仅人工移出或修复已确认由自己引入的候选条目，不跟随链接；恢复为安全、可完整散列的树后，再以原 taskId diff／reject。不得通过删除任务、used 记录、检查点或 journal 来强行解锁／重建来源。无法确认来源或记录已损坏时，应保留现场，不能承诺自动恢复。

工作区整体搬迁后，未执行的旧计划仍因绝对路径变化而 stale，不能经 Apply 或 WritePlanReport 重新授权。已开始任务及已接受历史按持久身份、已登记计划、检查点、版本和精确内容来源验证，不依赖旧主机路径。活动任务仍可 diff／reject，待审任务仍可经真实检查显式 accept；已接受历史可继续读取，但已结算任务不能重复接受或拒绝。已经发布 settlement journal 的恢复只完成原有持久决定，不产生新的批准。搬迁不会放宽漂移、损坏记录或检查器门槛。

有 `status:running` 的 edit-start、缺少 result 的中断执行恢复 checkpoint 并记录 failed，不重跑写入。只有 intent、尚未登记执行的中断任务从已验证 accepted 建立来源 checkpoint，并持久标记合成 start 为 `unstarted`，只允许生成 failed 结果；即使恢复再次中断，也保留外部候选差异供 diff／reject，不把漂移候选当作基线。failed 后的普通候选字节漂移也可审阅／拒绝，但 failed 任务永远不能接受；身份、计划、检查点或失败记录的来源／状态不符合约束时明确拒绝打开。

升级前已经写出的合成 `running` 与真实 `running` 记录无法可靠区分，旧记录继续按原回滚语义处理；不猜测迁移来源，也不能恢复旧版本已经丢失的外部差异。新标记不改变计划 schema 或旧摘要，但不能据此宣称旧二进制支持新状态。

尚未提交的执行恢复必须先核对任务身份、intent 和已消费计划绑定，再合成 checkpoint／start 或执行回滚；已有 start 的 taskId 也须在回滚前核对。损坏来源不能先改写候选或新增执行记录再报错。旧版 active 任务可没有消费记录；若记录存在，仍须核对，不能通过降低外层任务版本绕过它。

持久 JSON 记录先在同目录临时文件中完整写入并同步，再原子、不覆盖地发布。写入失败不会把半截 JSON 发布到正式记录名；故障解除后可重试恢复。Open 必须先校验并完成已发布的 settlement／restore journal，再清理保留的内部 staging，最后恢复中断执行；回滚复制在 journal 发布前再次中断，不应阻断下次重试。损坏 journal 保留证据并拒绝打开，不以清理绕过校验。这些边界已有短写和中断反例验证，不表示断电、任意磁盘故障或非协作外部写者均已覆盖。

已发布且通过自身来源与哈希检查的 restore journal 仍先恢复指定 checkpoint，即使后续 execution 校验因损坏的 used／start 记录而拒绝打开；这也覆盖 pub 暂缺的已提交中断状态。它只完成原持久决定，不生成新的执行结果、不重放操作，也不更改原书或 accepted。此规则不允许损坏 journal 自身或检查点时继续恢复。

如果 `apply` 已发布本次候选，随后启动失败且仍能核对任务、intent 与已消费计划，JSON 错误回复保留 `data.taskId`，供排除 I/O 故障后显式 diff／reject。回复仍为 `ok:false`，不是成功执行或接受授权；未发布候选时不生成虚假 taskId。此行为不提供其他计划的任务发现、latest 选择或丢失回复后的发现协议，也不会重放操作。

### 2.1 C1/C2 本批实施契约

本节记录本批接口约束。C1 与 C2 的读核心、app/CLI 已集成并作为源码发布到 main。C1 保留现有参数语法、envelope、退出码和编辑行为，在现有 app/CLI 中建立命令描述来源，用于帮助、参数校验和能力描述；不更换解析框架、不新增运行时。

- `kepub version --json` 返回构建版本、Go 版本、平台与可用的构建修订信息；没有发行版本时明确为开发构建，不运行 Git 或网络查询来猜测版本。
- `kepub doctor --json` 检查核心运行环境及 Java／固定 EPUBCheck 的就绪状态，单独报告可选 Amp 的发现状态，不验证登录、不调用模型、不安装依赖、不输出完整环境。诊断成功收集可返回 `ok:true`，依赖缺失体现在报告和正式检查能力中，不能冒充检查器就绪；执行故障和取消明确报告。对外部版本探测设置时间／输出上限并回收受管进程，复用现有 checker 完整性规则。
- 帮助支持总览和明确命令的说明，JSON 帮助仍是单 envelope；未知命令或选项不得因加 `--help` 就成为有效命令。capabilities 的已有数组形状和 operation ID 保留，新字段可增量添加；未实现能力仍为 planned。
- capabilities 增量 `commandSchemas` 描述实际 CLI 命令，与操作的 `inputSchema` 区分；例如 `metadata.set` 是计划文件内的操作，不是顶层命令。命令描述从已有能力来源生成帮助和允许／必填参数，不能将 planned 项当成可执行能力，也不宣称所有结果的细粒度 schema 已完成。

C2 首版命令为 `kepub content --workspace DIR --resource BOOK_PATH [--query TEXT] [--limit N] --json`，只读，不接受 BOOK、rootfile 覆盖或任务参数；注册为 `publication.content` v1 / `available` / `read_only`。publication 读核心经 app 的现有工作区快照接口接入 CLI：

- 工作区使用已有 `AcceptedSnapshot` 冻结当前 accepted；返回真实 workspaceId、revisionId、rootfile、精确 bookPath、resourceSha256 和 locatorVersion。查询期间保持现有协作锁，关闭快照和工作区；不从内部目录猜当前版本，不修改出版内容。
- 仅支持所选 publication manifest 声明的 `application/xhtml+xml`，路径必须为精确 BookPath，不是 href／fragment／本机路径。C2 首版采用 UTF-8 XML；T1a 已扩展严格 UTF-16 LE／BE 与原生 HTML DOCTYPE，原始 8 MiB、深度／token／索引限制不变，新增解码预算见 §2.4。不支持的资源类型明确失败，CSS 查询留到后续，不另建语法路径。
- 结果节点取 XHTML body 内的 XHTML 元素：叶元素，或有非空白直接文本的非叶元素；不包括 body 容器、head、script/style 子树和外来命名空间子树。含被排除子树的祖先元素也不作为结果，仍遍历其受支持子元素，避免经祖先 text 返回被排除内容；这不是全书全文索引。`text` 为 XML 解码后的后代文本原顺序，不 trim、不做 Unicode／空白归一化；同时返回元素 namespace/localName、可选 id、结构 locator 和 `hasChildElements`。混合内容仅供读取，不能由这个布尔值推导已允许编辑；locator 不是 XPath 执行器或可写偏移。
- query 缺省表示全部上述节点；显式 query 必须非空且不超过 4096 UTF-8 字节，以区分大小写的字面子串匹配解码后的 text，不支持正则，不将不同返回元素拼接搜索。一个混合元素自身的后代文本已按原顺序合并，因此可匹配跨内联标签的短语。每个匹配元素返回一次，不默认选择首个；重叠父子节点可分别返回且 locator 不同。
- limit 缺省 50，范围 1～200；按文档顺序返回。报告匹配元素总数、返回数量和 truncated；无命中是成功的空数组。返回文本累计上限 1 MiB，超限明确 `CONTENT_LIMIT`，不裁剪单节点文本或悄悄遗漏。所有上限校验在访问工作区前尽可能完成。
- 整本与资源原字节不变；同文多处、实体、非 BMP、BOM/CRLF、命名空间、活动候选存在但只读 accepted、并发 busy、超限与哈希独立核对均须测试。本批不开放正文写入，也不把定位信息当编辑授权。

### 2.2 C3 受限正文修改实施契约

2026-10-05 用户授权实施并随后授权源码发布；本节接口已冻结并集成至 main，`content.text.set` v1 为 available。验证记录见 [C3](verification/C3_CONTENT_EDIT.md)。复用 `plan → apply → task diff → task accept/reject → workspace export`，不新增直接写文件命令。

新增 `content.text.set` v1。请求使用 `schemaVersion:2`，恰好一个操作；params 必须完整提供 `bookPath`、`revisionId`、`resourceSha256`、`locatorVersion`、`locator`、`expectedOldValue`、`newValue`，不接受 null、重复／未知字段或字节偏移。这些字段的 JSON 顺序作为正文操作的规范编码顺序。前五项及旧文本取自同一次 `content` 返回，不能只按相似文本重找首个匹配。

- BookPath 必须精确匹配所选 manifest 的 `application/xhtml+xml`，不是 href／本机路径；revisionId 等于当前 accepted，resourceSha256 是原资源的 64 位小写十六进制 SHA-256。版本／资源哈希过期返回既有 exit 4／`INPUT_DRIFT`，不生成候选。
- locatorVersion 仅为 1；locator 非空、合法 UTF-8、不超过 4096 字节，按 C2 的结构 locator 精确匹配，不执行 XPath。只允许唯一直接 body 中、未处于 head/script/style/外来命名空间子树的 XHTML 元素；body 自身不可写。
- 目标必须是无子元素、注释、CDATA 或处理指令的简单文本，且有独立起止标签。显式空元素 `<p></p>` 可写；自闭合元素不写。混合内容即使可以查询也拒绝编辑，no-op 同样校验支持子集。
- 旧值按 XML 解码后的文本精确匹配，不 trim／Unicode 归一化。旧值和新值均不超过 1 MiB UTF-8 字节；允许空字符串，新值必须是合法 XML 文本。`<`、`&` 等只作为转义文本，不解释为 markup。目标外字节、属性、ID、href、BOM／换行、其他文件和 OPF 时间戳保持；no-op 完全不改字节。
- 对同一受锁基线重解析、计算字节范围并局部替换，替换后再次解析并核对目标新值；沿用 XML 8 MiB／深度／token／索引限制。locator 不直接转换成用户控制的写偏移。参数、定位／旧值不匹配或不支持目标在 plan 阶段明确拒绝，沿用 exit 2／`INVALID_OPERATIONS`，不自动降级。

**兼容和来源校验：** metadata 请求／plan schemaVersion 1、execution version 1、params 规范编码及现有 policy 摘要保持不变，包括支持旧 initial-only policy 的既有规则。新正文请求／plan schemaVersion 2、execution version 2，operationVersion 1；正文 policy 使用字符串 `kepub-content-text-v1:accepted-baseline;single-set;locator-v1;simple-text;no-timestamp;review-required;conformance-not-run` 的既有 digest 算法。v1 不能携带正文操作，v2 本批仅支持正文操作；版本／操作／policy／execution 的不匹配必须拒绝，不静默升级。workspace、revision、task、settlement 的现有外层格式不为本批重新编号。

Apply、Open 恢复和 Accept 都须重新推导相同的单资源写集合及精确结果哈希，不能只改分派入口。正文 review 增加可选 `content`，包含 bookPath、locatorVersion、locator、oldValue、plannedValue、实际候选 newValue（不可读取时为 null）及可选 unavailable；不把计划新值当实际候选值。既有 metadata review 字段含义保持。候选漂移仍可 diff／reject，不得接受或沿用旧检查；接受及正式导出仍运行真实固定 EPUBCheck，不自动接受、不自动草稿。

先完成保持 metadata 行为的必要重构并独立提交／回归，再加入正文能力。验收包括旧二进制生成的计划及待审任务、新旧版本组合拒绝、节点局部字节保留、陈旧读取绑定、写集合／执行记录篡改、恢复回滚，以及真实二进制正文接受和拒绝闭环。正文能力通过本批验收后才提升为 available；C4 模型运行和 GUI 不进入本批。编码 href 的报告兼容问题未在 C3 初次交付时修复，后续独立审查已解决其库存／附属行区分，当前规则见 §7.1；不改变正文操作 schema 或旧摘要。

### 2.3 独立 CLI 后续范围（计划，尚不可执行）

新建和新增结构／样式操作默认面向 EPUB3，制作目标对齐 EPUB 3.3、OPF `package@version` 为 `3.0`。保留现有 EPUB2 读取、检查和受限编辑；不增加完整 EPUB2 编辑语法、新建 EPUB2 或 EPUB3 降级。现有 open／validate／pack／export 不隐式转换版本。

- **EPUB2 → EPUB3：** 新增显式、版本化转换操作；维护元数据、唯一标识符、阅读顺序、NCX → nav、封面、必要 XHTML／CSS 和引用。T3 首批保留全部 CSS 原字节，需 CSS 变换则阻断；T5 复用共享解析器交付 CSS 迁移扩展及正反例，T6 验收，不另造解析器或用正则改样式。冻结转换策略、输入与新增 ID／时间戳；展示真实差异，经正式 EPUBCheck 才能接受及正式导出。无法安全保留语义时明确阻断，不只改版本号、不删原书。边界见开发方案 §3.7；命令名称与 schema 尚未冻结。
- **正文与书籍结构：** 完整 EPUB3 XHTML 解析／保留，以及混合内容、结构编辑、全书检索／替换、章节和资源维护、默认新建 EPUB3、常用元数据与历史回退。只读语法兼容不直接赋予写权限；输入片段由内置操作解析并绑定 hash，不提供外部编辑器往返或任意文件写入入口。
- **按标题拆章（T4b）：** 将所选 EPUB3 spine XHTML 中的多个章节按标题元素或明确章名规则拆成独立文件。先只读预览候选切点，允许明确排除／补选；计划冻结规则、locator、源 revision／hash、输出路径与完整写集合，apply 重算，不把原始 XML 正则切割当结构编辑。只在安全节点边界拆，标题留在新章，前言不丢，重复标题不覆盖；同步 manifest／spine／nav／保留的 NCX 和跨章 ID／脚注／资源引用。引用覆盖不足或结构歧义拒绝，样式影响显式报告；不隐式合章、导入 TXT 或放宽资源上限。全批走候选／diff／正式检查／accept 或 reject，EPUB2 先显式转换。详见[开发方案 §3.9](DEVELOPMENT_PLAN.md#39-网络小说按标题自动拆章t4b待实现)；操作 ID、schema 与预算尚待冻结，当前没有可执行拆章命令。
- **多操作事务：** 新请求／计划／执行格式另行版本化，贯穿写集合、恢复、接受与历史来源校验；不能仅移除“一个操作”限制。现有 `metadata.set` v1 与 `content.text.set` v1 的含义、旧计划摘要和安全门槛保持。
- **字体与样式：** 发现本机字体族／字重／样式，区分精确安装与系统替代；只写 CSS 字体栈，不嵌入、下载或安装字体，不泄漏本机字体路径。已有字体资源保留；缺字体不等于 EPUB 不合规，也不保证别的阅读设备能用该字体。增加结构化 CSS 操作与基础图片排列，高级排版／媒体后置。
- **标准字体混淆与旧资格：** T2 按开发方案 §3.5 增加仅限标准 IDPF 字体混淆的受控编辑资格，保持字体字节与密钥标识符不变。旧工作区必须显式版本化重评，合法 journal 先按原决定恢复，无活动任务后绑定旧原因／策略、revision／tree／声明 hash；不得删 `ReadOnlyReasons` 或历史证据。未知／非标准／混合加密和签名仍只读，T1～T6 不提供签名书编辑或重签；T3～T5 的保留验收含标准混淆正例，呈现由 S2 负责。
- **混淆资源引用：** 资格核验必须包含 encryption.xml 的 CipherReference，按 OCF URI 基准映射；T4a 改名须原子同步声明，否则拒绝，本阶段拒绝删除被混淆字体。签名引用不得忽略，但签名书仍只读，不通过改写／删除签名放行。字节／密钥未变不是路径改动安全的充分依据。
- **终端与平台：** 人类可读输出、文本 diff、任务状态及发行说明；JSON 仍保持机器契约。Linux 先验收，随后 Mac 打包／实机测试；不以 GUI、Amp 或外部编辑器作为前置。

以上通过实现与独立验收后才进入 capabilities 的可用项；已交付的 T1a 编码／终端增量见下一节，不扩大 §2.2 的简单文本写权限，也不使下文历史设计示例立即可执行。

### 2.4 T1 终端增量实施契约（已本地验收）

本节 T1a 入口、原生 HTML DOCTYPE／编码增量及 T1b 内部 DTD／实体／默认属性／来源均已集成到 main，分别通过父验收及 DeepSeek V4.1 Flash 的代码／demo 审计，源码已推送，未发布版本化安装包。证据见 [T1a 验证](verification/T1A_ENCODING_TERMINAL.md)与 [T1b 验证](verification/T1B_XML_SUBSET.md)。两批通过仅完成规定的 T1 范围，不代表完整编辑器或全产品复审。既有 JSON envelope、单资源 `content`、操作／计划／执行版本和正式检查门槛不变。

- `kepub search --workspace DIR --query TEXT [--limit N] --json`：持锁读取所选 rootfile 的 accepted revision，按 manifest 顺序检索其中所有 XHTML，不读取活动候选、不选另一个 rootfile。沿用 `content` 的区分大小写字面子串及节点／排除规则；命中数是匹配结果元素数，不是短语出现次数，不跨资源拼接匹配，也不是浏览器可见文本。
- query 必填，范围为 1～4096 UTF-8 字节；limit 默认 50、范围 1～200。返回工作区／revision／rootfile 身份、完整匹配数、返回数和 truncated；每个结果包含精确 bookPath、原资源 SHA-256、locatorVersion、locator 和解码文本。达到返回数限制仍须扫描剩余资源才能声称完整匹配数。不同资源出现同一 locator 不合并。
- 每资源 XML 8 MiB 统一按**原始资源字节、含 BOM**计量，读入和修改后的序列化结果均适用；每资源解码后的 UTF-8 流及实体展开后的 UTF-8 流分别最多 16 MiB，包括标记／属性／文本／保留声明，两份表示不相加、不按祖先重复计文本，也不跨资源累计。另将每次实体替换产生的字节累计到每资源 16 MiB 展开工作预算，限制中间开销；不把转码结果重新按 raw 8 MiB 截断。沿用元素深度 128／200,000 tokens／32 MiB 索引限制，展开生成内容也计数；T1b 的 DTD 声明最多 4096、实体嵌套最多 16、替换次数最多 100,000。全书 XHTML 原始扫描字节累计上限 128 MiB，返回文本累计上限 1 MiB UTF-8 字节。超限或任一应扫描资源不可读取／解析时明确失败，不把未扫描部分当零匹配，不静默跳过坏资源；truncated 仅表示返回条数限制。
- 32 MiB 索引独立保留既有计量：字符数据 UTF-8 字节先计两次，每向上一层祖先汇入子树文本再计一次，另加各元素 location 的 UTF-8 字节。单段 `html > body > p` 文本约计四倍，有效文本上限略低于 8 MiB，深层嵌套更早触限。原始／解码／展开流、索引等限额须同时满足；16 MiB 流上限不是正文可索引容量承诺，也不是实际驻留内存上限。
- `kepub task status TASK --workspace DIR --json`：查询精确任务的执行与结算状态，返回身份、基线及当前审核／接受／拒绝信息；未知任务明确失败（exit 4／`TASK_CONFLICT`），不以 latest 或其他任务替代。只读持锁，沿用 Open 的来源、恢复与安全树核验，不为状态查询绕过损坏记录。`matchesExecution` 只比较活动候选；历史任务当前返回 false 表示不适用，不能解释为历史损坏，历史仍验证决定／revision／计划消费。`checks` 列举检查尝试，随机文件名不构成时间排序或 latest。status 不产生批准或导出；`task diff` 仍只针对精确活动任务，已结算任务返回 exit 4／`TASK_CONFLICT`。
- 非 JSON 输出使用面向终端的可读摘要；任务 diff 按资源展示实际文本差异，不能把计划文本当候选事实。单资源 256 KiB、累计解码显示文本 1 MiB；二进制、不可解码或超显示预算的资源显示路径、变化类型与字节／哈希摘要并明确未展示文本，不伪称没有变化。控制字符／换行转义，显示不是可应用 patch；机器输出既有字段与含义不因终端排版而改变。

标准语法接纳不扩大 `content.text.set` v1 的结构权限。T1 编码支持须覆盖读取、定位、局部写入、来源重算与历史重开：未改资源保持原字节，已改资源保持原编码、BOM／声明和目标外字节。T1a 独立验收原生 HTML DOCTYPE／UTF-16 LE／BE 全链路、本节 search／status／diff、授权原书与公共合成样本，不宣称完整 T1。T1b 须完成开发方案 §3.4 的必需解析／保留矩阵：内部实体（含标记及参数实体）、ATTLIST 默认／固定属性及规范化、允许的声明与命名空间；预算内必需项不得仅标 unsupported。T1a 与 T1b 都通过才进入 T2。

**编码可解析不等于正式检查通过。** 固定 EPUBCheck 5.3.0 对 UTF-16 `application/xhtml+xml` 报 `HTM_058` error，对 UTF-16 OPF／container 报 `RSC-027` warning；其[编码分支](https://github.com/w3c/epubcheck/blob/029831b8f477e4519e9734c984ee24357547a698/src/main/java/com/adobe/epubcheck/xml/XMLParser.java#L129-L181)不按 EPUB2／3 区分。UTF-16 XHTML 的读取／局部编辑／来源重算／重开仍保真，但正式接受和正式导出必须保留这个失败、accepted 不推进且不生成正式产物；显式 `workspace export --draft` 保留当前 accepted 的原字节并标记未验证，不导出候选。不得隐式转码、过滤该诊断或降低 checker 门槛，也不把检查器拒绝等同于 XML 非良构。正式成功正控使用原生 UTF-8 XHTML，以及 UTF-16 LE／BE OPF／container 配合 UTF-8 XHTML；后者须用 `metadata.set` 修改 UTF-16 OPF，验证接受后历史来源和导出字节，`--strict` 仍阻止 warning。大段 UTF-16 CJK 注释跨解码 8 MiB 的样本验证解析／编辑／重开与正式拒绝，不伪称其通过正式检查。

T1a 验收 raw／decoded／扫描／返回限额，DTD 声明数、实体展开／工作预算及下述未展开实体语义由 T1b 验收，不以“本节全部限额”把两个批次合并。

EPUB3 依据固定 [§3.9](https://www.w3.org/TR/2026/REC-epub-33-20260113/#sec-xml-constraints)／附录 B 按 manifest 媒体类型检查 external identifier，接受允许的 NCX／SVG／MathML 元组但不读取任何外部 DTD 子集，包括离线 catalog；不接受 XHTML 外部标识符、内部子集中的外部 ENTITY 声明或 XInclude。PUBLIC 仅在匹配键中按 XML §4.2.2 折叠合法 space／CR／LF 并去首尾空白；SYSTEM 精确匹配，存储标识符、NOTATION 输出和原资源字节不改。内部子集按非验证 XML 语义处理，未知能力与非法输入分开；未声明实体不得静默从外部补齐。实体生成节点或默认属性无原始可写区间时明确拒绝直接局部写，不伪造 locator／offset，读取和原字节保留仍须正确。物理独立起止标签内的已知纯文本实体可按 §2.2 整段替换原始内部区间；no-op 保留实体引用，声明及目标外字节不改。生成边界、子元素／注释／CDATA／PI、未知内容仍不满足简单文本写条件。T3 显式 EPUB2 迁移模式才启用版本／hash／许可冻结的有限离线 DTD／实体；不读取书籍指定的任意 URL／本机路径，迁移结果和来源进入 diff。

XML 1.0 §4.1／§5.1 允许的非 standalone、外部子集／参数实体场景，可能存在良构但未取得声明的实体引用。T1b 必须记录未展开实体名、原字节位置和 partial 来源，保留原文，不以空串代替，不把后续可能被覆盖的声明／默认值当确定事实。使用独立能力诊断 `XML_ENTITY_UNRESOLVED`，禁止对依赖该未知内容的元素直接局部写；违反 Entity Declared WFC（如 standalone=yes）的输入仍报非良构。可表达 coverage 的读取／inspect 显式 partial；`content/search` 对应读取范围内的不完整文本明确 exit 3，不返回成功的残缺文本、完整匹配数或静默跳过资源。此实现是必需的可判定处理，不是允许必需解析项笼统报 unsupported。

附录 B 非法判定只用于已确定 package version 3.0 的 EPUB3 profile；EPUB2 非迁移路径的合法外部 DOCTYPE／未支持实体明确为能力不足（exit 3），不套 EPUB3 标准诊断为非法。真正非良构仍拒绝，不由 DOCTYPE 猜书籍版本。版本、资源类型、未展开实体与实际 checker 处理差异均入反例矩阵，不为匹配 checker 的外部子集行为而解除零外部读取策略。

限额与兼容验收包含 UTF-16 ASCII／CJK／代理对在原始 8 MiB 两侧，读取／写入／重算／历史一致。转码后跨 8 MiB 的成功样本用大段 CJK 注释与小段可编辑正文，确保索引未超限；LE／BE 各一份，均须验证读取／定位／局部编辑／来源重算／历史重开一致，不能仅验证解析。正文为主的 CJK 和不同嵌套深度另测索引超限拒绝，不以注释样本代表大正文容量。旧二进制不支持新增输入语法时允许安全拒绝，不能丢记录后“兼容”；旧输入及已发布摘要含义保持。语法错误／能力不足诊断变化在实现批次发布兼容说明，不冒称旧错误码含义未变。

#### T1b 的 XML 来源输出（已本地验收）

合法内部 `NOTATION` 的 SYSTEM／PUBLIC 标识符依据 XML §4.7 描述 notation，不是外部 ENTITY 声明或 DOCTYPE 外部子集。按 XML 语法解析并保留，向应用提供被引用的 notation 名称及标识符，绝不据此解析外部资源或启动程序。EPUB §3.9／附录 B 的 DOCTYPE 约束不扩大到所有 notation 标识符；内部子集中的外部 ENTITY 声明仍按 EPUB3 规则拒绝。此判定不豁免其他适用规范约束，也不将 parser 接受当作正式 checker 通过。

为不伪装不完整读取，T1b 在 `info` 与 `inspect` 的 `data` 内按需增量提供顶层字段 `xmlCoverage`（不是 envelope 顶层；inspect 中与 rootfile／section／value／limitations 并列）。navigation／references 的结果值本身另含同形字段，即 inspect 返回时位于 `data.value.xmlCoverage`；不替换旧 `value`／元素数组、不改变 envelope／操作／计划／执行 schema 或摘要。

```json
{
  "xmlCoverage": {
    "resources": [{
      "bookPath": "EPUB/chapter.xhtml",
      "resourceSha256": "原始资源的SHA-256，64个小写十六进制字符",
      "status": "partial",
      "unresolved": [{
        "name": "unknown",
        "kind": "general",
        "startByte": 100,
        "endByte": 109,
        "origin": "reference"
      }],
      "notations": []
    }]
  }
}
```

- `resources` 只列本次实际解析中存在未展开实体或需要报告 notation 的资源，按精确 BookPath 排序；与本次请求无关或未读取的资源不暗中全书扫描。没有此类信息时省略 `xmlCoverage`，不把省略解释为全书 XML 或 EPUB 合规。
- `status` 为 `complete | partial`，只指所选非验证 XML profile 的处理；被识别但未读取的实体、未知参数实体后的不确定声明／默认值均须传播 partial。未展开文本保留引用／来源，不用空串或猜测值冒充确定文本。结构必需信息（如 rootfile、package version、manifest 路径）未知时明确 exit 3／`XML_ENTITY_UNRESOLVED`，不猜结构，也不因缺失该未知值伪报 XML 非良构。
- `kind` 为 `general | parameter`；`startByte/endByte` 是含 BOM 的原始资源内 0-based 半开区间，不是解码流偏移。`origin:reference` 指实体引用自身的原字节；从实体展开间接产生、没有独立原始区间时，`origin:expansion` 指向导致该内容的原始引用区间，不冒充未知实体自身逐字节位置或可写区间。同一区间可关联多个展开来源，必须保留必要的不同实体记录；范围与资源 hash 绑定。
- `notations` 元素为 `{name, publicId, systemId}`，未提供的标识符用空字符串；至少包括 XML §4.7 所述在属性值、属性定义或实体声明中被引用且本次已解析声明的 notation，允许同时报告已解析但未引用的声明。未知声明不得伪造标识符；外部 ENTITY 禁令仍适用，不因它引用 notation 而放行。它们是声明数据，不是可跟随 URL 或读取授权。资源内列表按声明／引用处理顺序确定；重复 notation 采用首个已解析声明作为本实现的确定性非验证策略，不靠 map 随机顺序决定语义。唯一名称是有效性约束（VC），不是良构约束（WFC）；此流程不声明 DTD 有效。后续同名声明仍完整保留在原资源字节中，本批不另加重复声明输出字段。
- navigation／references 沿用已有 coverage、status 与 diagnostics。未知 XML 信息使结果至少为 partial；不足以建立导航／引用结构时仍可为 blocked，不把 blocked 降为 partial。两种状态独立：可表达的未展开 XML 来源仍报告 `xmlCoverage.status:partial`，纯业务结构阻断不把已完整解析的 XML 改为 partial；XML 非良构／限额等硬失败不为该资源生成 xmlCoverage 记录，而沿用 blocked／diagnostics 或相应命令失败，不能将失败伪装成 complete。不得从未知默认属性／实体文本生成肯定的引用或“无引用”结论。`content/search` 仍只返回完整文本，应读取范围不完整时 exit 3／`XML_ENTITY_UNRESOLVED`，不返回成功的残缺匹配数；不得靠 query 或 limit 跳过未知内容。依赖未知值的写入、实体生成节点或其他无独立原始可写区间的目标拒绝直接局部写。
- 导航结构诊断使用相关元素的局部确定性，不因整书 partial 一律隐藏错误。未知实体可能生成所需子节点／标签时，不断言其缺失；已知重复、非法子节点、非空白裸文本及已知缺失仍诊断。未知 href/src 的 target／exists 保持 null，已知链接不被未知标签抹掉；未知标签可保留字面引用，但 partial 结果不保证未知部分展开后的全书导航语义。

这些增量只表达本次 XML 来源与不确定性，不提前实现 §2.5 的全规范五维能力模型。验收须检查 UTF-16 原字节区间、直接／嵌套实体来源、NOTATION、缺失 PE 后默认属性、standalone=yes 非良构反例、无外部读取，以及旧无 DTD 输出的兼容性。

### 2.5 Issue #3 的能力证据设计（计划，尚未加入机器输出）

规范支持与命令可用性分开。S0 固定规范原文和条款后，建立单一机读矩阵，未来向后兼容扩展 capabilities／doctor／诊断，不替换当前 envelope、退出码、operation ID 或检查状态枚举。

离线资产矩阵与生成脚本已本地创建；实际修复、审查与放行结论见 [S0 独立验收](verification/S0_PARENT_ACCEPTANCE.md)。这不是新的 CLI 返回字段或可用能力，也不代表全部规范条款已经实现。首轮固定树 Droid 审查曾发现阻塞问题，其正常退出不构成批准。离线 `gate` 必须核验绑定同一输入的父验收与真实独立审计完整批准，以及资产、派生物、来源和执行证据；布尔字段或普通样本通过不能单独解锁 T1，也不提升五维能力声明。依用户最新指令，当前使用 Amp／DeepSeek V4.1 Flash，Droid 待用量恢复后使用；分别核验实际执行协议，不将 Amp 结果冒称 Droid 批准，完整范围及父验收条件不变。

- 逐特性记录 `featureId`、`specVersion`、`specSection`、`normativeLevel`、`applicability`、`preserve`、`parse`、`edit`、`render`、`validate`、`platform`、`testIds`、`evidence`；五维状态为 `supported | partial | unsupported | policy-disabled | not-tested`，不适用另给理由，不用 supported 代替。
- 按开发方案 §11.8 的清点单位覆盖完整 BCP 14 关键字集（含 NOT、REQUIRED／RECOMMENDED／OPTIONAL）、定义／语法约束、deprecated 与条件要求，对账来源片段及 hash；与产品安全／预算策略区分。保留未知语法不表示可编辑；CSS 解析不表示渲染；规范允许但产品暂不支持或策略禁用，不伪称标准禁止。
- 诊断区分非法输入、资源限额、能力不足与策略禁用，附规范条款、精确 BookPath／可得位置、处理阶段及 revision／snapshot。功能状态与单次检查结果分别提供；当前必需检查通过不等于五维全部 supported。
- EPUBCheck、引用 coverage、阅读系统测试、真实布局和人工 accessibility 分列；证据绑定规范／checker／工具哈希、输入和实际平台。未测、缺工具、超时不能通过，不能只据 `CSS.supports()` 或 WKWebView 选型提升能力。

字段容器、schema 和诊断 code 在对应实现批次另行冻结；上述是设计，不是当前 CLI 返回字段。规范资产、S0～S4 映射与关闭条件见开发方案 §7.4／§11.8；T1～T6 不承担后续 UI 渲染验收，也不能据此关闭 Issue #3。

### 2.6 Issue #4 的修复与诊断差异设计（计划，尚不可执行）

原生 Go 操作优先，沿 S0 → T1～T6 实施。T2 先完成多操作／多资源事务，再增加确定性修复操作、FixProposal 与 ValidationDelta；不是新任意写入口，不默认新增 fix／repair／polish 顶层命令。现有 operation／plan／execution schema、摘要和历史来源规则不因本节文字而升级，所需新版本在实施前冻结。

- **修复性质：** 合法 DOCTYPE／UTF-16 的接纳是修 Kepub，不是删合法原书声明。修复须有明确违反的规则和可确定操作，内容／样式编辑、优化与 EPUB2 → EPUB3 转换单独授权；缺作者信息不猜标题或空 alt。首批仅处理可安全建立上下文的书籍，严重损坏输入的修复导入工件后置，不弱化安全解包。
- **FixProposal：** 最低字段沿[开发方案 §7.2](DEVELOPMENT_PLAN.md#72-修复提案)，绑定规则／规范／提案版本、workspace／revision／tree／resource hash、适用 locator 与旧值、版本化操作／读写集合／固定 ID 与时间、风险／人工项、真实差异、提案 hash、验证计划及结果状态。序号不能独立作为批准身份；input／tool／rule／proposal 内容变化必须重提案／审阅，plan/apply 重新核验，不能选择第一个相似节点。
- **依赖撤销：** 撤销被依赖的修复后重算剩余操作、来源与报告，或回滚整个候选；不能删执行记录却留下依赖变更，也不能改已接受历史。
- **ValidationDelta：** 每次检查绑定 `checkerId/toolVersion/ruleset/specBaseline/profile/flags/inputHash/reportHash/runStatus/coverage`。用诊断身份多重集与可靠来源映射区分已解决、持续、新增／升级、新近可检查及不可比较／未完成；编辑前后 hash 可不同，但须分别绑定真实快照及受审阅的版本变化。行号平移／改名／拆章不能只比坐标，工具同码不视为同规则，范围缩小不视为问题解决。详情见[开发方案 §7.5](DEVELOPMENT_PLAN.md#75-validationdelta诊断身份来源映射与检查覆盖)。
- **正式门槛：** Delta 不批准接受；错误数量改善、可选快检通过或外部进程退出 0 均不能替代必需完整检查。继续固定 EPUBCheck 5.3.0 并核对目标 3.3 修订覆盖，不自动随上游切到 3.4；fatal／error、strict warning、缺工具、超时和报告绑定规则保持。自动无障碍与实际渲染分别给证据，不由 EPUBCheck 或 Ace 替代人工确认。
- **可选工具：** Readium／Go 库优先用于对照，epubsana／epubveri／Ace／Calibre 等须先核版本、协议、许可与供应链，再按 §9 逐能力开放；不使 Rust／Node／浏览器／Calibre 成为独立 CLI 必需依赖。不改变现有能力枚举，不把 planned 注册成 available；未来工具不可用状态须可区分且不得触发自动安装。

本节只是接口语义设计，不声明这些字段或修复规则当前可用。新增回归与阶段条件见[开发方案 §11.9](DEVELOPMENT_PLAN.md#119-issue-4-的阶段落点与落实条件)。S0 按 §11.8 的有限清单验收，负责 checker 2026 增量核对，未知已证规则基线使用 `specBaseline: unknown`，目标规范 URI 另记；计划复审无阻塞问题后按用户授权实施，不跳过 S0 或将 planned 当 available。

### 2.7 T2 首批多操作／多资源事务实施契约（已本地验收，父已放行；公开记录见 docs/verification/T2_MULTI_OPERATION.md）

本批按 [开发方案 §11.7](DEVELOPMENT_PLAN.md#117-v010-独立-cli-批次与完成标准) 先交付**版本化多操作／多资源事务底座**：把 plan／apply／checkpoint／restore／journal／diff／accept／历史来源重算从单操作单输出扩展为完整的冻结输入事务。它**不是完整 T2**：不新增操作类型、不开放混合内容／结构编辑、不做 OPF／nav／ID／链接依赖同步，FixProposal 与 ValidationDelta 仍为设计。旧 `metadata.set` v1（schema 1）与 `content.text.set` v1（schema 2）的含义、规范编码摘要与旧计划兼容规则不变；新增能力只使用新的版本号，不静默升级。

- **请求 schema 3：** `schemaVersion:3` 携带 2–256 个操作，每个为 `metadata.set` v1 或 `content.text.set` v1，参数形状与 v1／v2 相同。单操作请求继续使用 schema 1／2，不提供第三个等价编码；操作数超限、版本不符、未知操作或重复键明确失败，不截断或忽略。
- **冻结输入：** plan 仍绑定工作区身份、绝对路径、baseRevision、完整输入树 hash、rootfile、操作集摘要与策略摘要（`kepub-multi-v1:accepted-baseline;multi-operation;multi-resource;frozen-baseline;simple-text;no-timestamp;review-required;conformance-not-run` 的既有摘要算法）。每个 `content.text.set` 的 `revisionId`／`resourceSha256` 绑定冻结基线；旧值始终对照冻结基线校验，**前一个操作的输出不会放松或改写后一个操作的旧值期望**。
- **目标唯一：** 同一 `bookPath` 的 locator 必须互不相同；metadata 目标按冻结元素的结构 location 判重，因此显式 `id` 与省略 `id` 命中同一元素也算重复。重复／别名目标在 plan 阶段拒绝，不会被执行两次或按序叠加。
- **完整写集合：** 写集合为所有实际字节变化资源的排序去重列表，覆盖全部操作与资源；no-op 操作不贡献条目，全 no-op 事务写集合为空且仍产生完整待审记录。计划文件的 writeSet 只是派生结果，apply／执行恢复／接受／历史结算都从冻结 checkpoint 重新推导并要求完全一致。
- **执行与失败：** 按写集合顺序逐资源以同目录临时文件＋原子改名替换；完成后核对实际变更路径与写集合完全一致，并逐个核对目标类型、大小与 SHA-256。任一资源失败、记录发布失败或进程中断都会回滚整个候选（checkpoint／restore journal），不留下半本书；已发布的 restore journal 先完成其原决定。失败执行可 diff／reject，永远不能 accept；候选漂移可 diff／reject，不能沿用执行状态接受。
- **审核与历史：** v3 的 `task diff` 在既有 `diff`（实际候选树）之外增量返回 `operations` 数组，按操作顺序给出计划目标与**实际候选值**（不可读取时为 `unavailable`，不用计划值冒充）。accept 对冻结树运行真实固定 EPUBCheck，settlement／revision／历史状态重新推导 v3 完整写集合与预期差异；未改资源字节、原书 hash、失败无正式产物与输出不覆盖规则不变。
- **限制：** 请求文件仍为 32 MiB 上限；单操作解析沿用每资源 XML 8 MiB 与既有 token／深度／索引预算，不因多操作放宽。

本批不新增 CLI 命令；`plan`／`apply` 的输入语法不变，只有请求文件内容使用 schema 3。首批已通过独立 high 审查、父本地 probe 与放行（固定提交与公开验证记录见 [T2_MULTI_OPERATION.md](verification/T2_MULTI_OPERATION.md)）；剩余 T2（结构编辑的后续范围、依赖同步、修复提案与诊断差异、字体混淆资格）不因首批放行而完成。

### 2.8 T2 第二批 XHTML 结构编辑实施契约（已通过 medium 独立审查与父验收并本地集成；公开记录见 docs/verification/T2_MULTI_OPERATION.md）

本批在 schema 3 事务底座上增加**版本化 XHTML 结构编辑**：混合内容与属性写入、元素插入／替换／删除／同资源移动，以及 ID／链接／资源依赖门禁。它**不是完整 T2**：FixProposal、ValidationDelta、批量替换、字体混淆资格与跨资源移动仍属剩余范围，`content.text.set` v1 的含义与 schema 1／2／3 编码不变。旧二进制遇到 schema 4 请求允许安全拒绝，不丢记录、不改已发布摘要含义。

- **请求 schema 4：** `schemaVersion:4` 携带 1–256 个 v1 操作，可为 `metadata.set`、`content.text.set` 与六个新操作 `xhtml.attribute.set`／`xhtml.attribute.remove`／`xhtml.element.insert`／`xhtml.element.replace`／`xhtml.element.delete`／`xhtml.element.move`；至少一个 `xhtml.*` 操作才使用 schema 4，单操作旧请求继续使用 schema 1／2／3，不静默升级。策略摘要为 `kepub-xhtml-structure-v1:accepted-baseline;multi-operation;frozen-baseline;locator-v1;reference-gate;no-timestamp;review-required;conformance-not-run`。
- **冻结绑定：** 每个结构操作绑定 `bookPath`、`revisionId`、`resourceSha256` 与 `locatorVersion:1` 的精确结构 locator；全部字节编辑都对照**冻结基线**计算，前一个操作的输出不放松后一个操作的旧值期望或目标位置。错误码对应固定：`resourceSha256` 与冻结资源不符（漂移）拒绝为 `INPUT_DRIFT`（exit 4），`expectedOldValue` 不匹配拒绝为 `INVALID_OPERATIONS`（exit 2），`revisionId` 不匹配拒绝为陈旧计划（`INPUT_DRIFT`）。
- **编辑模型：** 每个操作对冻结字节产生「区间替换」或「插入点」；同一资源的区间必须两两不相交，插入点不得落在任何替换区间内或与其边界重合，插入点之间不得重合（同锚点同位置只允许一次插入）。目标元素的物理标签／内容区间缺失（实体生成或合成默认属性）时拒绝，不伪造 byte offset。`content.text.set` 在同一模型下作为元素内容区间替换，语义不变。
- **目标与片段规则：** 结构编辑目标必须是选定 manifest 中 `application/xhtml+xml` 的条目，沿用单操作 v1／v2／v3 的权限边界，未登记资源不被 schema 4 放宽。属性写入限定 XHTML 元素，拒绝 `style`、`on*`、`srcset`／`imagesrcset`、`http-equiv`、`srcdoc`、`data`、`action`／`formaction`、`poster`／`background`／`cite`／`usemap` 等本批无法维护的 URL 属性、命名空间声明与 `xml:base`；元素删除／替换／移动拒绝 `html`／`head`／`body` 与文档根，子插入拒绝 `html`／`head`。片段只接受 XHTML 命名空间元素，在**插入点的命名空间上下文**中解析校验后按作者原始字节插入（按目标资源编码），拒绝 `script`／`style`／`base` 及 `iframe`／`frame`／`frameset`／`object`／`embed`／`applet`／`portal`／`link`／`meta` 等嵌套浏览上下文、插件与仅限 head 的元素、事件属性、注释／CDATA／声明与处理指令；片段内的 id／`xml:id` 必须是合法 XML 名称。外来语法与 ruby／表格／脚注／方向／混合内容只在未被目标区间覆盖时原字节保留。
- **依赖门禁（全事务最终状态）：** 门禁在收集完整事务的全部身份与链接事实后统一校验，不按资源分组逐段放行，因此操作顺序不改变接受与否或写集合。移除或改名 id／`xml:id` 时，已知入站引用（nav／NCX／XHTML 链接、`headers`／`for`／`list`／`form`／`itemref` 与 ARIA IDREF 属性）存在即拒绝（`REFERENCE_CONFLICT`，exit 1）；引用提取覆盖不足以证明无引用时同样拒绝（`REFERENCE_COVERAGE_INCOMPLETE`，exit 1），IDREF 提取按 `xhtml.idref` 独立报告覆盖。新增 id 必须是合法 NCName、在冻结资源内唯一；新增 `href`／`src` 拒绝 `javascript:`／`data:`，内部目标必须存在于冻结清单，fragment 必须唯一可解析（目标资源若在同事务被编辑，则按该事务最终状态判定）。同一事务内先添加的 id 可被后续链接引用，指向同事务被删除／改名的旧值的链接在两个方向都被拒绝。**新增 IDREF 事实同样校验：** 属性写入与片段携带的 `headers`／`for`／`list`／`form`／`itemref` 及 ARIA IDREF 值（含多 token 列表）按同一 IDREF 词表采集，并逐 token 对照该资源事务最终身份状态，目标必须存在且唯一，否则拒绝；不存在、同事务被删除或在事务内新增成功都按最终状态判定，操作顺序不改变结果。
- **验证：** 拼接结果重新解析（profile＋complete）并与独立的树域模拟逐节点比较（名称、有序属性、直接文本、子节点数）；**每个**插入／替换／移动块都按其计划位置核对精确字节：位置由冻结编辑事实**独立重算**（对每个早于该点的区间／插入点累加尺寸变化），替换块不再使用全输出 `Contains` 或降级规则，因此混合文本中的相对位置、gap 插入与别处相同上下文都不能掩盖错位；点插入另与父元素标签或锚点相邻核对。遗漏任何块即拒绝。模拟与字节不一致时拒绝写入。移动限同资源，跨资源移动与资源级依赖同步仍属剩余范围。
- **审核增量：** schema 4 的 `task diff` 在 `operations` 数组中为属性操作给出 `attribute`（计划值与**实际候选值**，缺失即 `null`，不可观测时 `unavailable`），为元素操作给出 `element`（动作、锚点、位置与候选观测）；`content.text.set`／属性目标按**计划树路径**在候选中定位，避免同事务插入／删除造成的 locator 位移被误读为旧值或缺失。文件级 `diff` 仍是权威视图。
- **限制：** 单资源 8 MiB XML、片段 8 MiB、属性值 1 MiB、请求文件 32 MiB 与既有 token／深度预算不变；一次事务内同锚点同位置只允许一次插入，替换区间边界不允许共享偏移的插入（可用 `xhtml.element.replace` 表达替换语义）。

本批不新增 CLI 命令；`plan`／`apply`／`task diff` 的输入语法不变，只有请求文件内容使用 schema 4 与新操作 ID。本批已通过独立审查与父验收并本地集成（固定提交 79a4548，公开记录见 [T2_MULTI_OPERATION.md](verification/T2_MULTI_OPERATION.md)）；剩余 T2 仍不得标记为完成。

### 2.9 T2 第三批显式范围字面／正则批量文本替换实施契约（本批实现中）

本批在 schema 3／4 事务底座上增加 `content.text.replace` v1：在一个**显式 locator 范围**内按字面或正则把**全部**命中替换为新文本。它**不是完整 T2**：FixProposal、ValidationDelta、字体混淆资格与跨资源移动仍属剩余范围；`content.text.set` v1 与 schema 1～4 的含义不变，旧二进制遇到 schema 5 允许安全拒绝。

- **请求 schema 5：** `schemaVersion:5` 携带 1–256 个 v1 操作，其中至少一个为 `content.text.replace` v1，可同时携带既有 `metadata.set`／`content.text.set`／`xhtml.*` v1 操作；不含 `content.text.replace` 的请求继续使用 schema 1～4，不静默升级。策略摘要为 `kepub-content-text-replace-v1:accepted-baseline;multi-operation;frozen-baseline;locator-v1;explicit-hits;no-timestamp;review-required;conformance-not-run`。
- **操作参数：** `bookPath`、`revisionId`、`resourceSha256`、`locatorVersion:1`、`locator`、`mode`（`literal`／`regex`）、`pattern`、`replacement`、`expectedHits`。绑定与错误码沿用：资源漂移 `INPUT_DRIFT`（exit 4）、旧值／命中数不符 `INVALID_OPERATIONS`（exit 2）、陈旧 revision `INPUT_DRIFT`。
- **范围（显式）：** locator 指向的目标元素必须属于选定 manifest 的 `application/xhtml+xml` 资源，且位于 body 内、非 `html`／`head`／`script`／`style`、非外来命名空间。搜索范围为**该元素子树内每个 XHTML 命名空间元素自身的直接字符数据**；外来命名空间子树与 `head`／`script`／`style` 子树不搜索也不写入，计划事实报告 skipped 元素计数。文本按元素自身直接字符数据匹配，**不合并后代文本**，因此命中不跨元素边界；同一元素内跨实体引用、CDATA 等不可写片段的命中整体拒绝，不静默跳过。
- **匹配语义：** `literal` 为区分大小写的精确子串；`regex` 为 Go RE2（无回溯引用／环视），逐元素直接文本求左起非重叠命中。`pattern` 非空、≤64 KiB、合法 UTF-8；能匹配空串的模式（含空字面量）在 plan 阶段拒绝；`replacement` ≤1 MiB 且必须是合法 XML 文本；`regex` 替换支持 `$name`／`${name}` 展开（字面 `$` 写作 `$$`），展开结果按文本转义写入。
- **命中数与写集合：** `expectedHits` 为 0–10000 的精确期望；实际命中数不等即拒绝（`INVALID_OPERATIONS`），无部分写。每处命中必须是同一可写字面区间内的物理区间替换，区间由冻结字节独立重算；不可写（实体生成、CDATA、未知来源）即拒绝。`expectedHits:0` 且无命中为 no-op，不贡献写集合。实体来源、UTF-8／UTF-16 编码、未改字节与失败无正式产物规则不变；单资源 8 MiB、每处替换与受影响元素直接文本 1 MiB 预算沿用。
- **全链：** plan／apply／checkpoint／restore／journal／diff／accept／历史来源重算都从冻结基线重算规则、命中与完整写集合；`task diff` 的 `operations` 为每个替换操作给出 `replace`（`mode`、`expectedHits`、实际 `hits`、每个受影响元素的 locator 与**实际候选直接文本**，不可观测时 `unavailable`）。accept 仍以固定 EPUBCheck 与冻结树为准。
- **验证：** 不对称正反例（跨元素边界不命中、同元素跨内联子元素命中拒绝、空匹配拒绝、命中数不符拒绝、实体／CDATA 拒绝、UTF-16 与 CRLF 保真、regex 捕获展开）、有效事务独立字节 oracle、live fuzz 与真实 checker 闭环；不扩大 `content.text.set` v1 权限。

## 3. 目标选择与全局约定

### 3.1 明确目标

只读命令接受一个 BOOK/目录，或者 `--workspace ID` 绑定accepted revision，或 `--task ID` 读取稳定候选代际；三者互斥。`--revision REV` 只能与对应工作区联合使用。

对目录、文件、工作区的格式检测与能力限制来自相同核心，不根据扩展名自动启用转换。多rootfile要求 `--rootfile BOOK_PATH`；出现歧义返回参数错误，不能选第一个后继续写。

任务读取包含实际generation/输入hash；当前候选不稳定时返回上一冻结视图的标识和stale状态，或按请求拒绝读取，不伪装最新。

### 3.2 输出和交互

- `--json`：stdout 恰好一个 UTF-8 JSON 对象；包括失败情况。日志/进度去stderr。作为其他选项的值时不启用 JSON 模式；解析失败后的模式识别也按相同值边界处理。
- `--jsonl`：仅长任务支持，逐行完整事件，最后一个terminal事件；与 `--json` 互斥。
- `--output/-o`：仅产物命令使用，值为显式输出位置；Kepub 自己保持一致，不能直接映射到 Calibre `-o`。
- `--strict`：正式检查门槛提高至warning；不意味着全世界阅读器一致。
- `--timeout`：正数秒，作用于当前请求及受管子进程；超时回收进程组并保留失败记录。
- `--`：未被前一选项作为值消费时结束选项，用于以短横线开头的路径。调用外部工具时先转为合法绝对路径并用参数数组。
- `--no-input`：禁止交互，缺少目标、授权或必要选择时返回明确错误。JSON模式默认不做终端问答。

不提供可以跳过所有安全、版本和校验门槛的全局 `--force`。草稿导出是显式模式 `--draft`，输出和报告均标记未获正式验证；不能因缺Java而自动切草稿。

serve默认在stderr显示本机访问说明，结构化使用场景用受控事件接口；不要把会话令牌写到公开诊断包。原生 `amp` 不支持混合TUI和JSON stdout。

## 4. OperationRegistry

### 4.1 注册条目

每项操作至少包含以下字段：

| 字段 | 含义 |
|---|---|
| operationId / operationVersion | 操作语义及独立版本 |
| implementationStatus | planned / available / unavailable / unsupported |
| reason | 不可用原因，不猜测已安装工具能力 |
| inputSchema / outputSchema | 严格参数与结果约束 |
| mutatesPublication | 是否写出版内容 |
| risk | read_only / bounded_edit / structural / lossy / external |
| requiresModel / requiresNetwork / requiresGUI | 执行依赖，默认不擅自开启 |
| supportedFeatures | EPUB版本、语法、资源与布局子集 |
| preconditions | 输入版本、哈希、覆盖、权限等 |
| postChecks | 执行后必须进行的检查 |
| idempotency | 重试与无变化行为 |

阶段状态与运行时可用性要有明确表达，例如实现尚不存在是planned，代码存在但工具缺失是unavailable。不能将文档中的 planned 状态生成可执行工具调用。

### 4.2 首批操作

| operationId | 输入边界 | 行为 |
|---|---|---|
| `publication.inspect` | 已选择的publication和section | 只读结构与coverage |
| `references.inspect` | BookPath与方向 | 返回引用边及未知/阻断范围 |
| `metadata.set` | 明确且唯一命中的metadata元素、预期旧值、新值 | 保留无关字段/namespace/refinement；不能唯一选择则拒绝 |
| `content.text.set` | 同次读取的BookPath、revision、资源hash、locator及旧／新文本；schema 2 | 仅简单正文元素局部替换；实际diff、正式检查和人工决定仍分开 |
| `resource.rename` | 一个现有非OPF资源、未占用目标BookPath | 更新已证明覆盖的入站/出站引用，不修改内容语义 |

初版 `resource.rename` 不支持循环交换、批量图改名、OPF改名、多rootfile重构、未知引用。只变大小写的改名必须有专门的平台安全流程和fixture，通过前显示unsupported。

`metadata.set` 首期不改变package unique identifier、语言体系、复杂关联或升级EPUB版本。无变化返回 `changed:false`，不借机重新格式化整份OPF或更新时间。

第二轮实现范围进一步限定为：每个计划恰好一个 `metadata.set` v1，仅现有 `dc:title` 或 `dc:creator` 的简单文本。按 namespace、local name 和可选明确 ID 选择，预期旧值必须匹配且目标必须唯一；不能通过选择第一个同名字段消除歧义。局部替换保留目标外全部 OPF 字节，复杂子内容或无法确定字节区间时拒绝。标识符、语言、批量操作和 `resource.rename` 仍不开放；这不是把未来操作契约缩减为永远只支持一个字段。

C3 已增加 §2.2 的受限正文操作，`resource.rename` 仍为 planned。正文操作贯穿版本化分派、计划重算、持久执行来源及接受／恢复验证，保留元数据 v1 语义；不接收任意 JSON 操作或整文件覆盖请求。C2 的只读定位信息绑定 revision／资源 hash，不能直接作为可信字节偏移执行写入。

可执行请求的字段和 Go/CLI API 见 [M2-B 验证记录](verification/M2_B.md)及 [C3 正文修改记录](verification/C3_CONTENT_EDIT.md)。以下 rename/impact 示例仍是未来设计，不能作为当前可执行计划输入。

未来操作如 `css.prune`、`image.optimize`、`font.subset`、`toc.rebuild`、`publication.convert` 分别设计参数与风险，不通过一个任意字符串的 `run_command` 逃离注册表。

## 5. 声明式操作请求

示例是设计契约，不能直接执行。请求文件只描述操作，不含workspace凭证或任意可执行代码：

```json
{
  "schemaVersion": 1,
  "operations": [
    {
      "operationId": "resource.rename",
      "operationVersion": 1,
      "params": {
        "from": "EPUB/Text/ch01.xhtml",
        "to": "EPUB/Chapters/ch01.xhtml"
      }
    }
  ]
}
```

未知字段、未知操作版本、不合法路径、空操作集、超出步骤上限都必须失败。批量步骤不是任意循环/条件脚本；每一步只能调用注册的确定性操作。未来带外部资源的输入通过应用登记为输入artifact并绑定hash，不让书籍内容提供任意本机读取路径。

## 6. 计划契约

### 6.1 plan不改书

plan从指定accepted revision读取，完成参数、能力和引用覆盖检查，计算修改影响与风险，写出计划文件。文件报告本身是允许的副作用，出版内容、accepted pointer与原始EPUB不得改变。

计划示例中的HASH/ID是说明用占位值；真实实现必须输出合法ID和完整摘要：

```json
{
  "schemaVersion": 1,
  "planId": "PLAN_ID",
  "workspaceId": "WORKSPACE_ID",
  "baseRevision": "REVISION_ID",
  "inputTreeSha256": "SOURCE_TREE_HASH",
  "operationSetSha256": "OPERATIONS_HASH",
  "policySha256": "POLICY_HASH",
  "operations": [
    {
      "operationId": "resource.rename",
      "operationVersion": 1,
      "params": {
        "from": "EPUB/Text/ch01.xhtml",
        "to": "EPUB/Chapters/ch01.xhtml"
      }
    }
  ],
  "impact": {
    "exactWriteSet": false,
    "changedCandidates": [
      "EPUB/package.opf",
      "EPUB/nav.xhtml",
      "EPUB/Text/ch01.xhtml",
      "EPUB/Chapters/ch01.xhtml"
    ],
    "referenceCoverage": "partial"
  },
  "applicable": false,
  "blockers": [
    {
      "code": "REFERENCE_COVERAGE_INCOMPLETE",
      "message": "尚未证明所有受影响引用均已覆盖"
    }
  ]
}
```

示例故意为被阻断计划，说明“生成了文件”不等于允许执行。partial coverage下的潜在文件列表不能伪装成完整write set；可执行结构计划必须完成对应支持范围的必要覆盖检查。

### 6.2 摘要规则

`inputTreeSha256`来自受控publication清单中的精确BookPath、文件类型、大小与内容hash，按明确排序和编码方式组合；不能取目录mtime。operation/policy摘要用已冻结的规范化JSON算法，字段顺序不应改变语义hash。计划自身hash不包含自指hash字段。

计划文件内容不可信：apply重新验证schema、操作版本、路径、预期输入与授权，从实际输入重算影响；不能相信用户或Agent写入的 `applicable:true` 或任意writeSet。

### 6.3 apply只进入候选

执行顺序：

```text
取得工作区写租约
 → 校验baseRevision/输入树/配置/实现版本
 → 重算计划与影响
 → 创建task及独立候选
 → 操作前checkpoint
 → 执行操作并验证真实writeSet
 → 快检/完整检查
 → 冻结结果供审核
```

计划读取后输入变化返回STALE_PLAN；越界改动、未知语法、阶段失败均不接受。必须验证创建/删除/改名与内容写入的完整文件集合，不仅检查被写文件的后缀。

当前 M2 的 apply 成功时返回显式 taskId、`status:"review_required"`、`reviewRequired:true`、`conformance:"not_run"`，包括 no-op；发布后启动失败的错误回复见 §2，正式检查在 accept/export 执行。计划登记并绑定绝对 workspacePath、持久ID、baseRevision、rootfile、精确Tree及操作/策略摘要，apply重算写集；已消费计划不能重跑，基线推进使旧计划返回 `INPUT_DRIFT`（退出4），已有/已结算/不匹配任务返回 `TASK_CONFLICT`（退出4）。候选被外部改动但仍是 §2 定义的安全、可完整散列普通文件树时，可重开 diff/reject，`matchesExecution:false`，不能继承原执行或检查状态；不安全条目或损坏来源仍拒绝打开。`matchesExecution:true` 仅表示实际候选符合已核验执行记录；已精确回滚的 failed 任务也可为 true，仍不能接受。

accept 不接受 passed 布尔值、检查替换回调或 draft 模式。它检查独立冻结树，要求 archive、parse.structure、固定 EPUBCheck 正式检查完整通过且无 error/fatal，再验证实际字节和来源。检查尝试记录为 checks_passed/checks_failed；通过检查本身不等于已接受。最终取消检查位于全部昂贵重算和日志准备之后、持久 settlement journal 发布紧前；journal 发布是不可撤回的接受意图，之后重开始终 roll-forward。随后 accepted 指针原子替换安装新可见基线，任务和检查点归档保留。无 journal 的预备快照不接受。初始导入不标为 pass；export 仅选当前accepted（初始为未验证的 initial），重新检查最终ZIP，显式草稿标记未获正式验证。No-op 接受仍建立独立审计revision，但内容树和字节不变。

### 6.4 与Amp候选的衔接

v0.2首期的plan以accepted revision为基线，不直接对运行中Amp候选作结构变更。需要接着处理Amp结果时：先结束任务、审核并接受内容结果，再对新revision生成结构计划；不能把旧revision计划偷偷套到新候选上。

同一候选内暂停Agent、获取写入租约、应用操作再继续对话，属于后续受控工具交接协议。没有该协议前，不能仅凭“模型说已暂停”判断没有子进程在写。

## 7. 机器结果和退出码

### 7.1 统一envelope

```json
{
  "schemaVersion": 1,
  "ok": false,
  "command": "validate",
  "requestId": "REQUEST_ID",
  "data": {
    "status": "incomplete",
    "inputTreeSha256": "SOURCE_TREE_HASH",
    "checks": [
      {"id": "parse.xml", "status": "failed", "source": "kepub"},
      {"id": "references", "status": "blocked", "blockedBy": ["parse.xml"], "source": "kepub"},
      {"id": "epubcheck", "status": "unavailable", "source": "epubcheck"}
    ],
    "diagnostics": [
      {
        "source": "kepub",
        "code": "XML_NOT_WELL_FORMED",
        "severity": "error",
        "bookPath": "EPUB/Text/ch01.xhtml",
        "line": null,
        "column": null,
        "message": "XML解析失败"
      }
    ]
  },
  "error": {"code": "DEPENDENCY_UNAVAILABLE", "message": "必要的完整检查器不可用"}
}
```

`ok`表示请求是否达到其契约，不直接等于出版物合规。`inspect`成功返回问题列表仍可ok=true；`validate`仅在请求的必需检查完整且门槛通过时ok=true。`task run/apply`执行成功不代表accepted，输出必须带 `reviewRequired` 和校验状态。

必需检查必须由用例的版本化政策指定，并在报告中区分。无结构改写的 `pack` 和本轮局部 metadata 编辑，需要安全容器/可解析结构以及正式产物的 EPUBCheck；不要求 Kepub 自有引用提取器覆盖全部 CSS 语法。引用 partial/blocked、未运行项及已发现诊断仍必须保留，不能改标 complete 或藏起具体 error/fatal。`resource.rename` 的覆盖门槛不同，不能以 EPUBCheck 通过代替受影响引用的充分覆盖。总体 `pass` 仅表示已声明的必需检查通过，不表示渲染、可访问性人工审核和所有自有分析均完整。

固定 EPUBCheck 5.3.0 的成功报告必须逐项绑定冻结 ZIP 的完整文件库存：原始文件名精确匹配，明确的 uncompressedSize 和字符串 checkSum 与实际资源一致，每个文件恰好一项。报告可能另含 OPF／内容检查生成的附属元数据行，只有 uncompressedSize／compressedSize 明确为 0、checkSum／compressionMethod 明确为 null，且名称与冻结原名派生的 NFC 明文或规范 URI 编码表示唯一关联时才允许；字段缺失不等价于 null。附属行不计入完整库存，不能弥补缺失文件或错误大小／哈希；原名重复、未知或歧义关联均拒绝。不对完整库存做 Unicode／百分号归一化，不以多次解码或优先猜测消除歧义。原始 BookPath 的安全与长度上限保持；由它派生的 URI 报告表示不是磁盘路径，不再套用原始路径的字节长度上限。正式成功退出、诊断计数、工具固定版本和归档／工具前后哈希门槛均保持。

检查报告可同时有内容错误与依赖缺失，全部保留。顶层error/退出码选择优先级：参数/授权错误 → 锁或版本冲突 → 必需依赖缺失 → 执行故障 → 内容/策略失败。示例因此返回依赖不可用，而没有丢弃XML错误。

位置不可得时null；上游原始错误码与Kepub码分开；checker版本、规则、配置、输入hash与原始报告路径保留。输出路径若含私有信息，在可分享报告中脱敏。

### 7.2 退出码

| 码 | 含义 |
|---|---|
| 0 | 请求按契约完成；不隐含书籍已接受/已验证 |
| 1 | 内容、校验或计划执行政策不满足 |
| 2 | 参数/schema/操作版本/授权输入不正确 |
| 3 | 必需依赖或能力不可用 |
| 4 | 工作区忙、版本冲突或过期计划 |
| 5 | Agent/外部执行器失败或超时 |
| 6 | 文件I/O或内部执行故障 |
| 130 | 用户取消 |

`--json`即使失败也不打印Python/Go堆栈到stdout；内部信息放stderr或脱敏报告。对外部工具不透传其退出码，而保留 `backendExitCode` 后映射到上述契约。

### 7.3 事件

JSONL事件至少含 `schemaVersion`、`requestId`、`workspaceId`、可空taskId、sequence、可空generation、type、data。sequence在请求内严格递增；重新连接通过状态快照和恢复游标，不能假定事件永不丢失。

task进程正常退出、协议terminal事件与文件冻结检查分别验证；有一项缺失就不能报告completed。进度文本、用户生成内容和终端控制字符不参与状态判定。

## 8. 接受、导出和权限

`task accept`在用户审核后显式执行，复核冻结hash/baseRevision；所有外部写者必须先停止，未来接入 Agent 后还须回收受管写进程。当前 M2 拒绝所有 error/fatal，不提供原书已有错误的确认豁免；T2 修复提案与 ValidationDelta 延续此门槛，错误数量改善也不能批准仍不合规的候选。检查缺失／结构不可安全解析不能靠确认绕过，strict warning 策略保持。

`workspace export`仅针对accepted；默认最终归档完整检查通过才生成正式产物。`--draft`是用户明确请求的例外，报告不能标pass。原书保护、已有输出冲突和校验政策不能被一个通用 `--yes` 取消。

GUI对Agent默认只暴露查询、提案和候选任务入口，不自动授予accept/export。原生Amp若可运行当前用户任意shell，仍可能调用用户权限下的CLI；这种限制是应用流程，不是OS安全隔离。高保证授权隔离必须另做执行环境，不能靠命令名字或AGENTS文件宣称完成。

## 9. 外部适配契约

不实现 `kepub calibre <任意参数>`。适配器从明确operation参数生成argv，并记录版本、支持状态和副作用。公开CLI优先；内部API helper仅在独立、受控、固定版本的适配层出现。

例如未来使用ebook-polish时，Kepub的 `--output` 必须映射成Calibre第二位置参数，而非 `-o`。可选内部CSS检查的Qt/WebEngine依赖单独探测，不因一个 `calibre-debug --version` 成功就宣称可用。

外部命令输入/输出都在task artifacts中，不能给原书作为输出目标；实际输出重新安全导入，检查条目、hash、未知资源和格式变化。模型和外部工具都不能跳过同一候选审核。

Issue #4 的可选 adapter 进一步遵循[开发方案 §8.3](DEVELOPMENT_PLAN.md#83-issue-4-的生态取舍与外部工件边界)：仅用冻结输入副本和指定输出目录，固定可执行文件／依赖 hash、版本、argv 与配置；默认不联网、不继承无关凭据，限制资源、输出和进程回收。独立目录或进程组不是 OS 沙箱，访问隔离须单独证明，许可义务也不因独立进程自动消失。

修复后 EPUB 是不可信工件，先完整安全导入、核对全部资源增删改及写集合，再通过版本化受控变换形成可审阅候选；**禁止直接覆盖活动 candidate**。允许写集合、来源和输出 hash 的持久格式须先与 T2 协调；历史重开／恢复验证冻结工件，不盲目重新运行不同版本第三方工具。异常／取消／半输出／二次检查失败不生成接受或正式产物，不清 journal 绕过。适配器尚未实现，不增加默认依赖或任意 shell 入口。

## 10. 文档生成与验收

实现后由注册表生成操作schema、help、capabilities结构和示例骨架；本文保留行为语义，生成物变更需契约测试。支持的CLI语法不能仅存在于README。

必要用例包括：

- 无GUI、无Amp、无Calibre的info/unpack核心流程；缺EPUBCheck时明确unavailable。
- JSON成功/失败各只有一个对象，JSONL断流不能成功；带空格/中文/短横线路径。
- partial引用计划不能apply；修改plan的applicable/writeSet不能绕过重算。
- 基线变化、操作版本变化、并发写、运行中Agent均拒绝危险apply。
- metadata no-op不改字节；改名更新入站/出站引用，不支持语法明确阻断。
- 接受并非导出；应用计划并非接受；工具退出0并非校验通过。
- 进程取消后无遗留写入者；阶段失败可回滚，输入原书未被覆盖。
- 提案序号重排／输入或规则变化／旧值不符／同文不同节点不误用批准；第二／第三文件失败、依赖修复撤销和恢复再次中断有原子或 journal 恢复证据。
- 诊断 A 消失而 B 新增且总数相同、fatal 减少仍有 error、blocked 后首次可查、范围过滤、同码不同引擎以及缺／截断报告不能混称改善通过。
- 无可选工具仍完成原生 CLI 闭环，缺必需 EPUBCheck 则正式接受／导出失败；外部输出增加字体／绝对路径／越界文件或改隐藏时间戳均被完整核验，不能绕过来源校验。

以上示例不自动注册为已实现命令；第二轮先实现校验打包与单字段候选编辑，再单独验收审核/导出。未实现项继续报告 planned；没有执行真实 Calibre/Amp 任务。
