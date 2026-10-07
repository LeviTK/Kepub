# Kepub

独立终端 EPUB 创建、修改与维护工具；后续扩展阅读／制作 UI 和 Amp 协作。

**当前状态：开发方案 v0.10，S0 固定规范与差距矩阵、T1a 编码／终端基础及 T1b 内部 DTD／实体／默认属性均已通过独立审计和父验收，源码已同步到 GitHub main。T1 已完成规定的兼容与终端基础范围；T2 首批版本化多操作／多资源事务底座已通过独立 high 审查与父本地验收（固定提交 23ba8f8），第二批 XHTML 混合内容与结构编辑经四棵固定树拒绝后完成根因修复，最终固定树 79a4548 通过 medium 独立审查与父验收并由父本地集成（公开记录见 [docs/verification/T2_MULTI_OPERATION.md](docs/verification/T2_MULTI_OPERATION.md)）；T2 第三个增量（显式范围字面／正则批量文本替换）开发中，T2 尚未完成，不代表完整 EPUB3 编辑器已经完成。** 后续按 Issue #3／#4 推进独立 CLI 的 T1～T6：默认 EPUB3，补 EPUB2 → EPUB3 转换、完整 EPUB3 XHTML 编辑、样式、系统字体发现和基础图片，不新增嵌入字体；UI、外部编辑器和 Amp 集成后置。既有 M1／M2／C1～C3 已作为源码发布到 main，新增交付状态分别记录。Linux 先功能测试，再打包和实测 Apple Silicon Mac；当前没有版本化安装包或 Mac 实机验收，文档 v0.10 不是软件发行版本。

S0 归档与门禁：规范原文、官方用例来源与上游研究索引已本地建立。首轮实现 Droid 审查曾发现门禁、证据关联及条款／schema 遗漏；各项修复、复审和实际放行结论以 [父验收记录](docs/verification/S0_PARENT_ACCEPTANCE.md)为准，不能用审查进程 exit 0 代替批准。当前固定资产含 118 项规范／关联文件和 169 个用例 ID／170 本必需测试书；归档验证不等于语义完整性、用例执行或 CLI 全规范兼容。S0 放行必须同时具备绑定同一输入的父验收与真实独立审计批准，离线 `gate` 核验其证据。依用户最新指令，当前审计使用 Amp 的 DeepSeek V4.1 Flash，Droid 待用量恢复后使用；两种执行证据分别核验，不改写历史审查结果，也不降低放行标准。EPUBCheck 5.3.0 对三项 2026 REC 修订的 12 个正反样本已实际核验，不能外推为全规范覆盖。范围与历史限制见 [S0 矩阵](docs/EPUB33_SUPPORT_MATRIX.md)和[检查器核验](docs/verification/S0_EPUBCHECK_2026.md)；源码与公共规范资产已同步到 main，私有原书、审计原始证据包与本地收据未提交，历史记录中的“未推送”保留为当时状态。

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
- XHTML href/src（含 nav）解析目标时去掉两端 ASCII 空白，报告仍保留 XML 解析后的原属性值；不会删除 NBSP 或百分号编码的文件名空格，也不会把这条规则全局用于 OPF、NCX、SVG、CSS 或磁盘路径。
- 支持 EPUB2/3 ZIP 与严格 UTF-8／UTF-16 LE／BE XML（含 BOM），接纳原生 `<!DOCTYPE html>` 及有界内部子集、内部通用／参数实体、默认属性和 NOTATION。EPUB3 按版本及 manifest 媒体类型检查允许的 DOCTYPE 标识符，外部子集零读取；EPUB2 合法但未支持的外部声明仍明确 exit 3。目录输入的只读查询、`xml:base`、远程 manifest href 仍不支持，不容错改写。
- 拒绝穿越、绝对路径、特殊/符号链接/加密 ZIP 条目、重复和大小写/Unicode 碰撞（包括隐式父目录）。实际解压限制：20,000 条目、256MiB/文件、2GiB 总量；路径 4096 字节/128 层；解析 XML 8MiB/128 层/200,000 tokens、累计文本/位置索引 32MiB。这些是初始产品策略，不是 EPUB 标准。
- 原书只读，所有资源（含非 manifest 文件与空目录）保留；unpack 完整 staging 后原子发布，不覆盖已有路径。输出父目录必须已存在。Linux 和 macOS 支持原子不覆盖发布；macOS **只做交叉编译，尚未实机验证**。突然终止可能留下私有临时 staging，但不会发布半成品目录；不宣称断电耐久性。
- `--json` 成功/失败 stdout 都是一个统一 envelope；失败用稳定 code，退出码 1 内容/安全问题、2 参数、3 未实现能力、6 I/O。`--output/-o`、`--rootfile`、`--section` 可置于 BOOK 前后，`--` 结束选项，JSON 和 `--no-input` 不问答。其他契约中的选项尚未实现，会拒绝，不忽略。
- 固定版式、脚本标记、SMIL/音视频、签名、加密/字体混淆声明会记录限制；不执行、不解密、不验证签名、不删除内容。缺失 manifest 资源显式报告。成功只表示只读请求完成，**不是 EPUB 合规验证**。

