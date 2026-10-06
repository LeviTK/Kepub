# S0 R3：未标记定义、建议及适用范围修复

本批以 `4fed666` 为实现基线，修复 R3 F1/F2/F3 与父同组补查；不是 S0 批准，也不启动 R4/T1。父拥有的三主文档、矩阵说明、验收记录、固定 R2/R3 树与旧 T1 草稿不修改。

## 阅读与真实限制

R3 实际 Factory Droid 0.233.0 / claude-opus-5-5 / medium，新会话完成后 exit 0，但决定为 rejected、readingComplete=false。其 residue 阅读不能代替三 REC 完整规范正文对照，tests 与治理文档也未完整阅读；新的 pre 无截断并不消除 residue 筛选的限制。原报告、stream、探针与阅读记录保留。R3 报告把 gate 负控写为 21，实际探针为 20；工具 `ls` 缺 approval 的 exit 2 不是产品测试失败。

编码 Orb 逐项读取本批固定 EPUB REC 段落与周围定义、列表、dt/dd、相似 informative 区域。25 个受影响 section 的 notes 明确限定实际阅读、条件及建议性质，不作为独立全文审查或实现执行证据。此有限源对账不能自动判断所有自然语言规范性，也不能证明没有其它遗漏。

## 语义修复

- General exemption：p4 与两叶的条件是 AND；content-specific exemptions 仍独立。Package link target 的 p3/两叶是 OR，不是 AND，也不要求下载外部 metadata。
- SVG：分别登记 by-reference 与 by-inclusion；后者及 SVG restrictions 的引言明确包含 XHTML 内联 SVG，不把 img/object 示例变为强制机制。
- Metadata：mandatory child text、内部空白 collapse、首标题/creator 顺序、primary/subexpression、linear/non-linear 含义及非规定呈现方式分别登记。
- 脚本：data block 非 scripted content，iframe 与 top-level 两执行上下文、inline/src 不改变上下文、HTML/SVG 两 container-constrained 替代及 spine-level 定义。只登记定义，不在 S0 新增脚本执行。
- 保留 subject scheme 条件性 case sensitivity、OCF scalar value/case identity 与独立唯一性规则、content URL 解析基准及 URL syntax AND algorithm 条件。
- XHTML 真正规范 sec-xhtml-req p2 继承 HTML；相似 sec-overview-relations-html p3 继承 informative，不能提升。Package prefix 的无 namespace 与其它文档 OPS namespace 保持区别。
- Role 多值顺序与 MIME security 建议是 lowercase should，不升格 BCP14 MUST。UTF-16 binary 是 MIME content-transfer-encoding，非 ZIP STORE/DEFLATE；注册中的 encoding/security 各 p 分别绑定，宽 dd 不能替代。

父同组另十处：html-script-element p3 数据块 fallback exemption；title p3 单标题建议；creator p2/p4/p6 的显示名、单独元素、次贡献者建议；meta p2 property/text 关系；meta p5、link p10、item-properties p2、itemref p9 四个按元素/attribute 区分的默认 vocabulary。原 scheme 无默认及禁止 default-prefix 的已有规则不重复。原文 excerpt 保留，全小写 should 登记为 advisory 而不是 MUST。

本批新增 **51** 个实际源实例（父原 41 + 同组 10）：1598 rows / 849 manual，1535 mapped / 63 excluded。原 1547 rows 与 798 manual 逐身份逐字段相等；500 section 身份不变，仅 25 条 notes/reviewer 改动。118 原归档资产、官方工件、upstream、产品/Go 字节不改变；全部五维仍 not-tested，semanticComplete=false。

## 有界验证与回归

沿冻结 DOM 的既有有限 families 增加这些成员，不由幸存 row 触发枚举。Complete section 中删整族或某个独立段落必须拒绝，兄弟/default 或宽祖先不能代替；合法 pending 仍可记录但 gate 不批准。Formal algorithm、Explanation、九步示例排除保持此前边界。

Amendment 可附 sectionNotes，按原文 section id/path 与已有 section review 绑定；无源/空/非字符串 note 拒绝且 matrix 不部分写入。成功仅更新该 note/reviewer，重复导入字节幂等；不借 note 创建新 section 身份或批准。

父原测试逐字安装 `scripts/test_epub33_parent_r3_definitions.py`，SHA256 `26f6a9f35b9539545a234fed0dc9636771fa425843ece110e841db7041b52d9e`，原 4 tests / 45 failures 的红日志不改。自身首次三源回归 3 tests / 8 failures / 38.818 s 保留；同组十处新增独立原文 quote/scope/level 回归在未重建矩阵上实际 1 test / 10 failures / 1.225 s。新测试还逐个删除十处绑定、检查五维未测及 sectionNotes 的原子性/幂等性。绿色结果不代替完整源语义审核。

## 完整离线 recipe

父矩阵说明仍由父维护；此实现树完整 recipe 在旧五 amendment 后追加本批，不能漏掉新 review 数据：

```sh
python3 -B scripts/epub33_assets.py index \
  --review docs/specs/epub-3.3/reviews/kepub-s0-epub-semantic-r2.json \
  --review docs/specs/epub-3.3/reviews/kepub-s0-rs-semantic-draft.json \
  --review docs/specs/epub-3.3/reviews/kepub-s0-a11y-semantic-draft.json \
  --amendment docs/specs/epub-3.3/reviews/publication-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r1-publication-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r1-accessibility-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r2-publication-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r2-rs-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r3-publication-amendments.json
python3 -B scripts/epub33_semantics.py index
python3 -B scripts/epub33_assets.py verify
python3 -B scripts/epub33_semantics.py verify
python3 -B scripts/epub33_tests.py verify
python3 -B scripts/epub33_upstreams.py verify
python3 -B scripts/epub33_tests.py reproduce
git diff --exit-code
python3 -B -m unittest discover -s scripts -p 'test_epub33*.py' -v
python3 -B scripts/epub33_assets.py gate
```

实际定向/完整 suite、固定提交后首次 recipe 和双 diff、gate exit 与 bundle 身份单独保留在工作区 `.agents/kepub-s0-r3-fixes-*` 日志及 checkpoint；未结束的检查不称 PASS。Gate 仍须以缺独立批准 exit 1。没有创建 acceptance.json、运行全部 RS 或声称全部 REC 规范支持。
