# Kepub 开发约束

本文件约束全仓；子目录 AGENTS.md 只补充所属模块规则，不扩大产品权限。

## 开始工作

- 先读 `docs/CLI_CONTRACT.md` 的对应批次、`docs/DEVELOPMENT_PLAN.md` §11.7–11.9 和 `docs/DEVELOPMENT_NOTES.md` 的相关规则；不要从 README 的阶段摘要推断功能已验收。
- 用 `git status --short --branch`、HEAD/tree 和 upstream 确认输入。区分本地 main、origin/main、编码工作树与待审固定树；不得清理、隐藏或覆盖其他人的修改和证据。
- 先确定行为所有者和已有 helper，再做最小修改。新能力先冻结版本化契约，不改旧 schema、摘要、权限或预算含义；不借任意 patch/shell/外部工具绕过原生事务。
- 修复拒绝或回归时加载 `debugging-kepub-regressions`；提交复审前或执行 review 时加载 `reviewing-kepub-changes`。项目 skill 位于 `.agents/skills/`，不支持 skill 的工具直接读取相应 SKILL.md。

## 不可省略的不变量

- **完整最终状态：** 先派生全事务编辑、收齐 identity/URL/IDREF 事实，再统一门禁。不得按操作或资源局部放行；同节点所有冻结 identity 属性先合并再求增减。
- **逐次绑定：** 每次显式资源/端点使用均校验该操作契约声明的冻结 revision/hash/locator/旧值，不增添旧 schema 的必填字段。缓存、预加载、相同路径与 no-op 不提供校验豁免。
- **事实完备：** 新增或搬入候选的引用均进入门禁，包括值未改写、外部 URL、无 ID 块。提取 complete 不等于编辑授权或 EPUB 合规。
- **来源与保真：** decoded 文本、词法拼写、原物理字节区间不是同一种数据。generated/default/unknown 来源不得冒充可写字面；合法字面邻接不得因保守降级被无依据误拒。
- **无隐式扩权：** 不为让反例通过而扩大 namespace/manifest/来源权限，不加契约之外的长度捷径，不静默跳过失败命中或部分写入。

## 验证与停止线

- 修改产品代码要跑受影响的普通测试；涉及共享事务、来源或引用门禁时加 focused race、旧 schema/冻结回归及相关真实 CLI。正式批次共享增量依序执行：

  ```sh
  go test -count=1 -timeout=30m ./...
  go test -race -count=1 -timeout=30m ./...
  go vet ./...
  ```

- 用 gofmt 格式化改动的 Go 文件。纯指导/文档变更检查链接、路径、skill 元数据与实际加载，不无意义重跑长产品套件。
- 回归必须有独立期望的永久测试、合法正控与失败反例。已拒绝病例/最小 seed 原样保留；在隔离旧树确认 red、修复树确认 green，不改弱冻结断言。
- 高风险组合按 `docs/DEVELOPMENT_NOTES.md` 选择有区分力的矩阵：两操作顺序、重复/缓存资源、三编码、alias、query/空 query、来源边界。比较实际 Plan+Apply 候选与独立字节/图 oracle，不只比 WriteSet、Contains 或“可读取”。
- 有效事务 fuzz 的生成输入必须合法、必须成功执行且核对独立 oracle；意外拒绝本身是 FAIL，不能 continue。记录 seed、参数、执行数、退出码和真实覆盖。
- 长测试自然运行并追踪原进程，不为取输出重跑或取消。检查器缺失、skip、工具错误不算产品 PASS；实际反例 FAIL 不被全仓/race/vet PASS 抵消。
- fixed candidate 一旦改变，旧批准不自动适用。编码方自审不代替独立审查；批次只在同一输入的独立批准和父验收后集成，之后再获下一批授权。

## Review 后维护

- 确认新根因后，在最窄所属 AGENTS.md 写稳定约束，在 `docs/DEVELOPMENT_NOTES.md` 更新“规则→失效模式→永久回归/证据”。同类再次出现先检查模型/事实边界，不继续只补症状。
- 维护指导不修改冻结待审树或历史失败日志；单独传递指导，随后产生新固定树。当前未经修复的产品问题仍须阻断，写规范不等于修复。
- 日志绑定 commit/tree、runner、命令、退出码；提交前 working-tree、fresh-fetch、父执行和 reviewer 执行分别标注。故障注入不等于断电，交叉编译不等于 Mac 实机，race harness 不等于 race CLI binary。
- 不提交私有书、凭据、原始敏感日志或 `.received` 探针。push、release、部署和关闭 issue 需要各自明确授权；部分 T2 验收不等于完整 T2/T3–T6 完成。
