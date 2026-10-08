# References：字面 IDREF 与 URL 分流

- 既有 IDREF 直接构造同文档 `bookpath.Reference`，不要拼 `#` 送入 URL 解码。
  `edge.href` 保留完整作者属性值，`target.fragment` 是单 token，locator 的 `/@name`
  保留属性来源；随后共用目标存在性、身份实例计数与诊断。普通 href/src 仍走 URL 规则。
- 与 publication 属性／片段／搬入事实共用版本化 `IDREFs`；列表只拆 ASCII 空白，
  NBSP、百分号、括号和非 ASCII 身份不规范化；单 IDREF 不拆成合法列表。
  不扩词表或把未知／不完整来源当无引用，也不放宽新建 ID 的 NCName 约束。
- inspect 用 parserVersion=2；只有已消费的旧计划来源明确选择 v1。旧 parser 的解码、
  分词、coverage 和 edge shape 必须按原样重推，不迁移旧审核摘要。
- 永久入口 `TestN2LiteralIDREF`、`TestN2IDREFValueTypesAndIdentity`，同时核普通 URL
  正控、同元素 alias、重复 ID 的歧义与完整来源关系；图 PASS 不能代替实际 Plan+Apply。

## 完整图的调用级预算

- 所有构建入口共用一份 context/GraphLimits；先核下一个资源实际读取余量，再扩索引、边、
  identity 实例、诊断、coverage/reason 与 XML coverage。重复字符串和 locator 也收费，
  不先按无界 inventory 长度分配 slice 或先 Marshal 全图；默认值／保守计量见 CLI §3.1。
- 超限返回 `REFERENCE_LIMIT`/1 和零 Graph，取消停止后续读取；零图不构成无依赖证明。
  Filter 只过滤完整构建的边，不能用指定资源跳过别处的入链；预留其 header 增长。
- 预算重建不改变 parser v1/v2 的边、诊断、覆盖 shape 或旧来源摘要；小预算逐调用注入，
  不重置为每资源预算，不改大默认值让反例通过。计量不是 RSS 或具体 OOM 阈值。
- 入口 `TestR4CancelledGraphStopsReading`、`TestR4CumulativeGraphCounts`、
  `TestR4ReservationBoundaries`、`TestR4FilteredResultHeaderBudget` 和
  `BenchmarkR4BoundedInventory` 核真实读取、预算±1、重复实例、过滤和分配前拒绝。
