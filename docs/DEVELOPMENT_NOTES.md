# 开发注意事项：T2–T6 的持续回归约束

本文件保存**稳定规则与失效模式**；行为以 CLI_CONTRACT 为准，接受/拒绝以固定输入的独立审查和父验收为准。AGENTS.md 自动约束目录，skill 在实现/排错/复审时加载；三者都不是能强制证明正确性的工具。

## 已确认的重复模式

2026-10-07 核对编码线程、[T2 验证记录](verification/T2_MULTI_OPERATION.md)和第四批冻结 receipt：存在同类边界遗漏，没有证据证明已修的相同代码分支被回滚后原样复发。

| 证据链 | 失效模式 | 不能再使用的简化 | 稳定规则 |
| --- | --- | --- | --- |
| 第二批 ff64188 → f295f3f → 6f9980e → b301fb9；79a4548 接受 | 资源局部门禁、漏新 IDREF、布尔 removed、同节点逐属性 delta | 单操作/局部计数代替最终 identity 图 | G-FINAL、G-FACTS |
| 第三批 e8614fb → b9840cc；d747c01 接受 | token 级来源、generated 词法/decoded 混淆、空实体边界、私有长度限制 | “decode 相同即可写”或整 token 连坐 | G-PROVENANCE |
| 第四批 9f3d1c3 被父/medium 拒绝 | cache 命中漏端点 hash、self/空 query 丢失、未改写 URL 漏 gate | “同资源已验证”“不重写就无新事实” | G-BINDING、G-URL、G-FACTS |
| 多批非产品记录纠正 | working-tree/fixed-tree、执行者、fuzz/race/平台覆盖措辞混淆 | 命令退出成功代替批准或覆盖证明 | G-ORACLE、G-EVIDENCE |

第四批证据：固定 commit `9f3d1c331b4d79c9d1be4a5fdbe5027e8336947c` / tree
`23a73d77ec0057ba6548530607d2bb751b036444`，medium 封存包 505592 B，SHA256
`1fd7a62854275c7dfa0028dd7964c074c0e15d7f571800589503d8c35f208dcd`；父核清单 98 项全部 OK。12 个阻塞负子例普通/race FAIL 与全仓普通/race/vet PASS 同时成立。此处引用既有封存证据，不声称重跑。后续新树修复不改写这次拒绝；此记录不扩大到完整 T2、T3–T6、Mac 或断电。

## G-FINAL：最终状态的单位必须正确

- 全事务统一收集后判定，不因资源分组或操作顺序而变。
- 同节点所有冻结 `id/xml:id` edit 合并后求不同值集合；不同节点同值为不同实例。同值 alias 在同节点只计一次，不用 per-attribute sum 或 removed 布尔。
- 冻结入站与新 URL/IDREF 使用同一最终计数；同值 re-add、合法 partial delete/swap/transfer 与重复碰撞分别测试。
- 入口：`publication.MergedIdentityDelta`、`workspace.recomputeStructure/structureGate`；永久回归：`TestStructureJointAliasEdits`、`TestStructureJointAliasAdjacentCases`、`TestStructureIdentityMultiplicityModel`。

## G-FACTS：所有进入候选的事实都要过门禁

- 属性 edit、fragment 和 move 中的 identity、IDREF、href/src 都必须进入共同模型，包括值未改写、外部 URL、无 identity 的块。
- coverage 不足拒绝危险写，不能把没提取到的引用当不存在。诊断身份/位置来自承载属性，不按任意属性的相同值定位。
- 正反配对：https 允许、javascript/data 拒绝；内部目标存在/缺失；新旧 IDREF 目标最终唯一/缺失/重复。不靠其他 identity 分支偶然挡住遗漏。
- 入口：共享 IDREF 词表、`references.Graph.CertainIncoming`、`publication.IsIDAttribute`；回归：`TestStructureNewIDREFGate`、`TestIdentityDiagnosticAttributeSource`。
- 第四批固定树 5e508f3 的永久入口为 `TestStructureCrossMoveNewLinkGate`；旧 cross new-link 冻结 red 探针继续保留。medium 已有限接受产品修复，并建议永久例点名 unchanged src（https 正控、javascript/data 负控），不能只靠 href 覆盖声称 src 已有永久控制。测试存在及定向 green 不代表独立整批批准。

