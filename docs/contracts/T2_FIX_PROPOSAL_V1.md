# T2 第五批：FixProposal v1 与 ValidationDelta v1 冻结协议

2026-10-07 父审定。本文是 [CLI_CONTRACT §2.11](../CLI_CONTRACT.md#211-t2-第五批原生修复提案与诊断差异实施契约已冻结待实现) 的协议附件；冻结不代表功能已实现或验收。基线为本地 `bb1b46f`，不是 `origin/main`。规则仅有下述两条，旧 schema 1～6、操作参数、摘要与持久记录不迁移。

## 1. 命令与选择

```text
kepub fix propose --workspace DIR [--select REPAIR_ID[,REPAIR_ID...]] [--emit-request] [--output FILE] [--json]
kepub fix delta --workspace DIR --before REV (--after-revision REV | --after-task TASK) [--strict] [--timeout SECONDS] [--output FILE] [--json]
```

- `propose` 只读当前 accepted 基线，不运行或依赖 EPUBCheck，不写候选、accepted、history 或缓存。默认输出完整提案；`--emit-request` 输出内嵌完整提案与派生操作的 schema 7 request。
- 用户先读取 `repairs[].repairId`，再以 `--select` 在同一基线上重推提案。显式空值、未知或重复 ID 返回 `SELECTION_INVALID`／2。已知但不可修复项可供审阅，不产生操作；零操作提案有效，但 `--emit-request` 返回 `INVALID_OPERATIONS`／2。
- FR-2 可能损失作者 query 语义：普通提案可以列出它，但含 FR-2 操作的 `--emit-request` 必须使用显式 `--select`；默认 all 不构成该规则的选择授权，返回 `SELECTION_INVALID`／2。修复从不自动应用或接受。
- 无 `--output` 时 stdout 默认输出可阅读 JSON 文档；`--json` 则遵守既有单 envelope，文档在 data，不能将 envelope 直接当 request。`--emit-request --output FILE` 写的是原始 request 文档，供 plan 直接读取；stdout 可报告输出位置，但不混接文档与 envelope。`--output` 不是要求用户显式指定另一个 stdout 选项。
- 输出复用既有工作区外路径、真实父目录、不覆盖、无链接／特殊文件规则；已有目标 `OUTPUT_EXISTS`／2。失败不发布半截文件，不修改工作区状态；已有 Open 恢复语义不因此改变。
- `delta` 的 before 是已核来源的 revision（含 initial）；after 是同一工作区的已核 revision，或成功 apply 后、尚未结算的稳定任务候选。已接受任务改用 revision；漂移候选、失败任务、未 apply、未知身份不得作为成功快照。仅通过现有来源链／锁／私有快照读取，不接收任意目录、伪造报告或不稳定 live pub。

## 2. 提案字段与规范编码

字段必须符合下表；拒绝重复／未知字段与错误类型。数组空值使用 `[]`，不使用 `null`；唯一允许的 null 是 unfixable repair 的 `operation`。字符串为合法 UTF-8，不做 Unicode 归一化。所有 hash 为 64 位小写十六进制。

| 字段 | 冻结含义 |
| --- | --- |
| `proposalVersion` | 整数 1 |
| `workspace` | `{workspaceId, rootfile, baseRevision, inputTreeSha256}`；实际持久 workspace ID、选定 BookPath、workspace revision ID 与库存 SHA-256；不是 Git commit/tree |
| `rules` | 所选 repairs 使用的 `{id, version}` 集合，按 id 升序去重；version 为 1，零项时为 `[]` |
| `selection` | `{mode:"all"或"explicit", repairIds:[...]}`；all 的 IDs 为 `[]`，explicit 为升序去重的实际选择 |
| `derivedFrom` | 仅 explicit 出现，值为同基线完整 all 提案的 `{proposalId, proposalSha256}`；重算父提案，不引用外部文件 |
| `repairs` | 按 `(rule.id, rule.version, target.bookPath, target.locator)` 升序的适用事实；not_applicable 不生成 repair |
| `repairs[].repairId` | `sha256:` + SHA-256(UTF-8(`ruleId + "|" + decimalVersion + "|" + bookPath + "|" + locator`))；规范 locator 不含 `|` |
| `repairs[].rule` | `{id, version}` |
| `repairs[].basis` | `{checker:"kepub-native", checkVersion:1, fact, spec:{specVersion:"EPUB 3.3",section,normativeLevel}}`；不含上游 code 或易变运行数据 |
| `repairs[].target` | `{bookPath,resourceSha256,locatorVersion:1,locator,element,attribute:{namespace,name},expectedOldValue}`；元素 localName 与属性 expanded name，旧值为冻结 decoded 值，不是字节偏移 |
| `repairs[].status` | `fixable` 或 `unfixable` |
| `repairs[].operation` | fixable 时为既有 `{operationId,operationVersion:1,params}`，含完整既有参数绑定；unfixable 时为 null |
| `repairs[].risk` | 下述规则的固定英文风险文字，不受 locale／消息格式化影响 |
| `repairs[].unfixableReason` | 仅 unfixable 出现；确定性拒绝原因；不能以空操作冒充可修复 |
| `repairs[].readSet/writeSet` | 升序去重 BookPath 数组；语义依赖集而非 I/O 日志。readSet 至少含 container、选定 OPF、源资源，FR-2 再含目标／身份／引用门禁依赖。unfixable 的 writeSet 为 `[]` |
| `limitations` | `{rule:{id,version},bookPath,locator,reason}`，按 `(rule.id,rule.version,bookPath,locator,reason)` 升序去重；无可表达定位时 locator 为 `""`。外来 metadata、解析／来源覆盖不足明确列出，不静默视为未命中；不声称完整合规覆盖 |
| `derived` | `{operations,readSet,writeSet}`；全部所选项的依赖并集与 fixable 项的写集合／操作。操作按 `(params.bookPath,operationId,params.locator)` 排序，不删除重复以掩盖派生错误 |
| `proposalSha256` | SHA-256(删除顶层 `proposalId/proposalSha256` 后的规范 JSON) |
| `proposalId` | `kepub-fix-proposal-v1:sha256:` + proposalSha256 |

规范 JSON 使用 Go `encoding/json.Marshal` 的默认字符串编码和递归对象 key 升序（按 UTF-8 key 字节排序），无 BOM、缩进、空白或尾换行；先按上述规则规范数组。对象可以规范为 map 再 Marshal，不为本批创造另一套 JSON 字符串序列化器。字符串 `<`／`>`／`&` 与 U+2028／U+2029 使用 Go 默认小写 `\u` 转义，其余合法非 ASCII 保留 UTF-8；数字仅整数。新增规范化只用于本协议，不能改变旧 `digest` 或操作参数顺序。

稳定 hash 包含全部绑定、来源、风险、limitations、selection 与派生事实；排除顶层两个自身派生字段，不排除 repairs 的 repairId 或 derivedFrom。不含时间戳、随机数、elapsed、EPUBCheck 原始报告。CLI envelope／外层输出换行不参与 hash。零修复的 rules、repairs、derived 三数组均为空；limitations 仍保留真实范围。

### 2.1 编码 golden

以下是编码向量，不冒充真实库存或可执行 fixture；32 个 `1` 是合形的示例 workspace ID，64 个 `0` 是 SHA-256 占位，revision 使用 `initial`。实现还须有真实工作区测试，不能以 Git ID 代填。

```json
{"derived":{"operations":[],"readSet":[],"writeSet":[]},"limitations":[],"proposalVersion":1,"repairs":[],"rules":[],"selection":{"mode":"all","repairIds":[]},"workspace":{"baseRevision":"initial","inputTreeSha256":"0000000000000000000000000000000000000000000000000000000000000000","rootfile":"EPUB/package.opf","workspaceId":"11111111111111111111111111111111"}}
```

零修复 SHA-256：`fd80e386cdf6f100ad1197bb3f9b586929850f876d4af95974b2cbe4ce54f289`。字符串单元向量 `{text:"<>&é\u2028\u2029"}` 的规范 JSON 必须是：

```json
{"text":"\u003c\u003e\u0026é\u2028\u2029"}
```

字符串向量 SHA-256：`d8b50ca8fdf1304c00d2b04832474db2f3e739257ecb111a31d2995badd30203`。

单修复 all 的完整 hash 输入：

```json
{"derived":{"operations":[{"operationId":"xhtml.attribute.remove","operationVersion":1,"params":{"bookPath":"EPUB/ch1.xhtml","expectedOldValue":"secrecy","locator":"/html[1]/head[1]","locatorVersion":1,"name":"type","namespace":"http://www.idpf.org/2007/ops","resourceSha256":"0000000000000000000000000000000000000000000000000000000000000000","revisionId":"initial"}}],"readSet":["EPUB/ch1.xhtml","EPUB/package.opf","META-INF/container.xml"],"writeSet":["EPUB/ch1.xhtml"]},"limitations":[],"proposalVersion":1,"repairs":[{"basis":{"checkVersion":1,"checker":"kepub-native","fact":"xhtml-metadata-content-epub-type","spec":{"normativeLevel":"MUST NOT","section":"6.1.3.1","specVersion":"EPUB 3.3"}},"operation":{"operationId":"xhtml.attribute.remove","operationVersion":1,"params":{"bookPath":"EPUB/ch1.xhtml","expectedOldValue":"secrecy","locator":"/html[1]/head[1]","locatorVersion":1,"name":"type","namespace":"http://www.idpf.org/2007/ops","resourceSha256":"0000000000000000000000000000000000000000000000000000000000000000","revisionId":"initial"}},"readSet":["EPUB/ch1.xhtml","EPUB/package.opf","META-INF/container.xml"],"repairId":"sha256:2b44d78d9e2d1afcdc2c72204304afa97adb27d157ac9a89b1d43ef59a887976","risk":"removes an author semantic annotation that the frozen spec prohibits here","rule":{"id":"kepub.fix.epub-type-on-prohibited-element","version":1},"status":"fixable","target":{"attribute":{"name":"type","namespace":"http://www.idpf.org/2007/ops"},"bookPath":"EPUB/ch1.xhtml","element":"head","expectedOldValue":"secrecy","locator":"/html[1]/head[1]","locatorVersion":1,"resourceSha256":"0000000000000000000000000000000000000000000000000000000000000000"},"writeSet":["EPUB/ch1.xhtml"]}],"rules":[{"id":"kepub.fix.epub-type-on-prohibited-element","version":1}],"selection":{"mode":"all","repairIds":[]},"workspace":{"baseRevision":"initial","inputTreeSha256":"0000000000000000000000000000000000000000000000000000000000000000","rootfile":"EPUB/package.opf","workspaceId":"11111111111111111111111111111111"}}
```

单修复 all SHA-256：`40ec5378b55559eedb91c900526e6dc8dcd85fcf883758ac9976298bde11ad8a`。在此内容上将 selection 改为 explicit、repairIds 改为包含上述唯一 repairId，并加入由该 all hash 构成的 derivedFrom 二字段对象，explicit SHA-256 必须为 `b8dad7dc6fdb9d9bc941eef264271b3f2f3e407864674878f6337cd47daf8874`。两例输出均再加入自身 proposalId/proposalSha256，hash 输入不加入自身二字段。

## 3. schema 7 的完整来源与校验优先级

Request 恰有 `{schemaVersion:7,proposal:<完整 FixProposal v1>,operations:[既有 v1 操作]}`。仅允许两条规则派生的操作，不允许把任意旧操作附加到绑定请求。Plan／新执行来源通过可选字段携带完整 proposal，且仅 schema 7 出现；旧 request／plan／execution／journal／revision 外形、digest／policy 含义不改，不添加旧记录的必填字段。

schema 7 对应 execution version 7；仍嵌入原 Plan，不给既有 workspace/revision/task/settlement 外层重新编号。新 policy 使用既有 digest 算法计算字符串 `kepub-fix-proposal-v1:accepted-baseline;multi-operation;frozen-baseline;complete-source;locator-v1;reference-gate;explicit-selection;no-timestamp;review-required;conformance-not-run`，旧 policy 不变。沿用既有操作数量（256）与请求／XML／字节预算，不自动切批或裁剪；过多 repairs 可展示，emit-request 超既有可执行上限须拒绝，用户显式 select 重提。

1. 结构／类型／未知字段按现有严格输入规则拒绝。proposal 自身 hash/id 不自洽为 `PROPOSAL_DRIFT`／4。
2. 四项 workspace 绑定均核对；旧 proposal 的 baseRevision 或 inventory 已过期为 `PROPOSAL_DRIFT`／4。不把 Git SHA 当作 workspace 身份。
3. 在已核冻结输入重推完整 all 提案和 selection 派生提案，比较**整个规范内容**，包括规则版本、来源／规范、target、risk、limitations、read/write sets、derivedFrom 与全部派生操作；仅重签 hash 的篡改仍为 `PROPOSAL_DRIFT`／4，不只比较操作集合。
4. request.operations 必须与已重推的 derived.operations 是同一多重集：数量、元素、重复计数全同。缺项／重复／额外或参数变异为 `INVALID_OPERATIONS`／2。旧子集必须重新 --select，不能裁剪请求数组。
5. 随后沿既有事务路径逐声明核验 revision／resourceSha256／locator／旧值，不因 cache、相同路径、预加载或 no-op 豁免。实际资源／revision 漂移沿用 `INPUT_DRIFT`／4，定位／旧值失败沿用 `INVALID_OPERATIONS`／2；不为新 schema 改旧 schema 的错误分类。
6. 合法操作排列允许；`operationSetSha256` 仍是旧算法的**有序**数组摘要，两序摘要可不同，但同一事务的完整候选字节与写集合必须一致。真实区间冲突照旧拒绝。
7. plan/apply/checkpoint/restore/journal/diff/accept/history 均从已保存完整源和冻结基线重算。删除外部 proposal／request 文件不影响重开、审阅、恢复或历史核验。task diff 同时给出来源及**实际**目标旧／新值，不回显计划冒充候选。

上述 FR-2 显式选择限制也在 schema 7 校验中执行；手工把 all 提案写进 request 不提供豁免，返回 `SELECTION_INVALID`／2。

## 4. 两条规则（均 ruleVersion 1）

共同前提：实际 EPUB3 publication、所选 manifest 的 `application/xhtml+xml`、全部既有 workspace 编辑资格／只读理由与完整解析、祖先／namespace／来源／预算规则、唯一冻结 locator、精确旧值。只用已授权物理跨度和既有 v1 属性操作，不重序列化整个资源，不更改 namespace 声明、manifest、spine 或资源名。EPUB2 不套用 EPUB3 规范，不生成这两条可执行项，以覆盖限制标注；本批不隐式迁移或重评资格。不能写的适用事实为 unfixable；无法完成事实提取时记录 limitation，不虚报 not_applicable。提案中的每个可执行项及最终合并事务均须通过既有门禁。

### FR-1 `kepub.fix.epub-type-on-prohibited-element`

- EPUB 3.3 §6.1.3.1 `MUST NOT`；`fact:"xhtml-metadata-content-epub-type"`。XHTML 的 head 加 HTML §3.2.5.2.1 全集 base/link/meta/noscript/script/style/template/title，九个元素上的 OpsNamespace `type` 均适用，不按 EPUBCheck 实测子集缩限。
- 操作为 `xhtml.attribute.remove` v1；保留 namespace 声明（即使之后未使用）、其他属性／文本／资源原字节。允许位置如 nav/aside/p、无属性为 not_applicable；generated/default/nonwritable 来源为 unfixable。foreign metadata 或外来祖先重入不扩权，以 limitation 记录范围。
- 固定 risk：`removes an author semantic annotation that the frozen spec prohibits here`。
- 正反入口：九元素、允许位置、foreign、实体／默认来源、同值多处、三编码完整字节、定位／旧值／资源漂移。head 属性移除已在 v1 权限内，不混同于禁止删除 head 结构。

### FR-2 `kepub.fix.relative-url-query-component`

- EPUB 3.3 §4.2.5 `MUST`；`fact:"xhtml-relative-url-query"`。仅 XHTML a/area/link 的无 namespace href 和 img 的无 namespace src；不是任意同名属性。它是显式选取的标准化，不保证保存作者 query 意图。
- 仅相对 URL 的真实 query／ForceQuery；复用既有 URL 解析与 ASCII 边界空白约定。移除解码属性字符串中真实 query 区间，保留外侧 ASCII 空白、原路径／百分号拼写、RawFragment 和显式空 `#`；`#` 内 `?`、`%3F` 不当成 query，https 绝对 URL 不修。
- `xml:base` 或 HTML base 影响、network-path host、path-absolute、外来 namespace、不可写来源、删除后目标／fragment 无法唯一确认或既有门禁拒绝时，不生成可执行修复，不猜路径。目标存在／身份唯一与完整新链接事实照旧校验。
- 操作为 `xhtml.attribute.set` v1，仅编码授权属性值。固定 risk：`removing the query component can discard author-defined URL semantics; explicit selection and review required`。
- 正反入口：非空／空 query、空 fragment、`%69nner`／转义路径、fragment 内 `?`／`%3F`、真实 img、未知同名属性、absolute／base／foreign／缺失或重复目标、三编码完整资源字节。新值只删 query，不顺手归一 URL 或转码。

本批不做 OPF-086b/088 词表替换、RSC-009 warning 推测、ID 命名策略、alt/title 猜写、DOCTYPE 删除、CSS/NCX/SVG/OPF 修复。

## 5. ValidationDelta v1

新输出的 data 为 `{deltaVersion:1,before,after,entries,limitations}`，不修改旧 Report／Revision，也不缓存报告。两侧私有快照都实际执行相同 validation options 和固定 EPUBCheck 5.3.0；native 两规则事实单独提取，不冒充 RSC-005/RSC-033 或解析上游消息文本。

- side 为 `{snapshot,checks,report,reportHash,nativeDiagnostics}`。snapshot 绑定 `{workspaceId,rootfile,baseRevision,inputTreeSha256}`，revision 侧另有 revisionId，task 侧另有 taskId 与 executionSha256（既有 execution 的 digest）。候选代际由 task／execution／实际库存绑定，不伪造尚未实现的 preview generation。
- report 原样保存现有完整 validation.Report（含 rawReport／执行记录）；reportHash = SHA-256(现有 Go 默认 `json.Marshal(report)` 字节)，不是 proposal hash，不剔除 elapsed，重跑不保证同 hash。
- checks 是实际 checker 元数据数组，每项有 `{checkerId,toolVersion,toolSha256,ruleset,specBaseline,profile,flags,inputHash,configHash,reportHash,runStatus,coverage}`；未知值写 `unknown`，coverage 保留来源与范围，不以 JSON 完整充当全面覆盖。native 使用 kepub-native/checkVersion 1 和这两条规则，upstream 保留真实 code/severity/resource/line/column/message 与完整报告；nativeDiagnostics 每项为 `{source:"kepub-native",checkVersion:1,rule,fact,severity:"error",target}`，rule/target 形状沿用提案定义。
- native coverage 为 `{scope:"manifest-xhtml-native-fix-v1",status:"complete"或"partial"或"not_applicable",limitations:[...]}`；范围与 EPUB3／权限／来源限制如实绑定。native 元数据 reportHash 另为 SHA-256(规范 JSON `{coverage,diagnostics:<nativeDiagnostics>}`)，不能用不含 native 事实的 validation.Report hash 冒充其报告绑定。upstream 元数据 reportHash 绑定实际完整 validation.Report。
- runStatus 区分 `completed`（含正常合规 FAIL）、`not_run`、`blocked`、`unavailable`、`failed`（执行／报告错误）、`incomplete`。两侧都尝试运行；缺 Java/checker 不自动安装，不冒称 PASS。原验证 fatal/error/strict warning 与 draft=false 原义不变。
- entry 为 `{classification,source,identity,before,after,reason}`；before/after 是实际诊断实例数组，不只计数。classification 为 `resolved`、`persisted`、`added`、`upgraded`、`newly_checkable` 或 `incomparable`。按**多重集**匹配，A 消失与 B 新增不抵消，重复实例保留，severity 提升单列。
- identity 必须以真实 checker/source、code 或 native rule/fact/version、BookPath、可证明映射的 locator/attribute 与冻结位置推导；不凭相同 code、标签或消息猜原因。native oldValue 变化不自动创建另一个事实身份。消息可原样展示，不能解析其自然语言以冒造子码；没有可靠身份或唯一归属的 generic RSC-005 是 incomparable，不借 native 目标消失宣告上游已解决。
- 工具／规则／参数／profile 或覆盖不一致、not_run/blocked/缺失、范围缩小、改名／拆章等缺映射时，不分类 resolved／upgraded 或计算改善率；新进入可靠覆盖范围单列 newly_checkable。unknown 不能被填成已证实规范基线，可比性只能来自其他实际冻结证据，不能以两个 unknown 相等自证。
- 合规失败但报告正常完整时，Delta 可成功输出解释（exit 0 不意味着书合规）；检查器缺失／执行失败／timeout／报告不完整时保留 data，沿用既有验证 fault/exit，不吞错误。两侧可能失败均如实记录，不能拿成功一侧替代另一侧；unsafe/input drift/I/O 仍拒绝快照或输出。
- Delta 不生成批准、不改 accept/export 门禁；任务接受仍重新运行真实正式检查。多重集、稳定位置映射、覆盖改变、同标签另一处 RSC-005、工具变化与 unavailable 必须有独立分类 oracle。

## 6. 撤销、验证与资产

apply 前选取子集重推完整新来源；apply 后 accept 前撤销用 task reject **整批**回滚，再从 accepted 重提。两规则均删除型，其逆会重新引入违规，不生成自动反向修复、不删执行日志、不改已接受历史；独立规则选择不冒称依赖修复覆盖。

需永久回归、独立字节 oracle、合法两序与逐字段变异（含重新签 hash 后伪造 basis/risk/sets/workspace）、完整生命周期与外部文件删除后重算；有限菜单先校准真实接纳／必达，再 live fuzz，合法族误拒立即 FAIL。共享事务增量按根 AGENTS 依序普通全仓→race全仓→vet，另旧 schema／冻结回归／focused race／真实 CLI／固定 checker／ZIP 文件及目录 oracle。旧树 red 必须是实际行为断言失败，不是新增 API 缺失导致编译失败。

UTF-8／UTF-16LE／UTF-16BE 完整字节正控是 native Plan+Apply 层；固定 EPUBCheck 5.3.0 对 UTF-16 XHTML 的 HTM_058 必须仍正式拒绝，不称三编码 strict 全通过，不隐式转码。字体资格／旧工作区重评、OPF/spine、资源增删改名、NCX/SVG/CSS 仍属剩余 T2，不进 T3。

HTML metadata 定义资产使用已取得的 dom.html，439903 B／SHA-256 `c830abc1b4bf25a516a64af1bf5258b381d0a437cd5d9411501cecb4a1f90c41`，不得用重抓的 Living Standard 冒充同输入。归档到 `docs/specs/epub-3.3/batches/t2-fix-proposal/` 的 original/text/manifest/ATTRIBUTION，原 S0 文件、manifest、INDEX 与 hash 不改。批次同时保存 [WHATWG IPR §7.1.1](https://whatwg.org/ipr-policy) 与 [CC BY 4.0 legalcode](https://creativecommons.org/licenses/by/4.0/legalcode.txt) 的原始许可证据和独立 hash：文档 CC BY 4.0、纳入源码的上游片段 BSD-3-Clause（无此类复制则不伪造声明），不改项目整体许可。注明 WHATWG（Apple、Google、Mozilla、Microsoft）、来源／日期／raw 未修改及 text 摘录修改方式、许可和免责声明；抓取／核验／归档执行者分开记录。
