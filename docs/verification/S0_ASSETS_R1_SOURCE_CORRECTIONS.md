# S0 R1：固定原文、继承条件、REC backlink 与 schema 修复

本批接续 R1 F1/F2/F5 的独立验证器 checkpoint，修订 F3/F4/F6、父 P8 及有限 T3 前置登记。它是供父独立复验的源数据／脚本增量，**不是 S0 批准或 R2 审查**；没有产品、Go/module/setup、三主文档或父验收记录改动。T1 仍暂停。

## 真实来源与范围

R1 实际 0.233.0 / claude-opus-5-5 / medium、94 turns、completion 和 exit 0，仍是提出发现的审查而非通过。保留其阅读限制：EPUB 后约 189 个块粗看、Notes/CSS/tests 抽样，不能称完成全量正文全文审查。父独立 F3/F4/P8 红日志和测试原稿保持不改；新增父测试逐字复制到 `scripts/test_epub33_parent_algorithm.py`、`scripts/test_epub33_parent_list_backlink.py`，SHA 分别 `d29d51d3b109c1e4a33dd3a00d25dd5fbfe618c32f04df73d7800a2c60b9e7a2`、`b4fafbb877d5aea0436946dd3c0c14a0fa05aff595b875aacb5f0aec6e190c3d`。

编码 Orb 本批读取实际固定 REC 的所选列表／前置段落、算法段落、相关无标记定义与原 bibliography；XML 依赖解析覆盖完整新 schema 字节，不把程序解析等同人工全文语义阅读，也不将本批称为重新全读全部三 REC、Notes/CSS 或 169 工件行为。补录数据仍须父复验与后续真实完整审查。

## F3：最小独立条件，而非所有列表都是义务

新增 59 源实例：filename 禁止类 20 叶（18 顶层类中的 noncharacters 有三个独立范围）、data URL 四情境、禁加密八资源、manifest 五条件属性、remote 四允许族、namespace 两排除域、fallback 两替代机制、toc 两顺序、skip 三／escape 四可选值、a11y page-navigation 三触发／sync 两顺序。实际结果为 1533 rows / 784 manual：1468 mapped、65 排除，原 1474 身份无删除，500 section records 不变，五维仍全部 not-tested。

新叶记录保留 fixed document/hash/DOM/excerpt，并绑定真实前置 `contextDOM/contextExcerpt/contextSHA256`。文件名 FULL STOP 只约束末字符；data URLs 的 a／area iframe 例外保留；remote 属 MAY、其余资源在容器的 MUST 另列；custom namespace 中 w3.org／idpf.org 是域排除条件而不是许可域。fallback 是 one-of，toc／sync 推荐同时反映两个维度，page navigation 是 any-of；a11y sync 额外目标明确 OPTIONAL。manifest 属性逐项按自身定义触发且不递归到嵌入资源，不要求每 item 带全部五属性。

无标记 rendition layout/orientation/spread/flow/alias 的 dd 记录绑定真实相邻 dt 的值与哈希，不把标题再算一项义务。可由原文推导的绑定既不能整组删除，也不能把 manual 和 row 一起改绑到同节真实但不相关的段落／取值。旧统一 reason 对冒号前言声称包含完整继承列表不属实；重建明确标注该 excerpt 只到冒号，列表条件必须另有实际源映射，不以一个段落计数宣称完整。

父确认的 optional-DC 五裸 dt、MathML 两 dt、viewport name/content 标题不另造义务；RS metadata “Some uses … include” 例示不提升。没有为了计数将所有 dl/ul/ol 自动认定为强制规则；leafCount 只是对已明确人工选择、固定 hash 的列表重建实际叶实例并防层级漂移。

## P8：说明与真 MUST 分开

固定 REC §1.5 原文为 “All algorithm explanations are non-normative”；`obfus-algorithm` p5 为 “The following pseudo-code exemplifies the obfuscation algorithm.”。保留九个已有伪代码身份，全部改为有理由的排除，不依 R1 的“再补八步”建议制造规范要求。

p1 说明修改前 1040 bytes／短文件情况，p2 描述 XOR，p3 描述循环／拷贝过程；按该明确算法说明范围，已有 p1、p3 两观察也排除，p2 不新增为义务。原 kind／level 标签作为旧观察身份保留，不表示其决定仍是 normative。不是整节排除：p4 压缩前混淆的真实 MUST、key 推导与 specifying 的真实 MUST 继续 mapped。验证器拒绝重新提升这些解释，但不排除单列的 marked MUST；父九步反例和新增 p1/p3 控制覆盖这一边界。

## F4：规范自己的测试引用是另一份来源