workspace list/注册表、资源改名、GUI、预览、生产 Agent、Mac 发布仍未实现。capabilities 中这些项为 `planned`，不能据此执行。当前 schema 描述已实现命令的参数和结果基本形状，不代表所有操作的细粒度结果 schema 均已完成。

验证：`go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...`；真实 CLI smoke 含 EPUB2/3、双向引用与错误 envelope。小型 EPUB 与恶意输入由测试生成，无第三方书籍。证据见 [M1-A](docs/verification/M1_A.md) 和 [M1-B1](docs/verification/M1_B1.md)。

## 本地已集成：C1 命令发现与诊断

```sh
go install ./cmd/kepub  # 安装到 GOBIN，未设置时为 $(go env GOPATH)/bin
./kepub --help
./kepub inspect --help --json
./kepub version --json
./kepub doctor --json
```

上面的 `./kepub` 使用前文构建的本地二进制；`go install` 后也可将安装目录加入 PATH，以 `kepub` 调用。构建需要 Go，已构建的 CLI 不要求用户安装 Go、Node、MyGo 或 Amp。正式 EPUB 校验另需 Java 17+ 与固定版 EPUBCheck。

- 命令帮助、capabilities 的增量 `commandSchemas`、必填及允许参数使用同一描述来源；保留原 capabilities 数组和 operation ID。未知选项、重复参数或非法值不会因加 `--help` 被忽略。
- `version` 返回实际 Go 构建／VCS 信息；没有发行标签的构建明确为 development，不联网推测版本。
- `doctor` 区分核心可运行与正式检查器可用。缺依赖仍可成功收集诊断，但 `formalValidationAvailable:false`；外部探测故障明确失败。探测有时间和输出上限，只发现 Amp 路径，不启动 Amp、不验证登录、不安装依赖。

证据见 [C1 CLI 验证](docs/verification/C1_CLI.md)。此批为本地源码交付，不是已发布的安装包或 Mac 实机验收。

## 本地已集成：C2 已接受版本的正文查询

```sh
./kepub workspace open 'book.epub' --output 'work' --json
./kepub content --workspace 'work' --resource 'EPUB/Text/chapter.xhtml' --json
./kepub content --workspace 'work' --resource 'EPUB/Text/chapter.xhtml' --query '目标短语' --limit 20 --json
```

`--resource` 必须是所选 manifest 中 XHTML 的精确 BookPath，不能传 href、fragment 或本机路径。返回 `workspaceId`、`revisionId`、`rootfile`、原资源 SHA-256、结构 locator 和解码文本；始终查询 accepted，即使存在被修改的活动候选。持锁查询，并发占用返回 `WORKSPACE_BUSY`（exit 4）。不需要 Java、Amp、Node 或 GUI。

query 区分大小写、按单个结果元素的文本做字面子串匹配；省略表示全部，显式值须为 1～4096 UTF-8 字节。limit 默认为 50，范围 1～200，另返回完整匹配数和 `truncated`，不会默选第一个匹配。XML 输入上限 8 MiB、返回文本累计上限 1 MiB；超限明确失败，不截断节点文本。

结果排除 script/style/head、外来命名空间子树及含这些内容的祖先。混合内容可读，但不代表允许修改；这不是浏览器可见文本、全书搜索或 EPUB 合规检查，locator 也不是可写偏移。证据见 [C2 内容查询验证](docs/verification/C2_CONTENT.md)。C3 在此基础上增加受限正文操作；这些 CLI 能力本身不等于真实 Amp 联调已通过。

## 本地已集成：C3 受限正文修改

