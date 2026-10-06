# S0 R2 修复增量（不是 R3 或批准）

接续 `df57cdd`，保留完整 R2 固定树、原始 rejected finalText／唯一决定块及真实 stream／exit。三主文档、父验收记录、产品、Go/module/setup 不改；T1 暂停。本批是供父独立验收的 S0 来源／验证器修复，不自称全文重新审查或完整规范支持。

## P9：纠正此前错误范围，保留历史失败

R1 的额外裁定及 `test_algorithm_explanation_paragraphs_cannot_be_repromoted` 把字体算法定义 p1/p3 当成 §1.5 的 Explanation，错误地要求排除。父当时认可过这个范围；认可和绿色测试不能改变固定源的含义。新报告不改写旧报告、旧 amendment 或其失败历史：R2 publication amendment 显式覆盖旧决定，恢复 p1/p3 原 featureId 为 mapped，新增 p2 的 XOR 定义。旧九条明示 exemplary pseudo-code 身份继续 excluded；压缩顺序／key／specifying 的真实 MUST 原样保留。

直接复读两份父下载的官方 patch 原字节，归入 `reviews/history/`，分别是：

- [core 642a45d0](https://github.com/w3c/epub-specs/commit/642a45d0cd81df784ff852d32a55a0f28f87d083)：SHA256 `0f0de8339e839508f292c99520f25f0da14e43717d0a31ad8fe74c5811ae7f3a`。添加 conformance 的 algorithm explanations 句，同时去掉具名 details/summary Explanation 内 p.note，没有改字体三段。
- [RS 271f0d7](https://github.com/w3c/epub-specs/commit/271f0d7f7c05652ee0fcf77fafe6ac89dc9b08e4)：SHA256 `5c5a6c4c38c30f2066fb479d7f1f165cedbf0d4b7b00f3b2a9830691501ce2f6`。添加该句并明确 details.explanation，与正式步骤／分支分开。

固定 REC `obfus-algorithm` p1/p2/p3 描述修改范围、XOR及循环／余部复制，没有 Explanation／example 范围；p5 明说 “The following pseudo-code exemplifies the obfuscation algorithm.”。定义所要求的是等价结果，不强制照抄示例伪代码或新增字体嵌入功能。

Librarian 返回 Issue1912 两条评论的原文引用，含“unambiguous definition”“normative algorithm”和非逐字实现的讨论；编码 Orb 自己直接请求两个 comment REST URL 均404，gh 请求亦404。随后 read_web_page 取得完整 Issue1912 页面（14评论），未包含该两段；因此不把 REST失败记录冒称为取得的原评论。当前修正的独立直接证据是实际 patch 和冻结 REC 正文。相关工具失败保留在本线程／工作区证据，不掩盖这个取得限制。

父新测试按原字节纳入 `scripts/test_epub33_parent_algorithm_scope.py`，SHA256 `d1d8ce4749e5fec5932684fdaea5e962cd8e97e671fb67ba30d7b13cbb4aad4a`。本实施树修复前真实4tests／17.112s／5FAIL＋1ERROR／exit1；父九步测试不改。只纠正我们自己那个已被源历史证伪的测试断言，不修改父预期迎合绿色。

## A／B／C：独立条件与权限，不增加强制功能

新增 OCF URL 两个成功分支，只引用实际 p1 的正式条件，排除附属 Explanation；空 property 字符串限制与空 reference 不同；alternate 分别登记 package／collection 两个条件定义，以及“不必生成超链接”的免除义务。nav 的 ol主层级与 li/a/span含义各单列，不能用内容模型基数代替。没有把这些许可改成必须提供UI／阅读系统功能。

源结构核对另发现需单列的有限步骤单位：filename→path 的 While 循环条件，及 RS property-values 的两个 grouping step／两个 branch condition。既有 RS li[2]/p[2]、li[3]/p[2]、li[5]/p[2] 是独立后置判定／返回，原身份和摘录已逐项核对并保留，不能被首p或宽祖先替换。

最终来源行1547（1533旧身份全部保留、14新增），mapped1484／excluded63。仅原字体 p1/p3 的决定从 excluded恢复mapped；五维仍not-tested。500 section记录保留作为原研究包记录，其历史文字／计数不是新的独立批准；本报告限定说明本次新增／纠错和核对范围。

## F-D：有限结构核对，不是自然语言真相生成器

- import先记既有featureId集合，在全部review＋amendment合成后、写入前要求保留；错误观察需reasoned excluded保留，不能悄悄删掉，也不自动补回坏输入。失败不部分写matrix。
- 从冻结DOM独立枚举所有正式 ol.algorithm 的步骤与分支／后置p，跳过具名Explanation和明示伪代码示例；不是根据幸存row触发。另核对27个有界、已明确审核的source list/value/definition族selector（包括R1叶列表、rendition值族、A/B/C/nav及字体定义），沿既有DOM／featureId，不另造逐段人工账本。
- 成员只能由自己的行／同一声明的BCP14实例表示。单p步骤可由直接host li表示；同一步骤多个p必须分开。宽section／ul祖先、重复另一个sibling、Explanation后代不能填补缺成员。
- 合法未完成section可pending；普通verify不自动判定所有自然语言规范性，不凭这些结构选择器宣布全文语义完整。gate仍拒绝pending与缺有效独立批准；修改已绑定输入后旧批准失效。原F-D仅证明普通verify接受review包删项再生成，不证明绕过当前identity批准。

## O-1及验证

固定官方reportCommit与每case reportCommit核对真实 `54092b4233253e9aac80e93ec4782b380b4b3403`；上游group／phase／role核对现有PROJECTS。已有许可缺口、behaviorTested=false、executed=false不升级；原118源资产和官方工件字节不变。

新增回归覆盖删整族、删整算法、独立后置p、宽祖先／重复兄弟、Explanation代替宿主、pending正控与gate负控、导入保留／失败不写、O-1的三字段以及文本内联空白。首批3tests真实6FAIL／13.064s保留。定向19tests首次有一个我们自己的非法HTML fixture失败（把details/p嵌进p，解析器正确关闭外p，句号不再属于它）；改为合法li宿主，期望文字不放宽。原100.090s失败日志保留；该修正单项0.024s通过。首次成员文本递归过早规范化误消耗内联空格，诊断后改为收集原文本再整体规范化；随后又发现已有BCP14句内实例应算同一声明而非要求重复添加整li，失败命令和first-index exit1保留。

完整Python套件首次实际105tests／281.886s／2FAIL／exit1，原日志 `kepub-s0-r2-fixes-suite.log` 保留。两项坏输入均被拒绝，但新增检查遮蔽旧诊断：导入丢失candidate时保留检查没有说明candidate mapping；官方来源不完整时先报缺reportCommit。修正保留身份的诊断，并将固定commit身份核对放在必需来源完整性核对之后；没有改变父测试或旧反例的预期。修正后的整组定向检查实际15tests／69.837s／OK／exit0。后续完整复跑使用新文件名，不覆盖首次失败日志。

完整recipe增加两个R2 amendment（其余冻结review保留），重建和四verify／official reproduce之后必须在固定提交重放无tracked漂移。完整suite／固定提交的真实结果与实际gate／bundle身份以工作区checkpoint和原日志为准；未执行的结果不写PASS。没有启动R3，没有重复Go大组合，没有真实acceptance.json。
