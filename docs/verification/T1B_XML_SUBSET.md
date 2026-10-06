# T1b XML required-subset candidate

This is an implementation checkpoint, not parent acceptance, an independent
code/demo approval, complete T1, or permission to enter T2. It starts at the
parent's unpushed local documentation baseline `72988bf` (not `origin/main`).
The main README, CLI contract, development plan, dependencies and S0 assets
remain parent-owned and unchanged by this product increment.

## Shared processing and scope

- Publication summaries, navigation/references, metadata/content writes and
  history/source reconstruction use `internal/xmltext`. Original UTF-8/UTF-16
  bytes, BOM, declaration and non-target bytes are not reserialized.
- Internal ELEMENT models, ATTLIST types/defaults/#FIXED, general/parameter
  entities (including markup), and NOTATION have bounded grammar processing.
  This is non-validating XML processing: DTD validity constraints are not
  silently converted into well-formedness constraints. First parsed entity,
  attribute and notation declarations have deterministic precedence.
- Character-reference construction, general-reference bypass, attributes and
  balanced entity content are distinct contexts. Attribute defaults normalize
  when declared, not after forward declarations become available. Constructed
  CR is not physically EOL-normalized a second time.
- XML Fifth Edition lexical Names are validated separately from Go's older
  Appendix-B Name table. A byte-length-preserving internal name adapter restores
  original names before namespace, duplicate and structure checks. It does not
  alter source bytes or grant writable locations.
- Effective OPF version and actual manifest MIME govern policy. Appendix B's
  exact external DOCTYPE tuples are not filename guesses, catalogs, downloads
  or exceptions for external ENTITY declarations. NOTATION identifiers are
  declaration data, not read/execute authority. No external XML resolver exists.
- Unread entities retain literal unknown text and original raw-reference spans.
  Unknown namespace/required identity yields capability diagnostics rather than
  invented namespaces or XML malformation. Unread PE suppresses later entity/
  attribute registration under the non-standalone rule.
- Optional `xmlCoverage` reports original hashes, sorted BookPaths, unresolved
  reference/expansion origins and parsed notation identifiers in the frozen
  `data`/`value` locations. It does not widen the request's reading scope or
  implement the later five-dimensional capability schema. XML partial does not
  downgrade an existing business-level block. Content/search reject incomplete
  text; virtual entity text/markup cannot become a direct writable interval.
- Raw/decoded/expanded/work, declaration, entity-depth/replacement and existing
  depth/token/index budgets are separate. Internal-subset comments/PIs are
  markupdecl productions and count toward the 4096 declaration limit.

## Checkpoint evidence and retained failures

Persistent raw logs are kept outside tracked assets. Initial DTD red tests and
old blanket-DTD expectations are retained. The new Fifth-Edition Name probe
first failed Go's Name table and subsequently passed without changing bytes.
The first CLI partial-search probe incorrectly expected error `data` to be nil;
the existing envelope carries a zero-valued result. Its exit-1 log is retained;
the corrected probe checks the real exit-3 diagnostic and absence of residual
matches, without modifying that envelope behavior.

The parent's trailing-Misc report was independently reproduced on this working
tree: NBSP, numeric whitespace references, empty/space entity references before
or after the root produced 24 failures across UTF-8/UTF-16 LE/BE. CDATA outside
the root was already rejected (six positive rejection controls). Lexical S and
reference checks now run **before expansion**; all 30 rejection controls and
legal Misc/inside-root controls pass. The real pinned checker independently
reports RSC-016 for trailing NBSP, CDATA, numeric reference and empty entity.

Latest normal T1b-targeted invocation passed: CLI four tests (39.498 s), XML
thirteen tests plus fifteen UTF-8/UTF-16 entity-source fuzz seeds, navigation and
reference partial/block tests. Earlier complete xmltext/publication/metadata/
references normal suites passed; metadata's T1b-specific integration is exercised
by the CLI default-version metadata edit/history/accept/export loop. Actual
EPUBCheck 5.3.0 validates the defaulted OPF/PE/generated-markup control and both
content/metadata formal edit loops, with every ZIP entry compared byte-for-byte.
HTTP declaration/notation probes observed zero requests.