正文请求使用 `schemaVersion:2`，恰好一个 `content.text.set` v1 操作。先运行 `content`，从同一次结果复制 `bookPath`、`revisionId`、`resourceSha256`、`locatorVersion` 和所选节点的 `locator`、`text`；将 `text` 作为 `expectedOldValue`，另填 `newValue`。不能只凭相似文本选择首个匹配。字段与兼容规则见 [CLI 契约 §2.2](docs/CLI_CONTRACT.md#22-c3-受限正文修改实施契约)。

请求仍经下文的 `plan → apply → task diff → task accept/reject → workspace export`，没有直接正文写入命令。只修改 manifest XHTML 的单个 body 后代简单文本元素，支持显式空元素，不支持混合内容、子元素、注释、CDATA、处理指令、自闭合或脚本／样式目标。新值按 XML 文本转义，不解释成标签；目标外字节和 OPF 时间戳保持，no-op 不改字节。读取绑定过期明确拒绝，不自动重新找相似目标。

正文 plan 和 execution 为 v2，已有 `metadata.set` 请求、计划、执行记录与摘要仍保持 v1。diff 的 `content.newValue` 是实际候选文本，不是计划新值；安全普通文件树的候选字节漂移可审阅／拒绝，不能接受。`content` 仍只读 accepted，accept 与正式 export 仍须真实 EPUBCheck。正文之外的结构、CSS、批量修改和真实 Amp 联调不在 C3 范围。

父 orb 独立执行 67 次真实 CLI 调用：旧二进制生成的 v1 计划／待审任务兼容、v2 操作／策略摘要、同文第二节点的精确修改、正文接受／正式导出／拒绝、空值／no-op、陈旧读取绑定和候选漂移均通过。导出仅替换预期节点文本，其他字节与原书 SHA-256 保持。证据见 [C3 正文修改验证](docs/verification/C3_CONTENT_EDIT.md)。C2／C3 当时保留的编码 href 报告兼容问题已在后续独立审查中修复；历史报告保留当时的失败结论，当前绑定规则见下节。

## 本地已验收：T1a 编码与终端基础

```sh
./kepub search --workspace 'work' --query '目标短语' --limit 20 --json
./kepub task status '实际 taskId' --workspace 'work' --json
./kepub task diff '实际活动 taskId' --workspace 'work'
```

`search` 按所选 manifest 顺序扫描 accepted 的全部 XHTML，达到返回条数后仍完成扫描和计数；不读取候选、不跳过坏资源。`task status` 区分指定活动任务与已结算历史，不用 latest 替代。历史任务的 `matchesExecution:false` 表示该活动候选字段不适用，不表示历史损坏；历史 `task diff` 仍拒绝（exit 4）。非 JSON 输出为可读摘要，diff 展示实际候选的分文件差异；二进制／超预算内容明确省略并保留大小与哈希，显示文本不是可应用 patch。

原生 HTML DOCTYPE 与 UTF-16 的读取、定位、简单文本局部替换、来源重算和重开保持原编码及目标外字节。原始 XML 8 MiB、解码 UTF-8 16 MiB、索引 32 MiB 分别限额；全书搜索扫描 128 MiB、返回文本 1 MiB，不因编码扩展放宽预算或 v1 写权限。内部声明／实体／默认属性由下述 T1b 独立验收。

**解析成功不等于正式校验通过：** 固定 EPUBCheck 5.3.0 拒绝 UTF-16 XHTML（`HTM_058`），正式 accept/export 保留失败；UTF-16 OPF／container 搭配 UTF-8 XHTML 可在非 strict 下通过，strict 仍拒绝 `RSC-027` warning。`workspace export --draft` 只导出 accepted，绝不夹带待审候选。

父侧 208 次独立 CLI 调用、未经预处理的授权原书（53 个 ZIP 条目逐字节对照）、旧二进制 v1／v2 兼容、root 普通／race／vet及 Darwin 交叉构建均通过；DeepSeek V4.1 Flash 批准本 T1a code/demo 范围，无阻塞问题。具体范围、失败记录与未测项见 [T1a 验证及父验收](docs/verification/T1A_ENCODING_TERMINAL.md)。这不是全产品复审、T1b 完成、Mac 实机测试或发行。

## 本地已验收：T1b 有界 XML 内部子集与来源

内部 ELEMENT／ATTLIST／通用及参数实体／NOTATION 按非验证 XML 规则处理，支持默认值、属性规范化与含标记实体；不读取外部 DTD，不把 DTD 有效性约束误报为 XML 非良构。允许的 PUBLIC 标识符只在匹配时折叠规范允许的空白，SYSTEM、声明和资源原字节不改。

未展开实体按需通过 `xmlCoverage` 报告原字节区间及来源，目录／引用显式 partial；未知 href/src 不生成肯定目标或“缺失”诊断，已知结构错误仍报告。`content/search` 无法给出完整文本时 exit 3。物理独立标签内的已知纯文本实体可整段替换，no-op 保留引用；生成节点、混合内容和未知内容仍不可直接写。

展开后实际序列化流与中间替换工作各限 16 MiB，不放宽原始／索引预算。父侧 **507 次独立 CLI 调用**、root 普通／race／vet、Darwin 交叉构建及编码方三组 live fuzz 均通过；DeepSeek 最终复审批准固定候选。前两轮拒绝、预算／命名空间／未知导航反例及一次旧树 race 超时均保留，详见 [T1b 验收记录](docs/verification/T1B_XML_SUBSET.md)。本批不新增结构操作、CSS／字体／图片编辑或发行，不提升全规范五维能力声明。

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

EPUBCheck 报告的完整 ZIP 库存仍按原始文件名、显式大小和校验和逐项精确绑定。报告还可能包含 NFC 明文／URL 编码的附属元数据行；仅在字段明确为零大小和 null 校验信息、且名称能由冻结库存唯一派生时识别，不能用它们补齐缺失库存或修正错误哈希。不会全局解码文件名，也不会把字面 `%20` 文件与空格文件合并。父 orb 另用 EPUB2／3 完成 72 次真实 CLI 验收，覆盖中文、字面百分号、NFD 和原始 1539 字节／编码后 4545 字节长路径的八次接受／正式导出，所有非目标资源、目录和原书字节保持；真正不合规的文件名仍由正式检查拒绝。

父 orb 在干净非交互 login shell 中配置真实 EPUBCheck 5.3.0，最终闭环合并后 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 全通过；普通／race 的 CLI 为 87.583s / 81.906s，workspace 为 108.225s / 108.602s，validation 为 151.116s / 142.217s。Darwin arm64 全仓交叉编译通过，没有执行。普通未配置 checker 的测试会跳过外部检查用例，不能代替上述验证。独立 SDK 模块另跑 `npm run check` 与 `npm test`：31/31 Node 场景及 Go race 通过，既有 SDK 失败门槛结论不变。

## 本地已集成：M2-B 确定性编辑与审核导出

`internal/workspace` 提供 Create/Open/Close、唯一独立候选、Checkpoint/Checkpoints/Restore 和精确内容树 SHA-256。原书、初始版本、候选与检查点均为独立副本；flock 保证协作进程单写者，恢复 journal 处理已记录的中断边界。候选普通写入不会污染基线；已知不支持内容和书内 Agent 配置保留但限制候选创建。

元数据 v1 计划仅修改一个唯一选中的现有 `dc:title` 或 `dc:creator` 简单文本，匹配预期旧值；C3 正文 v2 计划见上节，两类均每计划一个操作。目标外 OPF 与资源字节不变，no-op 不重排 XML 或更新时间。计划绑定工作区身份、绝对路径、当前 accepted revision、完整树和策略；apply 重新计算，失败回滚，成功仍是 `review_required` / `conformance:not_run`，不是自动接受。

工作区整体搬迁后，未执行的旧计划失效；已开始任务和已接受历史仍按身份、记录和精确内容来源核验。活动任务可审阅／拒绝，已接受历史可继续读取，原有持久决定可恢复。已登记执行的中断写入回滚；仅建候选、尚未登记执行时保留外部漂移供审阅并记录 failed，不将其晋升为基线。失败任务不能接受，也不会自动重跑；正常待审任务的显式接受仍需真实 EPUBCheck。完整边界见 [CLI 契约 §2](docs/CLI_CONTRACT.md#2-命令分组与实施次序)。

将下面请求保存为工作区外的 `operations.json`，并把旧值替换为书内的实际标题。若有多个标题，须提供明确的可选 `id`，不能自动选择第一个。

```json
{
  "schemaVersion": 1,
  "operations": [{
    "operationId": "metadata.set",
    "operationVersion": 1,
    "params": {
      "namespace": "http://purl.org/dc/elements/1.1/",
      "localName": "title",
      "expectedOldValue": "原书标题",
      "newValue": "修订标题"
    }
  }]
}
```

```sh
./kepub workspace open 'book.epub' --output 'work' --json
./kepub plan --workspace 'work' --operations 'operations.json' --output 'plan.json' --json
./kepub apply --workspace 'work' --plan 'plan.json' --json
# 将 TASK_ID 替换为 apply 返回的 data.taskId，先审阅再接受
./kepub task diff TASK_ID --workspace 'work' --json
./kepub task accept TASK_ID --workspace 'work' --json
./kepub workspace export 'work' --output 'edited.epub' --json
```

- 不接受修改时，在 accept 前执行 `task reject TASK_ID --workspace work`；记录与检查点保留，随后可生成新计划。已消费计划不能重跑，旧 task ID 不能操作新的候选。
- accept 对独立冻结树运行正式检查，通过后生成独立 revision；下一次编辑使用新基线。检查尝试与接受决策分开记录；最终取消检查位于持久 journal 发布之前，意图提交后重开只向前完成。没有草稿接受或既有错误豁免。
- export 只读取当前 accepted，而不是 active 候选；初始导入不带合规通过状态。正式导出重新检查最终 ZIP，显式 `--draft` 才允许未正式验证的草稿。候选被外部改动后，只要仍是安全、可完整散列的普通文件与真实目录树，就可以重开 diff/reject，但不能沿用旧执行状态接受。
- 链接、特殊文件、路径碰撞或损坏来源仍拒绝打开；accepted 的 content/export 也不绕过工作区核验。外部写者引入这些条目时，须停止写者并保留现场，只修复已确认由自己引入的候选条目，再以原 taskId 审阅／拒绝；不能通过删除任务、used 记录、检查点或 journal 绕过检查。详见 [CLI 安全树边界](docs/CLI_CONTRACT.md#2-命令分组与实施次序)。
- 工作区用明确目录定位，不使用全局注册表或 latest。open 只新建，不覆盖；plan 报告和 export 输出必须在整个工作区根之外、父目录已存在且输出不存在。操作／计划文件拒绝链接和特殊文件，FIFO 无 writer 不阻塞。

父 orb 另用含 BOM/CRLF/注释、特殊字符及空目录的自制 EPUB3 执行 27 次真实 CLI 调用：两次接受推进基线、旧计划拒绝、未接受内容隔离、缺 checker 拒绝、候选新增文件的 diff/reject、no-op、FIFO 与输出保护均通过。导出仅含预期两处 OPF 文本替换，其他文件字节完全一致；原书 SHA-256 未变。外部写者必须先停止；flock 与 cwd 不是同用户 OS 沙箱，断电和 ENOSPC 未证明。接口、恢复边界与 EPUB2/3 验证见 [M2-A](docs/verification/M2_A.md) 和 [M2-B](docs/verification/M2_B.md)。

来源复验会遍历 accepted 历史及完整内容树，历史增长会增加读取成本；大型书籍与长历史的目标平台性能尚未验收，当前不缓存跳过这些校验。

本批代码在源码发布前经真实 Factory Droid CLI 0.233.0／Claude Opus 5.5／medium 的 12 轮审查与修复；最后一轮独立重读全部生产 Go 和当前契约，正常 completion／exit 0，未发现剩余可复现实质缺陷。稳定产品的全仓普通／race／vet、真实 EPUB2/3 接受／导出及 Darwin 交叉构建通过。各轮失败、修复反例、准确覆盖范围和未验证限制保留在 [完整审查记录](docs/verification/DROID_REVIEW.md)；这不是绝对无 bug 或 Mac 实机、断电耐久性保证。代码已推送到 main；审查记录保留当时尚未推送的交付状态，不改写历史验证结论。

## Orb 启动

`.agents/setup` 仅支持 Linux amd64，固定 Go 1.27.1，从根及 SDK 实验的锁文件副本预热/验证 Go 缓存。存在 SDK 实验 npm 锁时，复用精确 Node 26.10.0 / npm 10.9.9，否则校验官方归档后局部安装；`npm ci` 从固定锁和完整性校验缓存准备实验依赖。没有 SDK 锁时不安装 Node/npm。所有工具链通过非交互 login shell 可用，不改系统 Node 或仓库锁文件。

第二轮 setup 另准备 Java 17+ 与官方 EPUBCheck 5.3.0：验证完整下载包，保留全部依赖 JAR，暖运行核对精确文件集合和内容，并持久提供 `KEPUB_EPUBCHECK_JAR`。这只是开发/正式检查依赖，不使只读 CLI 依赖 Java。缺失时才安装 Debian JRE；其安全更新版本不伪称为固定 patch。

`.agents/resume` 只修复 Go 链接与快速检查工具/依赖，不安装、不认证、不启动服务。已有 Go/SDK 的父 orb 补装 Java/checker 为 10.92s；最新暖 setup 3.14s、resume 0.44s，锁文件未变。额外 JAR 被 resume 拒绝，setup 重装修复 4.73s，干净 login shell 验证通过。证据见 [EPUBCheck 环境验证](docs/verification/ORB_EPUBCHECK.md)，此前 Go/SDK 的空 HOME 测试见 [原 setup 验证](docs/verification/M2_A.md#独立后续sdk-实验的-orb-setup-验证)。不预装 Calibre/GUI，不是 Mac 安装器；第二轮 setup 已随源码推送默认分支，但新服务端快照尚未单独验收。

## 文档

- [开发方案 v0.10](docs/DEVELOPMENT_PLAN.md)：Issue #3 的规范／阶段映射，以及 Issue #4 的安全修复、分层诊断、上游采用边界与验收；保留既有交付记录。
- [CLI 与操作契约](docs/CLI_CONTRACT.md)：拟定命令、OperationRegistry、plan/apply、机器输出、退出码与验收要求。
- [Calibre CLI 与编辑内核研究](docs/research/CALIBRE_CLI_REVIEW.md)：官方命令全景、关键源码调用链、证据及采用/不采用的设计。
- [开发方案 v0.2 历史原文](docs/history/DEVELOPMENT_PLAN_V0_2.md)：Calibre 研究后形成的上一版设计。
- [开发方案 v0.1 历史原文](docs/history/DEVELOPMENT_PLAN_V0_1.md)：最初的完整方案。

## 产品方向

先完成普通可重排 EPUB3 的终端制书闭环；EPUB2 保留既有读取和受限操作，新增完整编辑能力前显式转换到 EPUB3。新建默认 EPUB3，不在打开或导出旧书时静默升级。T1 已补齐规定的 DOCTYPE／UTF-16／内部子集兼容，单操作与简单文本写权限仍保留；这不是宣称所有合法 EPUB 已可直接编辑。

CLI 先验证 Linux，再打包并实测 Apple Silicon Mac。后续桌面首发仍面向 Apple Silicon，**已确认采用 MyGo 与系统 WebView**，当前接口基线为 MyGo 0.2.0，macOS 使用 WKWebView，不捆绑 Chromium。主编辑窗口计划使用 Go + TypeScript，MyGo 网页控制通过 `Window.Page()` API；React + Vite 仍是前端计划，尚未实现。MyGo 纯 Go 原生 UI 只作为独立设置、诊断、检查器等辅助窗口的候选。独立 `kepub` CLI 与 GUI 共用 Go EPUB 核心。

Bridge 是系统 WebView 的脚本通信适配，与系统 WebView 本身不冲突；框架选型确认不代表预览安全已经通过。MyGo 0.2.0 公共网页窗口的顶层页面会注入 bridge，另开窗口不等于无桥或独立存储；导航拦截也不等于禁止所有网络请求。[边界实验](experiments/mygo-boundary/README.md)已在真实 Linux WebKitGTK 发布模式复现同源 iframe 经 `parent.mygo` 调用 Go，并验证受限 sandbox 对照；这不是 Mac 或完整 EPUB 预览验收。出版物隔离仍须在 MyGo／WKWebView 适配内解决并完成 Apple Silicon release 实测，详见 [预览安全边界](docs/DEVELOPMENT_PLAN.md#62-可信壳与不可信出版内容)。

当前只建设不依赖模型的确定性 EPUB 操作和工作区、差异、检查、审核、导出；预览与 Amp 内容协作后置。Calibre 是参考和可选适配，不是运行核心功能的必需依赖。以下为后续桌面流程，不是当前 CLI 已实现的界面：

```text
打开 EPUB → 阅读并定位
                ↓
     确定性操作 或 Amp 编辑
                ↓
       候选结果 ↔ 实时预览
                ↓
 差异 + 检查覆盖 → 审核接受 → 导出
```

## 历史：v0.4 Amp 接入对照实验

Go 仍负责出版物、工作区、写租约、候选冻结、审核和导出。此前已并行比较 **A：Go 直接管理 CLI/JSONL** 与 **B：Go 监督 Node/TypeScript SDK 辅助进程，再调用 CLI**，采用相同输入/事件约定和取消、异常退出等故障场景，后续择一进入生产适配。SDK 不放入 WebView，也不成为只读 CLI 的依赖。

第一轮四条 Ultra 工作线已经完成：M1-B1 目录/引用/覆盖、M2-A 工作区快照与单写者、M0-A CLI 原型、M0-B SDK 原型。实现目录与依赖边界见 [第一轮分工](docs/DEVELOPMENT_PLAN.md#111-第一轮并行工作与文件所有权)。第二轮改用 Medium，M1-B2 校验/打包和 M2-B 元数据计划/候选/diff、审核与导出也已本地集成；范围见 [第二轮记录](docs/DEVELOPMENT_PLAN.md#112-第二轮medium-并行实现随后集成编辑闭环)。原型成功不等于生产 Agent 已接入；真实 Amp 与 Mac 实机验证单独列为门槛。

实验结论为 **A 优先进入后续受管 Agent 验证，B 暂不晋级**。固定 SDK 实包复现 stderr 排空、退出早于 EOF、原始未结束消息无大小上限的缺口；其测试通过代表成功复现并收敛失败，不是协议门槛通过。A 无需新增运行时，已通过 Linux 受控协议与进程测试，但两者都未验证真实 Amp、配置/权限发现、任意脱组写者或 Mac 回收。证据与离线命令见 [CLI 原型](docs/verification/AMP_CLI_SPIKE.md) / [SDK 原型](docs/verification/AMP_SDK_SPIKE.md)；后者为独立模块，根目录 `go test ./...` 不运行其 Node/Go 测试，须在 `experiments/amp-sdk` 执行 `npm test` 和 `npm run check`。

编辑器集成只借鉴文档/选区/诊断与 Agent 分离的职责，不复刻已停用 VS Code 侧栏或其私有协议。后续如需实时工具交接，再在 OperationRegistry 稳定后评估公开 MCP/插件 API。

## 历史：v0.3 的关键变化

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

## 开发路线：先完成独立 EPUB3 CLI

| 阶段 | 开发批次与终点 |
|---|---|
| S0．规范资产与差距矩阵 | 按 §11.8 有限清单归档固定规范，三份 REC 按来源逐条清点、映射与对账；3.3 测试报告和用例源码分别锁定，核对 checker 的 2026 增量；外部规范直接引用清单不递归归档整个 Web |
| T1a／T1b．标准兼容与终端基础 | 两批均已本地验收：原生 DOCTYPE／UTF-16、可读输出／diff／status／search，以及 §3.4 规定的内部声明／实体／默认属性、来源与预算矩阵；保持单操作和简单文本写权限 |
| T2．内置编辑、多文件事务与修复提案 | 先完成事务／来源重算／恢复，再增加混合内容、结构修改、批量替换及原生 FixProposal、ValidationDelta 和依赖回滚 |
| T3．EPUB2 → EPUB3 首批 | 先冻结 EPUB 2.0.1 及受信离线 DTD／实体资产，再显式迁移 OPF、NCX／nav、封面、必要 XHTML；首批 CSS 原字节保留，需改 CSS 的迁移由 T5 扩展；正式验证且保留原书 |
| T4．新建与维护 | T4a 默认新建 EPUB3，元数据、章节／目录／资源与历史；T4b 网络小说按标题自动拆章，预览切点并同步阅读顺序和链接 |
| T5．样式、系统字体与基础图片 | CSS 管理、字体族发现与回退声明；不嵌入字体；基础图片与简单排列；完成 CSS 迁移及 T4b 引用扩展验收，T6 回归正式 ZIP。T4a 后可开工，通过须有 T4b 首版联调 |
| T6．CLI 验收与发行 | 无可选第三方工具的新建／转换／修复终端流程，Linux 功能测试与构建，再做 Mac 打包及实机验收；正式检查仍需 Java／固定 EPUBCheck |

以上通过后才称为普通 EPUB3 的创建、修改与维护闭环。近期不做完整 EPUB2 编辑器、新建 EPUB2 或 EPUB3 降级；高级排版／媒体、UI、外部编辑器和 Amp 集成后置。T1 新入口的语义见 [契约 §2.4](docs/CLI_CONTRACT.md#24-t1-终端增量实施契约已本地验收)；除已验收的 T1 外，其余新增命令与操作仍按批次冻结、实现和验收，不能将计划能力当可执行功能。

当前 EPUB3 核心接受规定范围内的允许声明，但不读取任何外部 DTD 子集；未来仅显式 EPUB2 迁移使用冻结的有限离线资源。EPUB2 合法但暂不支持的声明报能力限制，不误套 EPUB3 非法规则。良构但无法展开的实体保留并明确 partial；`content/search` 不能提供完整文本时失败，不把未知内容当空串。T1 的 XML 8 MiB 按原始资源含 BOM 计量，另设每资源解码／展开预算，读取、编辑和来源重算一致；实体生成节点／默认属性不能伪造为原字节可写位置。

按标题自动拆章已加入计划：识别 XHTML 标题或明确规则匹配的独立章名（如“第一章”“第001章”），先列出边界供审阅，再拆为独立 XHTML，保留前言、正文、CSS／图片关联，并维护目录、阅读顺序与跨章链接。首版不在任意嵌套位置强切，不自动合章，不承诺拆前拆后的分页／样式完全相同；超过现有解析预算仍明确拒绝。可行性、依赖与反例见 [§3.9](docs/DEVELOPMENT_PLAN.md#39-网络小说按标题自动拆章t4b待实现)，当前尚未实现。

[Issue #4](https://github.com/LeviTK/Kepub/issues/4) 的生态借鉴不是“全部集成”：Readium／Go EPUB 库优先作解析对照，修复器借鉴可解释提案与反例，epubveri／Ace／Calibre 保持可选；Readium CSS 留给后续阅读层，不替代 T5 的 CSS 解析器。T2 提案绑定输入、精确操作和风险，ValidationDelta 区分已解决、持续、新增／升级、新近可检查和不可比较的问题，**错误数量改善不能代替正式检查**。外部修复工件不得直接覆盖活动候选；许可、版本和供应链须先核验。详见 [§8.3](docs/DEVELOPMENT_PLAN.md#83-issue-4-的生态取舍与外部工件边界) 与 [§11.9](docs/DEVELOPMENT_PLAN.md#119-issue-4-的阶段落点与落实条件)。本次依据 issue 的静态研究调整计划，没有运行或重新审计这些上游工具。

[Issue #3](https://github.com/LeviTK/Kepub/issues/3) 的总目标继续保留：S1 内容模型与无损编辑由 CLI 批次落实；S2 作者原样预览、S3 完整排版／媒体交互、S4 官方测试／Mac／人工无障碍随后单独验收。**T6 通过不等于关闭 Issue #3。** 保留、解析、编辑、渲染、验证分别记录状态，EPUBCheck pass、引用 complete 或采用 WKWebView 均不能证明全部支持；固定 3.3 用例，不混用默认指向 3.4 的测试入口。

系统字体只属于运行 CLI 的机器；书中只写字体族和回退，不复制字体本体，也不删原书已有字体。另一台阅读设备未安装或不采用所选字体时会替代，不能保证跨设备外观一致。EPUBCheck 合规不等于视觉排版验收。

T2 另负责标准 IDPF 字体混淆书的受控编辑资格，保持字体字节与密钥标识符，旧工作区须显式版本化重评，不删除只读原因或历史证据。T4a 改名须同步 encryption.xml 的 CipherReference，否则拒绝；本阶段不删除被混淆字体。未知／非标准／混合加密及签名书在 T1～T6 仍只读；不是解密或重签功能。T3～T5 覆盖标准混淆字体保留，实际呈现由 S2 验收。

当前 `task accept` 不能鉴别人类和 Agent，“先询问用户”是合作约定，不是权限隔离。GUI 不通过拼接命令或解析终端文本复用业务逻辑，而与 CLI 共用 Go 用例。

参考 Obsidian CLI 的命令发现、目标选择、查询与诊断，但 Kepub 保持无 GUI 可运行，不照搬当前活动文件或任意 eval；具体见 [CLI 构建取舍](docs/DEVELOPMENT_PLAN.md#91-参考-obsidian-cli但保持真正-headless)。[Amp 接口边界](docs/DEVELOPMENT_PLAN.md#813-external-apicli-与-typescript-sdk-的适用边界)另区分 SDK／CLI 的 Agent 执行与 External API 的工作区数据管理，后者不是发送编辑 prompt 的入口。

当前依赖、范围和验收见 [v0.10 开发批次](docs/DEVELOPMENT_PLAN.md#117-v010-独立-cli-批次与完成标准)、[S0 与关闭门槛](docs/DEVELOPMENT_PLAN.md#118-issue-3-阶段映射规范资产与关闭门槛)，转换边界见 [§3.7](docs/DEVELOPMENT_PLAN.md#37-epub2--epub3显式可审阅的版本转换)。原 C1～C3 交付和 C4／U／A 设想保留为历史／后续参考；既有接口约束见 [CLI 契约 §2.1／§2.2](docs/CLI_CONTRACT.md#21-c1c2-本批实施契约)。原父 Orb 统一协调，Medium 子 Orb 编码，固定新树经真实 Droid demo／审查、修复复测和父独立验收；旧基线检查不覆盖新代码。本地实现、源码推送和版本化发行分别记录，发布另获授权。

除上面明确列出的命令、库与独立实验外，设计文档中的命令和接口仍待实现；不能将其他设计示例当作当前安装使用说明。
