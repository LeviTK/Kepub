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
