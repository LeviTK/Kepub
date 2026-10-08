# Publication：局部编辑、身份与 URL 语义

- 复用 `StructureDocument`、`StructureEdit`、`ValidateEdits`、`ApplyEdits`、`VerifyStructure`；校验目标的完整祖先链、manifest 权限和插入点 namespace 上下文，不只检查当前元素。
- identity/IDREF 使用已有共享 `IdentityValues`、`ElementIdentities`、`CountIDs`、`MergedIdentityDelta` 与 IDREF 词表；不能新增一个只适用某操作的别名计数模型。
- IDREF v2 是字面身份而非 URL：列表只拆 ASCII 空白，NBSP 保留，单 IDREF 不拆；
  output.for 按宿主列表类型处理。属性、片段和搬入块共用同一 tokenizer，旧已消费来源显式用 v1。
  入口 `TestN2IDREFTokens`、`FuzzN2IDREFTokens`；解析既有身份不放宽新建 ID 的 NCName 权限。
- edit 只使用冻结来源的可写物理区间。精确验证预计位置及目标外字节；相同字节在别处出现不能使错位写入通过，不用全输出 Contains 兜底。
- 字面替换按每元素直接字符数据和匹配局部 fragment 求区间；实际 regex 任一零宽命中整体拒绝，不静默 continue；非空锚定和合法零命中保留。
- generated/serialized fragment 以正确 decoded 语义参与对齐，但仍不可写；只 decode 所需层次。保留实体两侧字面、零长 generated 分隔边界、CRLF 和 UTF-16 映射；不添加未定义的引用长度限制。
- 跨资源移动只改契约列明的 URL 属性，保留 RawQuery、显式空 query（ForceQuery）、fragment 和未改写拼写；检查 local/self/other/incoming 四条路径，不用 `"#" + fragment` 丢掉 URL 分量。
- 每个搬入的 href/src 都贡献链接事实，与是否重写值无关；https 正控、javascript/data 拒绝控、内部目标缺失控必须同时存在。
- 编码变换保持源外和目的外字节、原编码/BOM/声明；转码不获得额外 XML/namespace/来源权限。
- 替换共用逐事务 `ReplaceBudget`；索引有界、捕获元数据和模板展开先核预算再分配，不能
  用最终 expectedHits／字符串长度检查代替。模板计数以 Go ExpandString 语义为准，
  包括重复命名组的首个参与实例；转义编码由 xmltext 计数，等值替换仍核来源但不重写 CRLF。
  永久入口 `TestN1ExpansionBeforeAllocation`、`TestN1TemplateGrammar`、
  `TestN1EncodedAndResultBudgets`；计数／benchmem 不冒称 heap 峰值。

优先沿用测试：`TestStructureEditsExactBytes`、`TestIdentityUnitIsPerElement`、
`TestReplaceZeroWidthAndProvenance`、`TestReplaceGeneratedDecodedNeighbors`。
来源/移动改动用 UTF-8、UTF-16LE、UTF-16BE 的完整资源独立 oracle，并保留合法邻接与禁止跨 fragment 的成对病例。

对应开发注意事项：`G-FINAL`、`G-FACTS`、`G-PROVENANCE`、`G-URL`、`G-ORACLE`。