## G-BINDING：缓存性能不改变显式输入契约

- 每次使用每个显式 source/destination 都核其契约声明的冻结 revision/hash/locator/旧值，不改变旧 schema 的字段要求。缓存只保存可复用解析结果/冻结 bytes，不能记“该路径已通过，所以后续声明不查”。
- 同一资源至少两个不重叠操作；把坏绑定分别放首次/缓存后、source/destination、正反序，核拒绝码和无产物。全部合法绑定正控必须成功 Plan+Apply。
- 既有 `TestRequestsAndPlansRejectTampering` 是起点，不替代重复端点测试；第四批固定树 5e508f3 的永久入口为 `TestStructureCrossMoveEndpointHashBinding`。固定树 9f3 的 `reviewer_medium_cross_binding_test.go.received` 是冻结 red 证据，不能修改为弱期望。

## G-PROVENANCE：来源、语义与权限分别推导

- 对齐所需 decoded 语义不等于原实体词法拼写；对齐成功也不授予 generated/default/unknown 可写权限。禁止跨非 writable fragment，即使该 fragment 解码长度为零。
- 每个字面 fragment 自有物理 byte span；合法左右邻接单独可写，不吞引用、不修改无关字节。CR/CRLF、BOM、UTF-16 代理对和 reference CR 按解析层职责处理，不二次归一/重复 decode。
- Regex 在真实输入中发现任意零宽 match 就整体拒绝；不能仅 `MatchString("")` 或静默略过。目标合法性沿完整祖先路径到允许根，外来 namespace 重入仍拒绝。
- 未定义的长度捷径不是预算。合法长 numeric reference 保留；不能为通过测试扩大 unknown/CDATA/生成来源权限。
- 回归：`TestTextRunsKeepsWritableLiteralIntervals`、`TestReplaceZeroWidthAndProvenance`、`TestReplaceGeneratedDecodedNeighbors`；原 minseed `267ee5f836be007e` 与 joint `21d0b33faf043b04` 原样重放。

## G-URL：重基准不是丢弃 URL 分量

- local/self/other-resource/incoming 路径都核 RawQuery、ForceQuery（显式空 `?`）、fragment、转义路径；仅改契约许可的部分，不改其他属性/文本或外部 URL 拼写。
- 区分 `#id`、`?q=1#id`、`source?q=1#id`、`other?#id`；literal/regex 不代替结构 URL 解析。
- 真实跨资源候选比较完整 source/destination/nav/third-resource bytes；URL无需重基准仍服从 G-FACTS。
- 第四批 `reviewer_medium_cross_query_test.go.received` 与父 query 探针冻结 red；固定树 5e508f3 的永久入口为 `TestStructureCrossMovePreservesQuery`，而不是只核新的 plan 文本。

## G-ORACLE：测试必须区分正确与看起来正确

改变一个风险模型前先写：**可能的错误实现 → 与正确实现不同的输入 → 独立期望 → 实际断言**。

| 风险 | 最小有区分力的病例 |
| --- | --- |
| 局部门禁/别名计数 | 同 locator 多属性联合改、重复基线、合法 partial delete、两序、既有+新增 href/ARIA |
| cache/绑定 | 同资源多操作、首次/缓存后坏 source/destination SHA、两序合法正控 |
| 来源/替换 | 同值 literal+generated 的第二命中、直接/嵌套/空实体、59/60/61/80/更长 numeric、真实零宽和非空锚定 |
| URL/gate | self query、空 query、未改写 https 与 javascript/data、无 ID 块、源外/nav/第三资源入站 |
| 字节/位置 | UTF-8/UTF-16LE/UTF-16BE、BOM/CRLF/代理对/实体/引号、重复相同块与各插入位置 |
| 事务/历史 | no-op 空 write set、部分资源写失败、restore leftovers、再次 Open、候选 drift 拒绝 accept |

