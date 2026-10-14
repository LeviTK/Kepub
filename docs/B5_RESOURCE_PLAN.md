# 下一任务：B5 资源新增与 OPF 清单原子维护

状态：**契约准入计划，未冻结、未编码。** 所属 [B5 #48](https://github.com/LeviTK/Kepub/issues/48)，父项#33；原B5.1–B5.3清单完整保留。此计划不提前注册available能力、不扩旧schema、不关闭Issue。用户本轮只要求制定下一步，不推导产品编码授权。

## 目标、依赖与非目标

将作者明确提供的新资源字节和对应manifest条目纳入同一个原生事务，供后续 [C1 #49](https://github.com/LeviTK/Kepub/issues/49) 章节新增、E3封面/图片使用。前置B1/B2已在main交付；B3已由PR76合并。产品基线217d42b/e0f88851，后续纯文档提交须另核适用性，不把编码分支当origin/main。

非目标：联网抓取、任意shell/XML patch、删除/改名既有资源、字体嵌入、EPUB2隐式升级、自动拆章、spine重排、GUI或完整M1。C1负责章节/spine；nav生成/同步属C2，不能把B5的manifest properties当完整导航维护。

## P0：先交付版本化契约并独立准入

以下决策必须写入CLI_CONTRACT的新节，形成固定输入，审查通过后再申请产品编码；这里只列需冻结的内容，不冒称字段/错误码已决定。

1. **输入来源**：明确调用者给定的本机文件/字节如何有界读取、冻结hash并持久绑定；Plan后外部文件漂移必须拒绝或只消费已冻结字节。不能在Apply/history重推中依赖未绑定的外部路径。比较内联payload与工作区受限不可变输入资产方案，说明持久JSON与资源预算代价。
2. **路径与身份**：BookPath、父目录规则、目标不存在、manifest ID/href/media-type、大小写/编码alias与碰撞。新增/既有资源一并派生完整最终库存及identity事实，不按单资源局部放行。
3. **首版类型与属性**：冻结允许的媒体类型、内容真实性和新增URL/IDREF提取范围；普通资源先行。nav/cover-image的唯一性和切换策略单列，不因此暗授现有OPF任意属性写权限；不新增字体本体。
4. **版本与混合**：新增operation ID/version、请求schema、policy、能力shape及只读绑定入口；与metadata/content操作的混合规则单列。不能把B5混入旧schema1–8作为隐式扩权。
5. **预算与失败**：单资源、完整最终出版物、文件数、持久记录、操作数、失败码；依已有预算选确定规则，不以私有长度/内存捷径拒绝合法输入。明确Plan/Apply/恢复各阶段无半写的可观察结果。
6. **审阅和来源**：diff显示新增资源及manifest变更，证明所有非目标字节保留；history/recovery能够从持久冻结输入重派生，而非相信写集合。新资源中未改写的引用也进入完整门禁。

P0退出：六项均确定、规范资产满足当前流程、公开非对称fixtures和正反矩阵冻结、独立契约准入与父核收完成；不是产品PASS。

## 后续实现增量（须另获编码授权）

- **P1 / B5.1 + B5.3**：首版允许族的普通资源＋manifest条目原子新增；打通Plan/Apply/diff/Reject/history/recovery/正式Accept/Export/reopen。未实现B5.2时Issue保持开放。
- **P2 / B5.2**：按独立冻结规则处理nav/cover-image等必要属性、重复/切换/一致性。所有B5清单满足后才能关闭#48。
- **随后 C1**：基于B3/B5，新增章节并同步XHTML/manifest/spine，再做显式阅读顺序；不能通过直接改候选文件冒充原生能力。

## 已确认的所有权与缺口

- `internal/app/app.go` 能力声明，`cmd/kepub/`入口；`internal/workspace/edit.go`严格请求/schema/policy、Plan/Apply/预期diff/执行恢复。
- `workspace.writeCandidateFile`目前要求目标已存在；`expectedDiff`只替换基线entry。新增文件及父目录、added diff、来源重推不是现成能力，不能只改写入函数。
- `internal/workspace/structure.go`、`tree.go`和`core_metadata.go`拥有当前全事务事实、库存和净预算；`internal/publication/`拥有manifest/身份/XML事实，`internal/references/`提取引用。读取complete不授予新增/修改权限。
- `lifecycle.go`、`checkpoint.go`拥有history、审阅、接受与恢复；`internal/app/workspace.go`负责正式导出。复用这些所有者，不另造事务/版本库或直接调用外部写工具。

## 验收矩阵与执行安全

合法新资源→编辑/混合两序→实际diff→Reject后重开→重新Plan/Apply→真实checker Accept→Export/reopen；完整ZIP库存、OPF/spine/nav和非目标字节用独立oracle比较。碰撞、越界、陈旧revision/hash、Plan后输入漂移、未知媒体/未知来源、预算边界、失败/中断无孤立文件及未知外来文件不删除分别配反例和合法正控。

涉及XML的首版允许族须覆盖UTF-8/UTF-16LE/BE、entity/default/unknown、alias/query和新增引用事实；仅对实际支持的类型声称覆盖。普通→race→vet、冻结回归、真实CLI/固定EPUBCheck、独立固定树审查及父组合接受仍必需。

压力探针必须一次性、整个进程组有硬内存/禁swap/超时限制，不使用失败自动重启服务；GOMEMLIMIT不是硬隔离。大OPF＋多操作病例必须在边界内完成或按已冻结预算拒绝，OOM/skip不是PASS；缓存只复用解析，不缓存授权。

角色：父线程协调/P0编写；既有独立Claude Code reviewer审核固定文档与后续产品；编码者在P0准入及用户授权后指定。旧“两次定向修复失败后Claude Code、再high”的升级要求保留，不因权限403升级模型。低置信估算：P0约1–2工作日（独立review及时、无资产缺口）；实现工期在契约冻结后重估。
