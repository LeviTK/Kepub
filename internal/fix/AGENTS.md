# fix：原生规则的适用性与可执行资格

本模块拥有两条原生规则的 fact/repair 派生与 ValidationDelta 分类；不拥有旧编辑权限。

- 事实适用（fact）、物理可写来源（AttributeBytes）与**真实旧门禁资格**是三件事：fixable 必须能通过
  既有 gate 执行。新 URL 值按 gate 的原样解析证明目标存在（不 trim 出第二种语义）；不满足时保留
  repair、`status=unfixable`、`operation=null`、`writeSet=[]` 与确定性原因，不生成可执行项。
- 元素适用范围是**完整祖先链**：任何祖先不在 XHTML namespace 的重入元素只记 limitation，不产生
  repair/native fact（coverage 变 partial）。只读 workspace（含 SIGNATURES 等真实原因）不宣称任何可执行
  repair。生成/default/unknown 来源保持不可写。不得为让探针通过而放宽旧 namespace/权限/物理来源。
- FR2 的 `?x=1`／`?` 删除后为空字符串仍是适用事实与真实属性编辑；目标存在性一经确认即加入
  readSet，不以 fragment 非空或成功解析作为依赖收集的前提。不 trim 新值或放宽旧 gate。
  永久入口：`TestFixRelativeURLQueryBytes`（workspace，三编码完整七资源字节及旧 gate 正控）。
- 完整库存保留类型；新 FR2 只认 `file`，目录／未知类型不是资源。与结构门禁共用
  `publication.CheckReferenceTarget` 的存在性和唯一 fragment 条件，不为非 XHTML fragment 扩权。
  `Snapshot.TargetVersion=1` 仅供已消费旧 schema 7 policy 的完整来源重推，不成为新请求入口。
  入口：`TestN3TypedInventory`、`TestN3DirectoryTargetQualification`、`TestN3FileTargetMatrix`。
- 相同 severity 先逐实例 persisted，再按剩余实例数配对 severity 变化；不能一次消费整组或复用已
  persisted 的实例。native 同形边界也要保留 surplus，覆盖不足仍不可宣称 resolved。
  永久入口：`TestClassifyDeltaSeverityMultiplicity`、`TestClassifyDeltaNativeMultiplicity`。
- Delta 分类按**实例多重集**配对（重复减少 = 一 persisted + 一 resolved），工具 SHA-256、config、
  profile/flags（未知写 `unknown`）与覆盖范围参与可比性；generic RSC-005 等无唯一归属的 code 不
  resolved/upgraded；正常合规 FAIL 仍是 completed。

回归入口：`TestRuleForeignAncestry`、`TestRuleRelativeURLQuery`、`TestFixReadOnlyQualification`、
`TestClassifyDeltaInstanceAndComparability`、`TestMediumDeltaMultisetAndCoverage`（reviewer 冻结源）。
