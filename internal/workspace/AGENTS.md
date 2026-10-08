# Workspace：冻结事务与最终状态

本模块拥有 plan/apply/checkpoint/restore/journal/diff/accept/history 的来源重算。

- 复用 `recomputeStructure`、`structureGate` 与既有 execution/revision 验证路径；新操作必须覆盖全生命周期，不能另造直接写入口。
- 每个显式绑定均检查该操作契约声明的精确 BookPath、accepted revision、resourceSha256、locator 与 expectedOldValue；不为无此字段的旧操作新增必填项。资源解析可以缓存，输入声明的验证不能缓存为“该路径已检查过”；检查首次与缓存后的 source/destination、两操作顺序及合法正控。
- 门禁顺序是派生全部合法编辑 → 收集完整事实 → 最终状态校验。真实区间重叠先拒绝；no-op 不虚增事实，同资源 move 不虚增减 identity。
- 同一冻结 locator 的 identity 属性 edit 使用 `publication.MergedIdentityDelta` 合并一次；identity 单元为每元素的不同值集合，不能逐属性累加或用 removed 布尔近似实例计数。
- 冻结入站和新增 URL/IDREF 共用最终计数；合法 alias transfer、partial delete、swap 应保留。新 URL 值未改写、块无 ID、已缓存目标都不能使事实漏收。
- 引用覆盖不足或无法表达的同步依赖明确拒绝，不猜测“没有引用”；源、目的、nav 与第三资源的输出都属于事务 oracle。
- task diff 观察实际候选与映射位置，不回显计划代替观测；候选 drift 可以安全审阅/拒绝，但不能沿用旧 execution 接受。
- 失败、中断、恢复、settlement 和历史 reopen 必须按冻结输入重算整个 write set；测试部分资源已写、restore leftovers 和再次 Open。只报告实际注入的中断点。

优先沿用测试：`TestStructureReferenceGateOrderIndependent`、
`TestStructureIdentityMultiplicityModel`、`TestStructureJointAliasEdits`、
`TestRequestsAndPlansRejectTampering`、`TestInterruptedApplyRollsBackNotRerun`、
`TestInterruptedApplyWithPreJournalRestoreLeftovers`。
新增跨资源/组合测试必须执行 Plan+Apply 并比真实输出，不能只检查 plan 的 WriteSet。

- 有效事务 fuzz 先确定性校准有限输入菜单及关键分支触达。合法族 Plan/Apply 错误必须 FAIL；拒绝族单独核原因，不能让两序都被拒绝充当“两序候选字节一致”。
- 双移动至少有非重叠源和不会同时被删除的锚点，合法 seed 必须实际执行两序 Apply/完整字节 oracle；写了条件比较代码、live 执行数多或单移动通过都不能证明该分支运行。
- 待观测资源由独立 fixture/契约选定，不由产品 WriteSet 决定；未写 nav 也须在 Reject 前读取真实候选并核冻结字节。负控须核预期拒绝码和可识别的拒绝原因，两序同码不足以证明 overlap 门禁。

第四批固定树 5e508f3 已获 medium 有限产品修复接受，另有 `TestStructureCrossMoveEndpointHashBinding`、
`TestStructureCrossMovePreservesQuery`、`TestStructureCrossMoveNewLinkGate` 永久回归；
复审核它们与原冻结 red 的覆盖对应，不把入口存在当作整批批准。树外候选
`TestCrossMoveFuzzCalibration` 尚不属于该固定树，须随下一身份核其断言与覆盖。

## 第五批：schema 7 完整来源与快照身份

- schema 7 的保存来源在每个消费阶段（Plan/Apply/执行/恢复/diff/accept/history）都按 **plan 自身**
  `BaseRevision` 的冻结 revision 重推完整提案并核 operations 多重集；提交的 plan 与 `plans/<id>.json`
  都是不可信输入，digest 相等不构成重推豁免。旧 schema 1–6 的 shape/校验不变。
- `taskDigests` 是 settlement 与已结算历史共用的来源消费边界；rejected 也必须重推，不把只在
  accepted revision 尾部验证当作完整覆盖。永久入口 `TestFixRejectedSourceRevalidation` 核实际
  TaskStatus/Open、accepted 推进后的合法旧历史、journal 尚在但 task 已归档的 recovery 正反例。
- 每个快照（revision/task）绑定自己的已核 tree/inventory；历史 initial 不得继承当前 accepted 的
  `InputTreeSHA256`。native facts 与 checker 必须消费**同一份**已核私有快照（`archive.SnapshotDirectory`
  + `Unpack` 到私有目录），live 目录只是身份证据，不是检查输入；drift/I-O fault 保留。
- 提案两快照入口保留文件／目录类型，HashTree 错误不得忽略；完整 Tree／hash／空目录不删。
  schema 7 新 policy 追加 `;file-target-v2`；旧已消费来源显式重推旧资格及 derivedFrom，不能用
  新资格改写旧 history/settlement。旧未消费计划和新的旧策略 accept 均要求重新计划；
  原持久决定恢复及旧 accepted 正式导出保留。入口 `TestN3SnapshotHashError`、`TestN3LegacyTargetSource`。
- task diff 的 schema 7 review 携带经重推的完整提案（真实旧值在 target，计划新值在 operation 参数）；
  旧 schema 输出 shape 不变。