从固定三 REC 的实际 data-tests 重算 237 fragment refs／162 ID，其中 158 属冻结 169 case；raw document hash、DOM、原 attribute、reference 与节点文本 hash 保留。dt→dd、span/cell→具体 p/tr 的上下文规范化重用同一规则，不将缺失上下文改成整个 section。补入父指出的十一具体 REC 条款关联，保留旧 report/publication-side 条件、URLs、expected、normativeLevel 和实际工件。

未知 `fxl-svg-icb_multi`、`lay-fxl-layout-duplication`、`pkg-spine-nonlinear`、`pub-cmt-jpg` 明示 absent-from-frozen-report / unresolved / not-executed，不猜别名、不换 3.4。十一无 REC backlink 的已知 case 保留 report 目标及显式缺口。另 456 external structural-test references 单独登记，不当作冻结 RS 工件执行要求或已执行。来源对应中的每条 clause 现在保留 decision/reason，以免排除项只剩旧 level 标签而被误读为有效义务。

## F6：实际 NVDL／Schematron 和版权闭包

固定源提交 `029831b8f477e4519e9734c984ee24357547a698`；正文三个 GitHub tree/master 文件链接转为同 commit 的实际 raw URL，不按展示文字中的 typo 构造 URL。实际下载原字节，归档 118 文件，原 107 文件全部逐 hash 保留。

三 NVDL 去重闭包为 14 schema + 目录 LICENSE：package、ocf-container、media-overlay 三 nvdl，三 rnc，package/media-overlay sch，multiple-renditions/container rnc/sch，mod/datatypes.rnc、epub-prefix-attr.rnc、epub-type-attr.rnc、id-unique.sch。11 文件新取得，四 schema 已存在。directory LICENSE 是 IDPF MIT-form、SHA `bc31197ad4f01c1813dd200349c75309c29a1486b5b5d1ae51a5203acfc4df00`，不能用根 BSD 替代，不宣称法律意见。

有限显式 fetch --complete-schemas 现在重读已归档 HTML 的直接 schema 链接并闭合实际依赖；离线 verify 重算 NVDL validate/schema、Schematron include/href 及 inherited xml:base。第一次捕获只增加三个 dispatcher 和 license（111），源阅读查出初版 QName 少了 `/ns/structure/1.0`，漏掉实际 validate；修正为真实标准 namespace 后才得到 118／15-file closure。第一次 CLI exit 0 不当作完整闭包通过；空 capture log、实际原文检查及最终闭包记录保留。新增回归同时覆盖真实精确闭包、删 dispatch 依赖、main/blob 和 master/tree、非对称 inherited xml:base。

这是规范链接的源归档，不证明该源码就是运行 JAR、checker 全 REC baseline 已知、执行了 schema 或通过全部规范。

## T3：有限、机读、未取得

`reviews/t3-prerequisites.json` 及派生 semantic index 登记八家族：OPF/OPS/OCF 2.0.1、NCX、DTBook、XHTML1.1 DTD/modules/entity sets。前三引用逐项绑定已归档 bibliography 的原 URL/题名/版本（含 .doc、draft 命名），其余精确 edition/URI 待 T3 冻结，未捏造 URL；所有取得 hash/path 为 null、pending-T3-archive。验证器拒绝遗漏家族、假 acquired hash、改变真实 citation 或声称运行时下载。登记本身不替代 T3 开始前有限离线归档，也不在 S0 新增迁移实现／全 Web 下载。

## 验证与保留失败

完整离线 recipe 见 `docs/EPUB33_SUPPORT_MATRIX.md`，包含两个新 amendment；先重建，再四类 verify／official reproduce；固定提交后必须重放且 git diff --exit-code 0，gate 仍须因没有独立批准实际 exit 1。所有父原测试预期不修改；不重复 Go 大组合或新开 Droid。

第一次完整 Python：91 tests / 211.504 s / exit 1，一个新增回归失败。测试只按 DOM path 选记录，误选了拥有同一 XPath 的 RS 记录，却写入 EPUB 段落；验证器正确以源绑定不符拒绝，但不是该测试要验证的同节不相邻分支。明确添加 document=epub 选择条件，原异常预期没有放宽。原失败日志 `kepub-s0-r1-source-full-python.log` 保留；修正后四个绑定定向测试实际 4/4、30.186 s、exit 0。随后完整最终代码的实际结果及固定树重放证据单独附在工作区 `.agents`，未跑的结果不写 PASS。

最终完整 Python 实际 **92/92，229.677 s，exit 0**，日志 `kepub-s0-r1-source-full-python-final.log` 与实际 exit 文件保留。完整文档 recipe、四类 verify、official reproduce 实际全 exit 0：118 archive files，1533 candidates（1468 mapped／65 excluded），169 cases／170 report rows／170 publications，官方 executed=false，上游 behaviorTested=false／一个真实许可 gap。没有实际独立批准；gate 必须继续 exit 1。固定提交及提交后重放的原始记录和增量 bundle 身份由独立 checkpoint 文件给出，不用本报告自称完成 S0。