只选本次会被改动的相关维度，不机械跑所有笛卡尔积。expected output 用冻结 fixture 加独立标准库编码/显式作者期望构造，不从 planned spans/candidate/product helper 导出。最终完整 bytes 与图状态是证据；Contains、候选可读、WriteSet 相等仅是辅助。

有效事务 fuzz 先重放全部 seed，再 live；输入生成保持合法，意外拒绝立即失败，不跳过；两序比较实际所有候选资源，而非只比 plan。负例不能掩盖正例误拒。

- **先证明生成器和断言可达：** 有限菜单先确定性校准接纳数，其他生成器至少按输入族校准代表 seed。每个声称覆盖的合法族须实际进入 Plan+Apply 与独立 oracle；有比较代码或高 exec 数不证明比较执行。
- **接纳与拒绝分开：** 合法单/双操作 Plan 错误不能 `if err == nil` 静默略过；负族明确核预期拒绝原因。两序均拒绝只证明拒绝对称，不证明候选顺序独立。
- **锚点必须仍在：** 合法双移动用不重叠源和未被同事务删除的目的锚点，至少一个 seed 必须走完两序 Apply/完整资源字节比较。原 overlap 形状保留为负控，不能放宽产品门禁补生成器缺陷。
- **观测集合不能来自实现：** 由 fixture/契约指定要核的候选资源，不只遍历产品 WriteSet；未写 nav 也在 Reject 前读实际字节，两序都与独立冻结期望比较。两序同样错误仍可能通过相等比较；负控须核预期 code 与可识别根因，不只要求两序同码。

本规则补充源于 5e508f3 的永久 `FuzzCrossMovePlanApply`：第一移动固定围绕 `three` 插入，第二移动又删除 `three`，按区间门禁所有双移动都重叠。medium 独立菜单校准为单移动 16/16 接受、双移动两序 0/32 接受；父只核对源码因果，没有重复该枚举。该发现是测试/覆盖记录缺口，不是新的产品根因，也不能用历史 1,558 live execs 声称“两序候选比较已执行”。未修文案或测试时须保留此限制，固定产品树不得原地改动。

维护入口：`TestCrossMoveFuzzCalibration` 校准合法单/双移动与 overlap 拒绝，并证明两序候选比较实际执行；`TestStructureCrossMoveNewLinkGate` 区分真实 img src 与 a href 的正反控制。树外首版虽校准通过，仍按 WriteSet 漏读未写 nav、只核两序同码；v2 改为三资源无条件读取和明确拒绝原因，说明接纳数通过不能代替断言完备。维护补丁须随新身份独立验证，不能承继旧产品接受或 scratch PASS。

输入身份、包/日志 hash、校准耗时和执行者写入 [T2 验证记录](verification/T2_MULTI_OPERATION.md)及对应封存清单；本文件只留可复用规则、失效模式和回归入口，不累积聊天或运行日志。

## G-QUALIFY：事实、来源与真实旧门禁资格分离

- 全 Tree 的路径被扁平为“存在” → 目录 FR2 被声明 fixable、真实 plan 文件门禁拒绝；
  HashTree 错误被丢弃 → 无 hash／库存的貌似完整快照 → 保留 typed 库存，仅普通文件参与
  资格，共用只读 `CheckReferenceTarget`，错误原样传播。`TestN3DirectoryTargetQualification`、
  `TestN3TypedInventory`、`TestN3FileTargetMatrix`、`TestN3SnapshotHashError` 覆盖三编码／两快照
  入口／无与唯一、缺失、重复 fragment／中文和百分号。完整 Tree 与空目录不删。