- 已有外部输出用 `OUTPUT_EXISTS`/2（单点分类在 `outputPath`，预检原样传播 fault，其余路径问题仍
  `INVALID_OUTPUT`）；emit-request 在发布前执行既有 256 操作预算，超限 `INVALID_OPERATIONS`/2 且无产物。

## 持久 JSON 的读写预算

- 同类元数据共用 `maxJSONBytes` 和 `checkJSONSize`；预算按序列化后的 UTF-8 字节计量，包含
  转义和落盘换行，不能按原始字段长度估算。`writeJSON` 在临时文件创建前拒绝超限，不提高读预算。
- Create 在根目录发布前用正式读取入口读回 state 和 identity；失败只清理本次 staging。
  plan/execution/checkpoint/revision/journal 都沿同一写入边界，不另造无预算持久化入口。
- 注入预算保持私有、逐调用，不用可变全局或 CLI 扩权；短写用隔离子进程的小型文件预算，
  不做真实磁盘／内存耗尽。原书、完整 inventory、旧 schema、摘要、锁和 journal 恢复顺序保持。
- 永久入口：`TestR1JSONByteBudget`、`TestR1LegacyRecordByteBudget`、
  `TestR1CreateLongPathInventory`、`TestR1CreateMetadataShortWrite`。

## 替换推导的累计预算

- `recomputeStructure` 为含替换的全事务创建一份预算，多元素／操作／资源不能重置；
  Plan、Apply 和后续冻结来源重推各自新建相同默认预算，不复用先前消费状态。
- 在完整资源拼接前核全部输出大小，先计 disjoint removals 再加新字节，不按操作顺序
  误拒合法净缩减。预算失败在 plan／执行意图发布前拒绝，原书、accepted、锁与审计保留。
- 已登记 running／无 result 的执行重算超预算，先核 identity、冻结基线、保存来源、
  used、start 与 checkpoint，再沿原 restore journal 回滚并保留观察到的失败 diff；仍返回
  RESOURCE_LIMIT，不当作可容忍 drift。未开始／已完成／伪来源不改写；失败状态不能豁免
  后续 execution、reject、settlement／history 的完整重推。
- 小预算只通过私有逐工作区测试模板注入，不添加 CLI 开关或持久 schema 字段。
  入口：`TestN1TransactionBudget`、`TestN1ResourceBudget`、`TestN1ApplyBudgetRevalidation`、
  `TestN1PostStartBudgetRecovery`、`TestN1BudgetRecoveryBoundaries`。

## IDREF 策略版本与已消费旧来源

- 新 schema 4–7 的策略追加 `;reference-parser-v2`，旧 wire shape 和 schema 1–3 摘要不变。
  未消费旧策略在 Apply／WritePlanReport 前返回 INPUT_DRIFT，不用新图静默重新批准。
- 已消费旧策略仅按保存 policy 显式 v1 重推：execution、task diff、taskDigests／历史与
  settlement 共用该选择，不能只为某一恢复分支绕过门禁。旧 active 可以安全 diff／reject
  或中断回滚，但不能新 accept；已持久的旧 settlement 决定仍恢复完成，旧 accepted
  历史和正式导出保留。完整来源 hash、checkpoint、锁、journal 与真实 checker 均不豁免。
- 永久入口 `TestN2LegacyReferenceLifecycle` 的输入必须区分 v1／v2（p%41 与 pA），
  `TestN2LegacySchemas` 核 4–7 策略，`TestN2LegacyAcceptedJournal` 核真实 checker 后的
  durable 决定；保留旧文件字节，不重签或迁移旧来源来通过。

## 工作区资源 I/O 与取消

- 出版物资源树复用 `archive.Limits.CheckEntry`、`WalkDirectory`、`CopyBounded`；
  不先收齐无界目录／读完资源才拒绝，不混计管理目录、持久 JSON 与原 ZIP 的独立预算。
  stat 是快拒而非授权，实际余量 + 1 有界读取后仍核增长／drift；预算、I/O 与取消独立分类。
- 请求 context 必须沿原书核对、自身 frozen revision 来源重推、candidate／checkpoint／accept
  的每次扫描／复制传递。未提交 stage 失败清理，Close 或失败 Open 释放 owner 锁。
- 已登记 mutation 在取消后仍完整回滚；有效 restore／settlement journal 沿原决定恢复，
  仅临时 `WithoutCancel`，不跳预算、来源、hash 或真实 I/O，不删 durable intent 凑成功。
  入口 `TestR3CancelledAcceptDoesNotCopy`、`TestR3TreeBudgets`、`TestR3OriginalBudget`、
  `TestR3GrowthAfterStat`、`TestR3PartialTreeFailure`、`TestR3CancelledRunningRollback`、
  `TestR3CommittedRecoveryRetainsBudget` 和 app 的 `TestR3WorkspaceRequestCancellation`。
- task diff 的部分可审阅降级不吞掉快照取消／超时／预算失败；
  `TestR3DiffSnapshotCancellation` 核复制阶段真实取消、无私有快照残留、全 workspace bytes 不变。

对应开发注意事项：`G-FINAL`、`G-FACTS`、`G-BINDING`、`G-ORACLE`、`G-QUALIFY`、`G-JSON`、`G-IDREF`。
