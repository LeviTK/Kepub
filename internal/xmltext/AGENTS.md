# XMLText：decoded、词法与物理来源

- parser、expanded stream、decoded CharData 与原始资源（含 BOM）的偏移分别处理；`TextRun.Range`、`AttributeBytes`、`PhysicalMarkup`、`PhysicalContent` 不伪造生成来源的物理区间。
- linear literal、实体引用拼写、generated/default、CDATA、unknown 必须保留不同来源。相同 decoded 值不代表相同写权限；partial 不推导肯定的完整事实。
- fragment 对齐要比较正确 decoded 字符语义，区间从该 fragment 自己的原物理 span 得到；空 generated span 也是边界，不能吞掉未匹配引用或让匹配跨越它。
- 有明确来源的合法左右字面应可独立写；无法证明的 token 保守不可写，但先验证对齐是否正确，不用整 token 降级掩盖支持范围内的合法事务误拒。
- XML newline/reference 规范化按现有解析层责任处理，不对已经构造的 generated 内容再次归一；UTF-16 代理对不能用 UTF-8 索引当物理字节索引。
- 预算来自 CLI 契约，不引入私有 64 字节等阈值。禁止读取外部 DTD/实体；后续显式迁移 profile 的授权不能影响普通读取路径。

优先沿用 `TestPhysicalMarkupRangesUTF16`、`TestAttributeMarkupRejectsGeneratedValues`、
`TestTextRunsKeepsWritableLiteralIntervals`，并配合 publication 层真实 splice 测试。
新增 provenance 形状覆盖直接/嵌套生成、预定义/numeric、空实体、CR/CRLF、同值 literal+generated 与跨 fragment 负控。

对应开发注意事项：`G-PROVENANCE`、`G-ORACLE`。