- 新资格重推旧 schema 7 的 derivedFrom → 合法旧已消费来源丢失 → 新 policy 显式 file-target-v2，
  已消费旧 policy 只作原语义完整来源核验，不作新接受授权；`TestN3LegacyTargetSource` 和
  `TestN3FixTargetBinaryLifecycle` 保留历史及真实 checker／空目录正式导出边界。
- PK1 残余消费边界：仅在 accepted revision 重推 schema 7 → rejected history/settlement recovery
  接受一致重签的伪 risk → 共用 `taskDigests` 重推自身 BaseRevision；永久入口
  `TestFixRejectedSourceRevalidation`（含 accepted 推进后的合法旧历史和归档中断恢复）。
- severity 配对一次消费整组、native 两边非空就全 persisted → 非对称 surplus 消失或复用已配对实例 →
  equal severity 先配对、剩余切片逐实例消费；永久入口 `TestClassifyDeltaSeverityMultiplicity`、
  `TestClassifyDeltaNativeMultiplicity`，保持 generic/coverage 既有不可比较规则。
- FR2 空新值快捷跳过、仅 fragment 成功才收目标依赖 → `?x=1`／`?` 漏 repair/native fact、无 fragment
  漏 readSet → `TestFixRelativeURLQueryBytes`：三编码、七资源完整字节、旧 schema 4 实际 Plan+Apply
  正控与 schema 7 消费；不修改外侧空白的真实旧 gate 资格或原强删除 oracle 的历史 FAIL。
- **规则→失效模式→永久回归**：保存的 schema 7 来源只在 Plan 时重推 → Apply/执行/恢复/diff/accept/history
  信任 digest 相等的伪造来源（改 risk 重签、裁 operation 并重算 ordered digest）→
  `TestFixSavedSourceRevalidation`、`TestFixDeltaFrozenSide`、reviewer 的 `TestMediumFixSavedSourceRevalidated`。
- 历史快照复用当前 accepted 的 inventory → initial 侧声明当前 tree → `TestFixDeltaFrozenSide`、
  `TestMediumFixHistoricalSnapshotIdentity`。live 目录直接作为 checker 输入 → native facts 与报告 tree
  不一致 → `TestFixDeltaFrozenSide`、`TestMediumFixDeltaSnapshotDrift`（owner 窗口边界，不称公共 CLI 并发）。
- 规则只看当前节点 namespace/不查真实 gate/不查只读 → foreign 祖先重入被改写、外空白 URL 宣称可执行、
  SIGNATURES 只读 workspace 宣称可执行 → `TestRuleForeignAncestry`、`TestFixReadOnlyQualification`、
  `TestRuleRelativeURLQuery`（外空白 = 明确 unfixable）、`TestMediumFixReadOnlyQualification`。
  资格必须由真实既有 gate 判定，不做全局硬拒绝，也不放宽旧权限。
- Delta 分类不按实例多重集/不比真实 tool/config/coverage → 重复 2→1 全 persisted、tool/config/覆盖变化仍
  resolved、generic RSC-005 被 upgraded → `TestClassifyDeltaInstanceAndComparability`、
  `TestMediumDeltaMultisetAndCoverage`；真实 tool SHA-256 与未知 profile/flags 显式 `unknown` →
  `TestFixDeltaFrozenSide`、`TestMediumFixDeltaMetadata`。
- 发布边界：已存在输出统一包成 `INVALID_OUTPUT`、emit-request 不做预算 → F01/F10 →
  `TestFixOutputExists`、`TestFixEmitRequestBudget`、父 `b5-parent-output-exists-repro.py`、
  `medium_b5_budget_oracle.py`（256 成功 / 257 `INVALID_OPERATIONS`/2 无产物）。
- 外空白 FR2 的原始 bytes 探针恒要求删 query，强于"门禁拒绝→unfixable"的冻结 §4；按 review 资格解释
  校准：15 个可执行族继续两序完整 bytes，三个外空白族为明确 unfixable 负族，原 source/FAIL/seed 原样保留，
  不改弱、不删菜单、不称 18 成功。