Full root normal/race/vet, live fuzz and authorized-original regression results
are pending at this checkpoint. Their raw terminal completion/exit evidence,
not this checkpoint's targeted pass, determines their eventual status. Parent
CLI acceptance and fixed-tree independent product audit also remain pending.

## Post-checkpoint corrections

The original `2d62ad0` tree subsequently completed root normal/race/vet and
both live fuzz runs with actual exit 0 and no tracked drift (entity-source:
177005 executions/46.014 s; encoding: 24139 executions/31.016 s). These results
do **not** cover the following product changes.

The parent's navigation counterexample is reproduced and corrected: an unknown
href/src or unread default cannot establish a target, existence, or a missing
link diagnostic. Attribute-specific certainty preserves known links despite
unknown labels or unrelated attributes; known missing targets and structural
blocks remain diagnosed. Twelve EPUB3/EPUB2 asymmetric controls pass, and the
parent's exact synthetic EPUB now returns a null unknown target/existence while
retaining its known sibling and original coverage identity.

The initial blanket generated-text write guard was an overrestriction, not a
contract limitation. Following the parent's explicit ruling, local replacement
now proves literal opening-tag boundaries and the entire literal closing tag
through source spans, independently of decoded body text. Known entity text in
a physical simple-text element can be replaced as one original inner interval;
no-op preserves references, and declarations/other references stay unchanged.
Virtual nodes, unknown content and non-simple content remain unwritable. Twelve
UTF-8/UTF-16 LE/BE controls cover empty-only/empty-ends, nesting, PE declarations
and defaults; the former fuzz assertion and empty-entity budget assertion are
corrected with independent byte expectations and genuine virtual-node controls.
All initial red and intermediate failure logs are retained.

Fixed Namespaces §7 also requires NCNames for entity/notation names and PI
targets, and QNames for DTD element/attribute names. Twelve independent initial
red controls are now rejected; qualified element/attribute names, colons in
identifiers/NMTOKEN values and Fifth-Edition names remain positive controls.

This increment passes complete normal xmltext/publication/metadata/references
suites and all four T1b CLI tests (38.895 s, real pinned checker), including
entity-text content and metadata history/accept/export with exact ZIP bytes.
New-tree root/race/vet/live-fuzz results and independent acceptance remain pending
until their own actual completion evidence is available.

## Actual retained-stream budget and DTD particle correction

The isolated `5183e40` tree subsequently completed its original root normal,
race, vet and two live fuzz stages: all actual exits 0, identity/status unchanged.
Normal CLI 358.053 s and race CLI 596.227 s included the authorized original and
pinned checker; entity-source fuzz executed 10119 cases/46.020 s and encoding
fuzz 24492 cases/31.015 s. This does not resolve the independent B2 finding or
cover the following changes; independent audit of the original tree was rejected.

B2 was reproduced without changing that checked-out tree: the original CR probe
had raw 7500072 bytes, retained decoder stream 37500068 bytes and work 12000000;
an independent quote-attribute variant had raw 6000078, retained stream 30000074
and work 9600000. Both were incorrectly accepted. The stream now checks actual
serialized bytes before append, removing caller-supplied logical-size accounting.
Nested content work counts actual retained output; generated attribute/default
serialization also counts its real intermediate bytes. Existing raw, decoded,
depth, replacement and index limits are unchanged.

Regression controls cover both overflows and the same declarations with one
use (constructed CR/quote values remain exact), exact 16 MiB append and one-byte
overflow with no partial append, nested serialized-content work, and explicit/
default attribute-work boundaries with final stream/index/raw still legal.
The unmodified observational audit probe now exits 1 at its assumption that
expand succeeds, because expansion correctly reports XML_LIMIT; that is retained
as rejection evidence, not described as a passing assertion-based test.

The parent's unused-declaration Mixed-inside-cp syntax counterexamples also
reproduced. XML [46]/[48] now allows Mixed only at the top-level contentspec;
three nested-Mixed negatives and four top-level/nested-children positives pass,
without enforcing DTD validity or changing the depth limit.

