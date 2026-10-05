# Kepub CLI 与操作契约

> 设计版本：0.5 · 日期：2026-10-05 · 状态：分阶段实现；对齐 Issue #3 与开发方案 v0.9，补 T1 语义和规范证据设计，不改变已实现 schema。实际可用命令以 README 与 `capabilities` 为准。
>
> 本文定义 Kepub 自己的命令和协议，不是 Calibre 或 Amp 的使用手册，也不表示命令已经能运行。架构见 [开发方案](DEVELOPMENT_PLAN.md)，设计依据见 [Calibre 研究](research/CALIBRE_CLI_REVIEW.md)。

## 1. 设计目标

CLI 是与 GUI 平级的产品入口，也是 Agent 的确定性工具层。不要求用户安装桌面端来查询或打包 EPUB；不要求运行模型来改一个已明确指定的字段。

一个子命令对应一个应用用例。GUI 不拼 shell 字符串复用 CLI；GUI/CLI 都调用 Go 服务。Agent 使用 CLI 时仍面对相同任务、校验与审核语义。

Obsidian CLI 的参考取舍见 [开发方案 §9.1／§9.2](DEVELOPMENT_PLAN.md#91-参考-obsidian-cli但保持真正-headless)：借鉴可发现能力、精确目标、结构化查询和差异，不引入桌面运行依赖、当前活动文件默认值或任意 eval。保留现有 `--option` 与 JSON 请求文件语法；Amp 经 shell 使用本 CLI 不需要 SDK 或 External API。反向运行 Amp 的接口边界见 [§8.1.3](DEVELOPMENT_PLAN.md#813-external-apicli-与-typescript-sdk-的适用边界)。此补充不改变现有 schema 或注册新命令。

## 2. 命令分组与实施次序

下表 M0～M6 是技术工作包编号，不再表示执行先后。当前先完成 [S0 规范资产与差距矩阵](DEVELOPMENT_PLAN.md#118-issue-3-阶段映射规范资产与关闭门槛)，再按 [开发方案 v0.9 §11.7](DEVELOPMENT_PLAN.md#117-v09-独立-cli-批次与完成标准) 完成独立终端制书：默认 EPUB3、EPUB2 → EPUB3 转换、完整 EPUB3 XHTML、样式和系统字体发现；本阶段不接入 UI、Amp 或外部编辑器。C1 发现／诊断、C2 有界内容读取／定位、C3 单个 XHTML 简单文本确定性修改已实现并作为源码发布到 main，尚无版本化安装包。C3 操作 schema 在 §2.2 冻结；支持边界见 [§11.4](DEVELOPMENT_PLAN.md#114-首个正文读写版本的边界)。新增能力仍为计划，见 §2.3～§2.5；CLI 完成不等于 Issue #3 的真实排版／无障碍目标完成。

| 命令形态 | 语义 | 阶段 |
|---|---|---|
| `kepub version --json` | 实际构建版本／Go／平台／可用 VCS 信息，不联网推测版本 | C1 本地已实现 |
| `kepub doctor --json` | 环境/架构/外部工具发现，不调用模型、不修改书籍 | C1 本地已实现 |
| `kepub capabilities --json` | 版本化能力和可用性；区别 planned/available/unavailable/unsupported | M1 |
| `kepub info BOOK --json` | 出版信息摘要，含规范/渲染能力警告 | M1 |
| `kepub toc BOOK --json` | 读取导航，不改变 spine | M1 |
| `kepub inspect BOOK --section manifest --json` | 指定范围的只读结构查询 | M1 |
| `kepub content --workspace DIR --resource BOOK_PATH [--query TEXT] [--limit N] --json` | 只读 accepted revision 的单个 manifest XHTML，返回有界文本／结构定位／资源哈希 | C2 本地已实现 |
| `kepub unpack BOOK --output DIR` | 解到新目录，安全检查，不覆盖 | M1 |
| `kepub pack DIR --output OUT.epub` | 清单式归档及正式检查，不是convert | M1 |
| `kepub validate BOOK_OR_DIR --json` | 分层诊断及覆盖报告 | M1 |
| `kepub workspace open BOOK --output DIR --json` | 仅新建工作区，返回持久ID；已有路径不覆盖 | M2 已实现 |
| `kepub workspace list --json` | 全局工作区发现/注册表尚未实现 | planned |
| `kepub plan --workspace DIR --operations FILE --output PLAN.json` | 从当前accepted基线生成单字段操作计划，不写出版内容 | M2 已实现 |
| `kepub apply --workspace DIR --plan PLAN.json --json` | 核对计划，创建候选任务；不接受/不导出 | M2 已实现 |
| `kepub task diff TASK --workspace DIR --json` | 实际完整文件增删改及元数据／正文目标的old/new文本 | M2／C3 已实现 |
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
- 仅支持所选 publication manifest 声明的 `application/xhtml+xml`，路径必须为精确 BookPath，不是 href／fragment／本机路径。沿用 UTF-8 XML、8 MiB、深度／token／索引限制；不支持的资源类型明确失败，CSS 查询留到后续，不为本批增加第二套语法。
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

- **EPUB2 → EPUB3：** 新增显式、版本化转换操作；维护元数据、唯一标识符、阅读顺序、NCX → nav、封面、必要 XHTML／CSS 和引用。冻结转换策略、输入与新增 ID／时间戳；展示真实差异，经正式 EPUBCheck 才能接受及正式导出。无法安全保留语义时明确阻断，不只改版本号、不删原书。边界见开发方案 §3.7；命令名称与 schema 尚未冻结。
- **正文与书籍结构：** 完整 EPUB3 XHTML 解析／保留，以及混合内容、结构编辑、全书检索／替换、章节和资源维护、默认新建 EPUB3、常用元数据与历史回退。只读语法兼容不直接赋予写权限；输入片段由内置操作解析并绑定 hash，不提供外部编辑器往返或任意文件写入入口。
- **按标题拆章（T4b）：** 将所选 EPUB3 spine XHTML 中的多个章节按标题元素或明确章名规则拆成独立文件。先只读预览候选切点，允许明确排除／补选；计划冻结规则、locator、源 revision／hash、输出路径与完整写集合，apply 重算，不把原始 XML 正则切割当结构编辑。只在安全节点边界拆，标题留在新章，前言不丢，重复标题不覆盖；同步 manifest／spine／nav／保留的 NCX 和跨章 ID／脚注／资源引用。引用覆盖不足或结构歧义拒绝，样式影响显式报告；不隐式合章、导入 TXT 或放宽资源上限。全批走候选／diff／正式检查／accept 或 reject，EPUB2 先显式转换。详见[开发方案 §3.9](DEVELOPMENT_PLAN.md#39-网络小说按标题自动拆章t4b待实现)；操作 ID、schema 与预算尚待冻结，当前没有可执行拆章命令。
- **多操作事务：** 新请求／计划／执行格式另行版本化，贯穿写集合、恢复、接受与历史来源校验；不能仅移除“一个操作”限制。现有 `metadata.set` v1 与 `content.text.set` v1 的含义、旧计划摘要和安全门槛保持。
- **字体与样式：** 发现本机字体族／字重／样式，区分精确安装与系统替代；只写 CSS 字体栈，不嵌入、下载或安装字体，不泄漏本机字体路径。已有字体资源保留；缺字体不等于 EPUB 不合规，也不保证别的阅读设备能用该字体。增加结构化 CSS 操作与基础图片排列，高级排版／媒体后置。
- **终端与平台：** 人类可读输出、文本 diff、任务状态及发行说明；JSON 仍保持机器契约。Linux 先验收，随后 Mac 打包／实机测试；不以 GUI、Amp 或外部编辑器作为前置。

以上通过实现与独立验收后才进入 capabilities 的可用项，不改变 §2.1／§2.2 的当前限制，也不使下文历史设计示例立即可执行。

### 2.4 T1 终端增量实施契约（已冻结语义，尚待实现与验收）

本节冻结 T1 新入口的语义，不声明当前二进制已可执行；通过独立验收后再更新命令表与可用状态。既有 JSON envelope、单资源 `content`、操作／计划／执行版本和正式检查门槛不变。

- `kepub search --workspace DIR --query TEXT [--limit N] --json`：持锁读取所选 rootfile 的 accepted revision，按 manifest 顺序检索其中所有 XHTML，不读取活动候选、不选另一个 rootfile。沿用 `content` 的区分大小写字面子串及节点／排除规则；命中数是匹配结果元素数，不是短语出现次数，不跨资源拼接匹配，也不是浏览器可见文本。
- query 必填，范围为 1～4096 UTF-8 字节；limit 默认 50、范围 1～200。返回工作区／revision／rootfile 身份、完整匹配数、返回数和 truncated；每个结果包含精确 bookPath、原资源 SHA-256、locatorVersion、locator 和解码文本。达到返回数限制仍须扫描剩余资源才能声称完整匹配数。不同资源出现同一 locator 不合并。
- 每资源沿用 XML 8 MiB／深度 128／200,000 tokens／32 MiB 索引限制；全书 XHTML 原始扫描字节累计上限 128 MiB，返回文本累计上限 1 MiB。超限或任一应扫描资源不可读取／解析时明确失败，不把未扫描部分当零匹配，不静默跳过坏资源；truncated 仅表示返回条数限制。
- `kepub task status TASK --workspace DIR --json`：查询精确任务的执行与结算状态，返回身份、基线及当前审核／接受／拒绝信息；未知任务明确失败，不以 latest 或其他任务替代。只读持锁，沿用 Open 的来源、恢复与安全树核验，不为状态查询绕过损坏记录。status 不产生批准或导出。
- 非 JSON 输出使用面向终端的可读摘要；任务 diff 按资源展示实际文本差异，不能把计划文本当候选事实。二进制、不可解码或超显示预算的资源显示路径、变化类型与字节／哈希摘要并明确未展示文本；不伪称没有变化。机器输出既有字段与含义不因终端排版而改变。

标准语法接纳不扩大 `content.text.set` v1 的结构权限。T1 编码支持须覆盖读取、定位、局部写入、来源重算与历史重开：未改资源保持原字节，已改资源保持原编码、BOM／声明和目标外字节。T1a 原生 HTML DOCTYPE／UTF-16 只是首个增量；T1b 须依据固定 [EPUB 3.3 §3.9](https://www.w3.org/TR/2026/REC-epub-33-20260113/#sec-xml-constraints) 与附录 B 补齐允许的声明／实体矩阵。合法但未覆盖的离线 DTD／实体子集暂报能力限制，留在差距表，不以 XML 不合规代替限制，也不全部推给 T3；不得读取任意外部 URL 或本机实体路径。EPUB2 迁移所需解析在 T3 增量实现。

### 2.5 Issue #3 的能力证据设计（计划，尚未加入机器输出）

规范支持与命令可用性分开。S0 固定规范原文和条款后，建立单一机读矩阵，未来向后兼容扩展 capabilities／doctor／诊断，不替换当前 envelope、退出码、operation ID 或检查状态枚举。

- 逐特性记录 `featureId`、`specVersion`、`specSection`、`normativeLevel`、`applicability`、`preserve`、`parse`、`edit`、`render`、`validate`、`platform`、`testIds`、`evidence`；五维状态为 `supported | partial | unsupported | policy-disabled | not-tested`，不适用另给理由，不用 supported 代替。
- MUST／SHALL、SHOULD、MAY、deprecated、条件要求与产品安全／预算策略区分。保留未知语法不表示可编辑；CSS 解析不表示渲染；规范允许但产品暂不支持或策略禁用，不伪称标准禁止。
- 诊断区分非法输入、资源限额、能力不足与策略禁用，附规范条款、精确 BookPath／可得位置、处理阶段及 revision／snapshot。功能状态与单次检查结果分别提供；当前必需检查通过不等于五维全部 supported。
- EPUBCheck、引用 coverage、阅读系统测试、真实布局和人工 accessibility 分列；证据绑定规范／checker／工具哈希、输入和实际平台。未测、缺工具、超时不能通过，不能只据 `CSS.supports()` 或 WKWebView 选型提升能力。

字段容器、schema 和诊断 code 在对应实现批次另行冻结；上述是设计，不是当前 CLI 返回字段。规范资产、S0～S4 映射与关闭条件见开发方案 §7.4／§11.8；T1～T6 不承担后续 UI 渲染验收，也不能据此关闭 Issue #3。

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

`task accept`在用户审核后显式执行，复核冻结hash/baseRevision；所有外部写者必须先停止，未来接入 Agent 后还须回收受管写进程。当前 M2 拒绝所有 error/fatal，不提供原书已有错误的确认豁免。后续若加入此类策略，必须针对当前检查报告显式确认，结果仍标未验证有效；检查缺失/结构不可安全解析不能靠确认绕过。

`workspace export`仅针对accepted；默认最终归档完整检查通过才生成正式产物。`--draft`是用户明确请求的例外，报告不能标pass。原书保护、已有输出冲突和校验政策不能被一个通用 `--yes` 取消。

GUI对Agent默认只暴露查询、提案和候选任务入口，不自动授予accept/export。原生Amp若可运行当前用户任意shell，仍可能调用用户权限下的CLI；这种限制是应用流程，不是OS安全隔离。高保证授权隔离必须另做执行环境，不能靠命令名字或AGENTS文件宣称完成。

## 9. 外部适配契约

不实现 `kepub calibre <任意参数>`。适配器从明确operation参数生成argv，并记录版本、支持状态和副作用。公开CLI优先；内部API helper仅在独立、受控、固定版本的适配层出现。

例如未来使用ebook-polish时，Kepub的 `--output` 必须映射成Calibre第二位置参数，而非 `-o`。可选内部CSS检查的Qt/WebEngine依赖单独探测，不因一个 `calibre-debug --version` 成功就宣称可用。

外部命令输入/输出都在task artifacts中，不能给原书作为输出目标；实际输出重新安全导入，检查条目、hash、未知资源和格式变化。模型和外部工具都不能跳过同一候选审核。

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

以上示例不自动注册为已实现命令；第二轮先实现校验打包与单字段候选编辑，再单独验收审核/导出。未实现项继续报告 planned；没有执行真实 Calibre/Amp 任务。