## G-ASSET：派生资产元数据绑定

- 派生 text 的 bytes 与 SHA-256 必须由实际文件生成并核验；登记值漂移（1475 vs 1515）→ `text/extract.py`
  重生成后校验 manifest/ATTRIBUTION 全部行，不一致 exit 非 0。只修派生元数据/生成流程，不改 raw 原文、
  S0 身份或历史 manifest。

## G-JSON：持久化成功必须可在同一预算内读回

- 无预算 writer＋32 MiB reader → 合法多资源长路径的 state 可以创建成功却不能重开 →
  同类持久记录共用 `maxJSONBytes`／`checkJSONSize`，按实际 UTF-8 JSON（转义、结构、末尾 LF）计量，
  超限在临时文件创建前返回 `WORKSPACE_JSON_LIMIT`/1；不截断库存、不提升 reader 上限。
- Create 只返回内存状态 → 发布前未证明落盘元数据可解析 → state 经 Open 同一读取入口读回，
  identity 沿 ensureIdentity 读回；失败只清理本次 staging，不覆盖原书、已有目标或其他工作区。
- 原始字符串长度不能替代序列化长度；小预算逐调用注入而非修改全局。永久入口
  `TestR1JSONByteBudget`（独立字面期望、UTF-8／转义／LF、预算±1）、
  `TestR1LegacyRecordByteBudget`（旧字段顺序／optional shape）、
  `TestR1CreateLongPathInventory`（合法完整库存、预算边界、实际 Create→Close→Open 与资源字节）。
- 真实短写使用隔离子进程的几 KiB 文件预算，不耗尽磁盘／内存；永久入口
  `TestR1CreateMetadataShortWrite` 核只有 state 超过文件预算、无目标／staging 残留、原书与旧工作区不变。
  `TestCrossProcessCreateRace` 核唯一完整可重开的获胜目录、所有文件字节和原书；不将注入当断电。
- 所有 plan/execution/checkpoint/revision/journal 沿同一 writer；规范 digest 编码不带 LF 且保持不变。
  外部 edit 请求继续原有 32 MiB 与参数错误分类；正式 accept/export 仍使用真实固定 checker。

## G-INVENTORY：入口与后续消费使用相同展开库存预算

- ZIP 仅数显式条目 → 深路径隐式父目录不受限且后续扫描拒绝 → Open 在 privateDir 前登记全部唯一
  文件/目录，共享和显式父目录去重，但重复显式 ZIP 条目仍拒绝；不把工作区管理目录算成出版物。
- 文件字节不能约束路径库存 → 按唯一精确 BookPath 的 UTF-8 字节累计 `PathBytes`，加法前用剩余
  预算比较，避免溢出；ZIP 与目录扫描复用 `Limits.CheckEntry`，不截断资源或目录。
- Inventory 重置 DefaultLimits → 注入/入口预算在后续失效 → Archive 保留自身 limits，Unpack 发布
  staging 前验证，Inventory/WriteZIP/PublishZIP 最终重开沿用；正式 checker 不被草稿控制替代。
- 永久入口 `TestR2ExpandedEntriesBeforeStaging`（两个文件＋隐式父目录，阻断 temp 创建的正反控）、
  `TestR2InventoryRetainsEntryBudget`（实际 Inventory/Unpack、无产物/残留）、
  `TestR2CumulativePathBytes`（UTF-8、显式父目录两序、路径预算±1与共享/空目录）、
  `TestR2IndependentBudgets`（四种预算分离、int64 边界）和 `TestR2ExpandedRoundTrip`
  （Open→Unpack→Snapshot→Inventory→WriteZIP，独立完整库存/hash/资源字节）。

## G-IO：工作量有界但持久决定不能丢失