Complete normal xmltext/publication/metadata/references suites and four T1b CLI
tests with real checker pass (CLI 39.006 s), including exact ZIP history/formal
loops. This newest tree still requires its own root/race/vet/live-fuzz completion
and independent audit/acceptance; earlier tree results are not reused as approval.

## PUBLIC identifier matching only

The isolated `11a0fea` tree completed all five original stages with actual exit
0 and no HEAD/tree/tracked-status drift: normal CLI 394.749 s, race CLI
605.784 s, vet without diagnostics, entity-source fuzz 65803 executions/46.022 s,
and encoding fuzz 3787 executions/31.021 s. Those checks do not cover this final
small PUBLIC-matching correction or constitute independent acceptance.

The parent's XML-space PUBLIC counterexample reproduced as nine failures across
SVG/NCX/MathML and UTF-8/UTF-16 LE/BE. XML §4.2.2 now normalizes legal space/CR/LF
only in the match key. The allowlist, exact system identifier, original source,
stored identifiers and notation application output remain unchanged. Tabs,
illegal public characters and NBSP still fail PubidLiteral parsing; wrong PUBLIC
names and changed/spaced system identifiers remain policy failures.

All 72 asymmetric profile/encoding controls and complete normal xmltext,
publication, metadata and references suites pass. The parent's exact three
synthetic EPUBs also pass their intended CLI observations: canonical and
XML-space SVG produce no XML_POLICY, while wrong-name retains XML_POLICY.
New-tree verification and independent audit/acceptance remain separately pending.

## 父独立验收与本地集成（后续追加，2026-10-06）

**批准规定范围的 T1b，并已快进集成到本地 main，未推送或发布。** 受测／受审产品提交为 `d526d0765568231e06a662405b2f563c6d8fc1e3`，tree `64c0828d67364e6d532ff7b16bf7793231247548`；前置为父本地 `72988bf7add9a160178824b3b835db4eae9f91ad`，不是 origin/main。上文及 [导航修复检查点](T1B_NAVIGATION_CERTAINTY.md)的“pending”保留为各次交付时点记录，不覆盖其原文。集成后 `cmd`／`internal`／`go.mod`／`go.sum` 与固定受审候选无差异；其后只有本记录及三份主文档的状态／既有语义说明。

T1a 与 T1b 两批均通过，完成计划 §3.4／§11.7 规定的 T1 兼容与终端基础，不代表完整 EPUB3 编辑器、T2、多资源结构编辑、全规范五维能力、阅读系统或发行。S0 原批准仍只绑定历史固定输入，不创建新 receipt 或把旧批准挪到更新后的文档树。

### 固定树的父实测

Linux amd64 orb，Go 1.27.1、Java 17.0.20.1、固定 EPUBCheck 5.3.0。JAR SHA-256 `f7f96617c929371821609b88c8484d6dc9f24fe916499863c46094c5fb778a65`。普通和 race 均启用真实 JAR 与授权原书 opt-in；没有预处理原书或压制检查器诊断。

同一受管包装器按顺序执行，实际退出 0，末行 `T1B_PARENT_CANDIDATE_V5_FULL_CHECKS_PASS`，前后固定树干净；完成后才停止服务。

| 命令 | 实际结果 |
| --- | --- |
| `go test -count=1 -timeout=30m ./...` | exit 0；CLI 437.547s、workspace 190.588s、validation 354.418s |
| `go test -race -count=1 -timeout=30m ./...` | exit 0；CLI 601.994s、workspace 261.198s、validation 328.569s |
| `go vet ./...` | exit 0，无诊断 |
| `GOOS=darwin GOARCH=arm64 go build ./...` 及单独 CLI 构建 | exit 0；Mach-O 64-bit arm64，未实机执行 |

父独立 Python／ZIP harness 在同一候选执行 **504 JSON＋3 human＝507 次 CLI 调用**，组合实际 exit 0，末行 `T1B_PARENT_V5_COMBINED_PASS`。正例成功与负例按预期拒绝分开计，不把命令退出 0 一律当 EPUB 合规。调用构成为读取 83、未知导航 2、名称 7、实体文本修改 168、正式编辑闭环 62、外部读取陷阱 8、局部结构 13、T1a 回归 139 JSON＋3 human、旧版本兼容 22。

