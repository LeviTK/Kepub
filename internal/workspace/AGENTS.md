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
- task diff 的 schema 7 review 携带经重推的完整提案（真实旧值在 target，计划新值在 operation 参数）；
  旧 schema 输出 shape 不变。
- 已有外部输出用 `OUTPUT_EXISTS`/2（单点分类在 `outputPath`，预检原样传播 fault，其余路径问题仍
  `INVALID_OUTPUT`）；emit-request 在发布前执行既有 256 操作预算，超限 `INVALID_OPERATIONS`/2 且无产物。

对应开发注意事项：`G-FINAL`、`G-FACTS`、`G-BINDING`、`G-ORACLE`、`G-QUALIFY`。