- 先无界复制原书／hash 工作区再拒绝 → 临时字节和持锁工作量不受展开预算约束 → 原 ZIP
  独立 `MaxInputBytes`，出版物复用 archive 四预算；实际读取仅余量 + 1，stat 快拒后仍核
  增长和来源。管理目录／JSON 不算成出版物，hash wire 不变。入口 `TestR3OriginalBudget`、
  `TestR3TreeBudgets`、`TestR3GrowthAfterStat`（13 字节增长、8/16 预算分别超限／drift）。
- WalkDir 先整目录 ReadDir → callback 尚未核预算就分配目录列表 → 逐条遍历，在下钻、
  库存追加和目标创建前核条目／路径；恢复 no-follow、单链接与碰撞负控，不缩减真实库存。
- 读到探测字节就覆盖真实 read fault／cancel → 错误分类失真 → 保留 I/O／短写／取消，
  仅无其他错误的超限判 `ARCHIVE_LIMIT`。`TestR3ActualReadBudget` 与 `TestR3BoundedCopyFaults`
  独立核读取计数、bytes、int64 上界、读故障＋探测字节、短写和 EOF 取消。
- accept／第二资源写入前未观测取消 → 无用 staging 或额外写入 → 沿请求 context 传递；
  已登记写入完整 checkpoint 回滚，有效 restore／settlement journal 只忽略迟到取消，
  不跳预算／来源／I/O 或删除决定。`TestR3CancelledAcceptDoesNotCopy`、
  `TestR3PartialTreeFailure`、`TestR3CancelledRunningRollback`、`TestR3CommittedRecoveryRetainsBudget`
  核第二资源、全资源 bytes、两次真实权限恢复失败后的 journal、再次 Open／锁获取；
  app `TestR3WorkspaceRequestCancellation` 核九入口 code/130 和 source／workspace／输出不变。
- task diff 把全部快照错误降为 unavailable → 复制中取消仍成功返回部分 review →
  取消／超时／预算错误保留失败，普通不可解析候选仍允许安全审阅；
  `TestR3DiffSnapshotCancellation` 核真实取消窗口／全工作区 bytes／私有 stage 清理。
- 共用读取 helper 的通用预算文本替换归档既有 expanded-bytes 文本 → R2 单文件／累计
  正确超限却违反旧原因契约 → Open 保留归档具体原因，`TestR2IndependentBudgets` 原断言不改。
- 仅在系统调用间检查 context，不宣称即时打断任意阻塞 I/O；小预算／注入不等于真实耗尽、
  公共 CLI 并发攻击或断电。正式 accept/export 仍用真实 checker，旧 schema／摘要不变。

## G-REPLACE：拒绝前的分配也必须有界

- 无限 FindAll＋先 ExpandString／ReplaceBytes／拼接后查长度 → 小捕获模板在最终拒绝前放大。
  命中与元数据先限量，展开按 Go 模板规则预计量，转义由 xmltext 按原编码计量；
  `expectedHits` 仍是精确冻结绑定，不是报错后的内存控制。
- 每节点重置预算 → 多节点／操作／资源绕过累计限制 → `recomputeStructure` 为全事务
  共用一份 `ReplaceBudget`；每次 Plan／Apply／后续来源重推重新初始化，不添持久字段。
  最终资源先扣全部合法 disjoint removals 再加新字节，拼接前拒绝，原书／accepted／审计不变。
- running／无 result 的恢复先重算即返回预算错误 → 已写第一资源留在候选 → 分离静态
  envelope／保存来源核验与效果推导；完整核 used/start/checkpoint 后沿原事务回滚、记录
  失败并返回 RESOURCE_LIMIT。回滚不证明效果，不豁免失败／历史重推；未开始、已完成
  或伪造来源不改写。`TestN1PostStartBudgetRecovery` 与 `TestN1BudgetRecoveryBoundaries`
  使用 10→9 字节 owner 窗口、全七资源／审计字节、再次调用及合法历史控制；不称公共并发攻击。