- 35 组读取矩阵包含通用／含标记／参数实体、默认与固定属性、属性规范化、首次声明优先、NOTATION、未知 PE 后声明、UTF-16 直接／嵌套原字节来源，以及 malformed／standalone／递归／根外 Misc、声明数 4096/4097、深度 16/17、替换次数 100000/100001 的两侧边界。
- 12 组 UTF-8／UTF-16 LE／BE 实体纯文本编辑，以独立编码的目标字节为预期，覆盖空实体、嵌套、PE 和默认值；声明、其他引用和目标外字节不变，no-op 原字节。五组真实 checker 闭环分别完成候选、accepted-only draft、接受／历史／正式 export 的精确 ZIP 对照。
- 13 个父自造 nav／NCX 局部结构控制全过：未知内容不伪报缺失，已知兄弟节点缺失、重复、非法子节点及未知旁的已知裸文本仍诊断；已知 href/src 不被未知标签抹掉。未知目标不产生 target／exists／MISSING，references 无伪造边。
- HTTP 与本机文件陷阱确认外部 DOCTYPE、ENTITY、NOTATION 不触发外部读取，不以检查器自带 DTD 为核心 resolver。
- T1a 回归覆盖 UTF-16 OPF 正式成功／strict warning 拒绝、大 UTF-16 XHTML 保真编辑与 HTM_058 正式拒绝、搜索限额／后置坏资源、候选漂移、精确历史／活动状态和真实 human diff；旧二进制实际生成 v1/v2 计划／任务，再由新二进制继续执行与正式导出，原计划字节不变。
- 未经预处理的授权原书完成查询、精确正文修改和真实接受／导出，53 个 ZIP 条目逐字节对照；原书 SHA-256 `91b9d80c84258c89f47f6faac43eff6477b7c6649140ac848b3dc78924761e4b` 不变。书籍、正文和完整原书 CLI 回复不提交。

编码方另在同一固定树完成六阶段，逐阶段 actual exit 与 combined-status 均为 0，前后身份相同、status 空。普通／race CLI 为 477.713s／575.281s（30m 上限），vet 无诊断；`FuzzT1BEntitySourceAndLocalReplacement` 10,703 executions／46.023s、`FuzzEncodedReplacement` 17,868／31.014s、`FuzzT1BLocalContentCertainty` 21,445／31.021s。父已核 41 项证据 hash，未把旧树结果算入本树。包装器完成后仅 sleep，后按受管服务 stop 停止，不冒称 sleep 包装器自行 exit 0。

### 独立审计、修复链与保留失败

