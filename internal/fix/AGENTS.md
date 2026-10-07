# fix：原生规则的适用性与可执行资格

本模块拥有两条原生规则的 fact/repair 派生与 ValidationDelta 分类；不拥有旧编辑权限。

- 事实适用（fact）、物理可写来源（AttributeBytes）与**真实旧门禁资格**是三件事：fixable 必须能通过
  既有 gate 执行。新 URL 值按 gate 的原样解析证明目标存在（不 trim 出第二种语义）；不满足时保留
  repair、`status=unfixable`、`operation=null`、`writeSet=[]` 与确定性原因，不生成可执行项。
- 元素适用范围是**完整祖先链**：任何祖先不在 XHTML namespace 的重入元素只记 limitation，不产生
  repair/native fact（coverage 变 partial）。只读 workspace（含 SIGNATURES 等真实原因）不宣称任何可执行
  repair。生成/default/unknown 来源保持不可写。不得为让探针通过而放宽旧 namespace/权限/物理来源。
- Delta 分类按**实例多重集**配对（重复减少 = 一 persisted + 一 resolved），工具 SHA-256、config、
  profile/flags（未知写 `unknown`）与覆盖范围参与可比性；generic RSC-005 等无唯一归属的 code 不
  resolved/upgraded；正常合规 FAIL 仍是 completed。

回归入口：`TestRuleForeignAncestry`、`TestRuleRelativeURLQuery`、`TestFixReadOnlyQualification`、
`TestClassifyDeltaInstanceAndComparability`、`TestMediumDeltaMultisetAndCoverage`（reviewer 冻结源）。