- 估计展开不得偷改语义：`$0`、编号、最大名称、`${name}`、缺失／未参与组、`$$`、畸形 `$`、
  前导零数字名称和重复命名组均以 Go 标准库独立 oracle 对照；加法用剩余量防溢出。
- 等值替换不能把 CRLF 等原拼写规范化；仍核真实命中与可写区间，不把 no-op 当权限豁免。
- 永久入口 `TestN1ExpansionBeforeAllocation`（1 KiB 预算、旧实现实际展开 4 KiB），
  `TestN1HitAndMetadataBudgets`、`TestN1TemplateGrammar`、`TestN1CumulativeExpansion`、
  `TestN1EncodedAndResultBudgets`、`TestN1NoopAndZeroWidth`、`TestN1TransactionBudget`、
  `TestN1ResourceBudget`、`TestN1ApplyBudgetRevalidation`；三编码完整 bytes、七资源 oracle、
  失败前后全 workspace hash 与实际分配入口计数。不以字符串长度／计数／benchmem 冒称峰值内存。
- `FuzzN1BudgetTransaction` 固定 UTF-8 两资源、literal／regex × 四种替换的八病例菜单，
  精确累计预算下每次必须实际 Plan+Apply+Reject 并核七资源独立 bytes；拒绝不 continue。
  `FuzzN1ReplacementSize` 的模板计量 oracle 单列，不冒充全事务／全编码 live 覆盖。

## G-IDREF：字面身份与 URL 是不同语法

- `"#"+token` 经 URL 解码 → p%41 误指 pA、字面百分号被拒绝 → IDREF 直接构造同文档
  Reference，再进入共用存在性／歧义处理；普通 href/src 继续 URL 解码。edge.href 留完整原值，
  target.fragment 留单 token，来源 locator 留属性名。括号 IDREF 不是 XPointer。
- Unicode Fields → NBSP 身份被拆；全部属性按列表 → 单 IDREF 的非法多值被两个合法 ID
  掩盖 → publication 共享 tokenizer 按宿主／冻结属性类型仅拆 ASCII 列表，单值原样保留，
  output.for 是列表。既有身份读取不放宽新建 NCName、namespace 或来源权限。
- 只改 inspect → 属性／片段／搬入门禁与既有入链不同 → 三入口和图共用版本，完整事务
  最终状态门禁不变。永久入口 `TestN2LiteralIDREF`、`TestN2IDREFValueTypesAndIdentity`、
  `TestN2ExistingIDREFProtection`、`TestN2NewIDREFValueTypes`、`TestN2CrossMoveIDREFTypes`。
  三编码独立七／九资源 bytes 与百分号/NBSP/中文、URL正控、alias/重复身份均分别核验。
- 静默用新 parser 重推旧 policy → 旧批准含义改变或合法历史无法恢复 → parserVersion=2，
  新 schema4–7 策略追加 reference-parser-v2；旧未消费计划拒绝，新 accept 不沿用旧审核。
  已消费旧 task/history/journal 显式 v1 重推完整来源，旧 durable 决定恢复完成、既有 accepted
  正式导出保留，不重写摘要。`TestN2LegacyReferenceLifecycle`、`TestN2LegacySchemas`、
  `TestN2LegacyAcceptedJournal` 保留语义不对称控制和原记录 bytes；CLI 的
  `TestN2IDREFBinaryLifecycle` 核真实查询／plan拒绝与真实 checker 接受／导出完整库存。
- `FuzzN2IDREFTokens` 只验证合法构造 token 输入与独立 token 序列期望，不称全事务 fuzz。

## G-GRAPH：单资源有界不等于全书有界

- 只限每个 XML、全图累计 append/map 无预算 → 多个小资源放大边／实例／诊断／coverage →
  所有 Build/BuildSource 入口共用逐调用 GraphLimits，读资源和增长对象前收费，原始扫描、
  计数、索引及保守序列化预留分别限制。重复字符串／位置仍计，Filter 不减少扫描范围，
  过滤 header 也预留；不把预留字节或 benchmem 当 RSS／OOM 实测。