[DeepSeek 审计线程](https://ampcode.com/threads/T-01a110c5-dd31-748d-83f3-1c398f01c097)的实际终局为 `scope:t1b-code-demo-final-candidate`／`decision:approved`，父核验 `complete/end_turn`、usage 模型 `deepseek-v4.1-flash`、原文及规范串 hash。输入 identity 为 `83541ddaee23926d483809cb3eed28105dfcef87443c179f9432fd2c91eb2e21`。这是原审计与逐轮修复复审链的最终批准，不冒称本轮重读所有未改模块或完成全产品复审。

原 `2d62ad0` 审计 **rejected**：B1 未知导航目标／默认值被当作确定事实；B2 CR／属性转义后的实际保留流绕过 16 MiB；B3 Namespaces §7 名称上下文缺失。`5183e40` 修复 B1/B3 及物理实体文本边界，`11a0fea` 修复实际展开／工作字节计费与 nested Mixed 文法。其复审仍 **rejected**：F1 PUBLIC 空白等价值误拒，F2 未知内容可生成所需节点时仍伪报结构缺失。`ac98b61` 修复 F1，最终 `d526d07` 修复 F2；旧决定、日志和输入身份未改写。

父在旧 `11a0fea` 上的普通 root 通过，但默认 10m 的 race CLI 实际 600.054s 超时；该轮 vet／Darwin 未到达，不算通过，也不声称超时等于 race 缺陷。上述新树显式 30m 检查单独记录。初始实体写入在旧审计中被分类为冻结契约下的保守限制；父后来明确裁定物理独立标签内已知纯文本可整段替换，拒绝生成边界／复杂／未知内容。主文档补述此窄边界，不将旧分类回写成当时已有批准。

最终审计亲自运行普通 7 包、race 4 包、定向测试、xmltext/publication vet、独立 CLI/checker；37 项 F2 控制、39 项 V1–V4 回归通过，旧预算红例报 XML_LIMIT，15,000,068 B 绿例 complete。F1 单元测试为 3 profile × 3 编码 × 8 变体共 72 项；**独立 CLI/checker 不是这个笛卡尔积**：直接 checker 只有 SVG UTF-8 九例和 SVG UTF-16 LE／BE 四例，共 13 次，NCX 三例／MathML 两例为 UTF-8 产品观察而未调用 checker。另有三次真实 `kepub validate` 的 canonical／XML-space 成功与 wrong-name 拒绝，以及实体文本的正式编辑导出。

父据脚本与日志指出原审计摘要覆盖表述过宽，审计者单独补充更正并保持 approved／身份不变，原报告与包零改。首次 fuzz 管道只取得 tail 退出码，虽输出 PASS／292,045 executions，不作为独立 go exit 0；第二轮明确取得 `PIPESTATUS[0]=0`，24,054 executions／46.018s。其夹具装配及 `error:null` 处理失败留在审计 scratch，不伪称已全部包含于最终证据包。

**无阻塞问题不等于 findings 为空。** O1：未知 label 保留字面 `&label;`；O2：未知 type 旁已有确定 toc 时继续读取确定 toc；O3：未知内容可能含 base，但当前已知 href 仍按已知结构解析，已知 base 则 blocked；O4：MathML unknown-vocabulary partial 为既有行为。父接受这些局部读取边界，partial 不承诺未知部分展开后的全书导航语义；不能据此授权结构写入。CLI 契约明确未知依赖与已知错误的区别，而不把 partial 提升为完整引用覆盖。

### 证据指纹及未测范围

父已逐项核验最终编码证据 41 项、最终审计证据 34 项，以及独立补充文件。私有 export／完整回复留在 `.agents`，不提交；摘要 hash 仅定位证据，不代替审查原文与实际测试。

| 证据 | SHA-256 |
| --- | --- |
| 最终产品增量 bundle（6,185 B，前置 ac98） | `a2920ab1a9d206b22cd6e0bc8e192c534ef288ccf07aedf767006e5b868a5c4a` |
| 编码方最终检查包（11,784 B） | `11a17de62d1bd733ac427c35aa902b700a828f7b00c0914f5cc8334b6c1a4ac6` |
| 最终独立审计包（63,172 B） | `a3f2a7cd97772a76eec6c6299791e50642a82dc9f4441da50090e132320c3022` |
| 实际审计 final（7,869 B） | `e66f3a22d5ae8ba1b932d55af8a6ab89145c18e73743ca64e53c4e0d6cfe770a` |
| 私有实际审计 export | `0585423b6e266fa1ccb660eccf7a4f8dfbe555b959b1046c916a042a5e4e9515` |
| 审计覆盖／退出码更正（2,831 B） | `b2d47b1982eac5ec4b3bc994827978622347a0a2ce6b3aabf730e84d88f39087` |
| 父 root 普通／race／vet／Darwin 日志 | `8b1965fd849dd752f5f0fe2ca10673df701a736d8d905eca2acb4ee93aef01e5` |
| 父 507 调用组合日志 | `1ec7f120098e621c4b0daef29650b34f86e5f82fa9e53dc82bf99636261d7e94` |
| 父固定候选证据清单（记录时 audit pending，原字节保留） | `26e9b44397fe07852c0e170928ed472c33e71918d8f11ed2f53b8a49575bc64c` |

独立审计者未取得私有原书或父精确 13 例；这些由父实际验收，不冒充审计者结果。名称矩阵中 DOCTYPE／ELEMENT 多冒号 QName 由产品拒绝而固定 checker 接受，记录检查器差异，不为凑一致放宽 XML 规则。未执行 Mac 实机、渲染、人工无障碍、官方 169 个阅读系统用例或 T2～T6；未调用 Droid、未发布安装包，也不关闭 Issue #3／#4。