- reader 已取消仍逐资源 blocked 并继续 → 三次读取且得到可消费图 →
  `TestR4CancelledGraphStopsReading` 冻结旧树三次／新树一次，返回错误与零图；
  `TestR4CumulativeGraphCounts` 核两个合法小 XML 及各预算±1、重复实例／missing／外链／未知。
- 构建失败只返回空边 → identity/cross-move 门禁误证明无入链 → 明确失败贯穿 inspect、
  validation、结构来源重推、diff／history，零图 CertainIncoming 有 global blocker；
  `TestR4DependencyPlansRequireCompleteGraph` 核三种操作 Plan/Apply 失败全工作区不变与
  合法七资源完整 bytes。正常 parser v1/v2／schema shape／policy／digest 不迁移。
- running 重算图限制未归入已登记失败 → 部分编辑留在候选 → 沿已核 checkpoint 回滚后
  仍返回 REFERENCE_LIMIT，`TestR4InterruptedGraphBudgetRollsBack` 保留真实失败 diff。
  正式 accept/export 失败不推进 accepted／发布输出；workspace/app/validation/CLI 的 R4
  正反例仍用真实固定 checker，内部小预算不进入持久 checker config 或新增 CLI 开关。
- `TestR4ReservationBoundaries`、`TestR4FilteredResultHeaderBudget`、
  `TestR4XMLCoverageReservation` 与 `BenchmarkR4BoundedInventory` 核分配／追加前拒绝。

## G-EVIDENCE：验证身份和范围不迁移

- 每组结果注明 input commit/tree 或 working-tree 身份、runner、命令/timeout/parallel、exit、skip、artifact hash；fixed fresh-fetch 不能继承作者工作树 PASS。
- CLI 子 binary 的 build flags 单独记录；固定 checker 与依赖 checksum 实核。ZIP oracle 包含目录；缺 checker/跳过不能记正式合规 PASS。
- full suite PASS 不抵消冻结/live FAIL；进程 exit0 不等于审查批准。修复变动后冻结新树，独立 reviewer 与父按同输入验收，不在待审树中边审边改。
- 保留旧 FAIL、环境/复制/测试 oracle 错误和纠正记录；manifest 从清单所在目录核验，bundle snapshot 大小/hash再次核准。
- 仅终端可见而未落盘的执行标为无原日志，不据说明文字声称已经封存，不补造或重跑冒充旧执行；首版 live 原日志不能迁为后续补丁 live。
- 故障注入和两次 Open 不等于真实断电或未执行的恢复再次中断；交叉编译不等于 Mac 实机。本地 commit/集成不等于 push/release；计数用 Git 实测，不按文案推算。

## Review 维护流程

1. 发现只记候选假设；用具体 trigger、源码路径、最小 red 和合法正控证实后，才能记根因。
2. 同类再现先核来源/事实单位/快路径等共同边界，并 scoped rg 检查同形分支；无关发现报告，不顺手修复。
3. 作者把稳定规则写入最窄 AGENTS.md，更新本文件对应规则的永久回归入口；reviewer 在报告中核约束是否实际覆盖新路径，而不只做关键词检查。
4. 修复树应含永久回归；旧树 red 用隔离 checkout/overlay，冻结 `.received`/seed 原样保留，不 revert/stash 共享树。green 同输入重放，必要时加有效邻接 fuzz 与真实 CLI。
5. 指导改动与产品证据身份分别记录，已冻结树不原地改。新 fixed bundle 携带所用指导；发现新 FAIL 仍停下，不因已写 AGENTS 或 skill 就放行。
6. 后续批次在正常复审检查点维护相关条目，不重跑已自然结束的长检查、不重写历史数字。新限制或 schema 语义须更新契约并获对应授权，不能由注意事项私自增加。
