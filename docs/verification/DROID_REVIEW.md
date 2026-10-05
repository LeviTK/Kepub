# Factory Droid CLI 审查记录

日期：2026-10-05。独立 Linux orb，分支 `review/droid-opus55`。精确未发布基线 `c824003dcb31a8f02db6adbd3ff27cfb6266e1f2`，**不是** origin/main `77821e6b163f45f81134394873d35f3ede8774ad`。完整历史 bundle 实际校验、verify 后导入；SHA256 `003501aeb6bbb9a1a6714572421444982d370e07519c57d9f77a5c3d4dd29b56`。

**最终结果：R12 全新完整复审正常 completion／实际 exit0，当前精确契约下无剩余可复现实质缺陷。** 最后产品修复 `4ada306`，测试增量至 `2458e84`，父精确文档 `b2fdd2a`；所有此前已确认问题均修复或经证据确认并澄清原安全树边界，不再把 CJK／NFD／长 URI 兼容缺陷留作范围排除。普通／race／vet 和交叉构建证据、最后一轮覆盖限制见下文；这不是绝对无 bug、所有平台／电源故障或同用户 OS 沙箱保证。仅本地提交并以 bundle 交父集成，未 push／发布／部署。

## 工具、范围与真实完成状态

官方安装脚本先检查、下载 checksum 验证；实际 Factory Droid CLI **0.233.0**，全部调用显式 **`-m claude-opus-5-5 -r medium`**，stream init 核对实际模型／effort，没有替换。默认只读不足以运行反例后使用 `--auto medium -o stream-json`；Droid 不修改产品，测试写临时 clone。协调者独立复现、最小修复与回归，没有用自身审查或 Amp 模式冒充 Droid。凭据仅检查存在，不打印、提交；日志／提示留 `/tmp`。

范围是 README、CLI_CONTRACT §2.1/§2.2、DEVELOPMENT_PLAN、C3 记录所承诺的当前 CLI／共享 Go 核心全链路与平台变体；实验 SDK／GUI、未来功能、真实 Amp 模型实验和用户书籍不在范围。每项真正缺陷先红反例再修复；误报／未证实观察不放宽契约以“变绿”。

| 轮次 | 实际结果与后续 |
|---|---|
| R1 | 初次只读权限失败 exit1；续接同一会话返回三项完整报告但末尾 error／exit1。保留失败，绝非终局完成证据。三项独立复现并修复 |
| R2 | 新会话完整复审，completion／exit0，56 turns；三项新发现并修复 |
| R3 | 续接 completion／exit0，65 turns，零新发现；Droid 承认主要增量并沿用旧读取，**不计最终完整复审** |
| R4 | 新会话全量 completion／exit0，45 turns；三项新发现并修复 |
| R5 | 新会话全量 completion／exit0，49 turns；两项分类问题及独立证实的短写假设修复，父另复现两项边界 |
| R6 | 新会话全量 completion／exit0，47 turns；High 中断恢复遗留目录问题修复 |
| R7 | 新会话全量 completion／exit0，93 turns；旧 `ebeee9a` 树，不能覆盖运行期间新身份修复。仅保留全历史重推成本观察，见限制 |
| R8 | 新会话全量 completion／exit0，42 turns；`41ec893` 树，两项新发现修复；确认身份顺序及合法 restore journal 边界，不能收尾 |
| R9 | 新会话全量 completion／exit0，53 turns；`6c92608` 树，原报告一项Medium可用性发现，独立核实为安全树范围/文档笼统表述，详见下文；不删报告、不当最终零发现 |
| R10 | 固定克隆独立全量 **completion／exit0，108 turns**；无新增／未披露实质缺陷，但再次复现**已知Medium报告库存兼容缺陷**。不以“no new”偷换剩余零；继续真实差分、上游源码核对、NFD红例及修复 |
| R11 | 全新固定产品 `511e18d`＋父精确文档，真实0.233.0／同模型／medium init已核对；实际 **completion／exit0，57 turns**，发现两项可复现问题，继续修复／新完整复审，不计终局 |
| R12 | 新固定产品／测试 `2458e84`＋父 `b2fdd2a` 精确文档，实际0.233.0／同模型／medium init已核对；**全新完整 completion／exit0，53 turns，零剩余可复现实质缺陷**，无 error 事件 |

R2–R9 会话依次：`85790ac6-5c48-4758-8076-e5256971b6eb`、R3 续接同会话、`1014625b-3b43-4996-b8fc-3a782c571e78`、`73d433c7-19fa-4603-a7e6-b145c0da6152`、`da9ba7da-eeaa-4316-99d3-c9c7990ecc0b`、`35a98f55-a05d-4170-b56b-1d3b7c569318`、`b9af09c4-040a-4e24-bca0-4673dc265406`、`399dacfc-d47d-4d0d-b915-59d08546d025`。各轮均实际核对上述版本／模型／effort；长运行跟进同一 PID，没有因静默盲目重启或并发 Droid。

R10新会话 `81b7f0b3-9947-49ce-9f16-70a9ebdafda0` 实际init再次确认同模型／effort。固定临时克隆fixture `4335d37` 仅提交父文档，cmd/internal树与产品 **`6c9260892860365cd8ba11f9e0362c9bd1f2f70f`** 完全一致；不是远端基线。父 `7d42424` 文档SHA256：README **`972815e5ef516d4202ada5a34c82d6b1d7fd8b9938ab7c24a0305349d546403d`**，CLI_CONTRACT **`79712b15f7cd5de830654016907e9819a469acc918287204c745c10f47b4f3a6`**，实际核对与读取后才启动。文档由父拥有，不重复回传其提交；最终增量只交本记录。

R10耗时2786883ms，末尾无error事件，原PID实际exit0。确认全部7502行生产Go（含平台变体）与7554行测试、完整固定README／CLI_CONTRACT、计划实施边界及C1/C2/C3/M2-B记录已独立读取。结论严格为**无新增或未披露实质缺陷，仍有一项已知兼容缺陷**，不是绝对无bug，也不是字面零未解决。

R11 会话 `153141e2-b0f6-4c83-bd09-f1e8d1c41874`，docs-only fixture `e7886eb6c08f2879d7a0593067df90f0556fee9e`；cmd/internal逐树核对与产品 `511e18dabf8b2859d736f370a31e850f470c4123` 一致（`b9a7e59ad165f38df3d3031c6d705ae2a1b3abb9`／`f163de356da72770000e8bfbdc984feebb011472`）。父最后文档提交 `d3be189`，README SHA256 **`5d7cb433fefcf97fffe172e1ee6a47229ba157c03151073110059efa5ddd9c11`**、CLI_CONTRACT **`60448bb7d0fe2b5c8202c26960ba83440e32811d06fa293b44f9ff2e6ba72a03`**，实际核对／全文读取、父真实验收和本orb最新组合完成后才启动；旧 C2/C3 失败报告未改写。未将未跟踪审查记录送作独立判断依据。

R12新会话 `22b0c60a-a4ed-43ef-89b3-e9cd1e142100`，固定docs-only fixture `03f35b1d4b695178321ca31d34a1e9e83b232493`，产品／测试 `2458e84` 的cmd/internal树精确相同（`e5a43c345c8c8bb6332da013dd6b718623d71ee3`／`dad94faf92109b25e6225a9dffca3a21a7553616`）。父 `b2fdd2a` README SHA256 **`0e11985d61d442b399e54cfdedd85a529e1c444e88792da18a5933d51391ca9f`**，CLI_CONTRACT **`56cece3fbef7db143d7da525aa0c5c75346cc2e119a94b82261ead7214dd7f76`**；核对读取、最新完整组合成功及父独立验收后新会话启动，未用此草稿作审查依据。

## 发现、反例与最小修复

| 来源／严重度 | 独立复现与修复（提交） |
|---|---|
| R1 Medium，历史搬迁／TMPDIR | metadata/body 真 accept 后搬迁阻断 Open；body 历史重推依赖坏 TMPDIR 且复制全书。消费后的任务严格绑定身份、完整来源而非旧主机路径；publication.ResourceReader 有界根锚定重推，保留 manifest、版本、hash、精确写集及全树漂移，不省校验。`1648a893` |
| R1 Low，候选 mimetype | 两类 × 损坏／删除：真实 diff 已含变化却失败。保留完整实际 diff，提取失败明确 unavailable；accept 仍拒绝，reject 可用。`1648a893` |
| R2 Medium，活动／pending 搬迁 | 六个 metadata/body review/diff/reject/accept／中断反例及 moved settlement intent/pointer。已开始任务允许精确来源重推与原决策恢复；未消费旧 plan 的 Apply／WritePlanReport **仍 stale**，不重新执行 mutation。`a465126a` |
| R2/R4 Low，coverage／版本 | cite/usemap/manifest/itemid/itemtype、object@classid 未支持却 complete，改 honest partial/blocked，incoming 保留全局 coverage，不发明 URL/usemap 语义。真实 clone/tag 构建 clean release-tag 后 pseudo 误报 release；覆盖官方三种 pseudo（含 prerelease 基线）、dirty、tag HEAD。`a465126a`／`b24d3d43` |
| R4 Medium，failed／intent 漂移 | 两类 × failed/no-start 四红例；先核对完整失败记录／plan/checkpoint，再允许普通字节漂移 diff/reject，failed 永不可接受；no-start checkpoint 来自已验证 accepted，不把外部漂移当合法基线。五类失败记录篡改仍硬拒绝。`b24d3d43` |
| R4/R5 Low，I/O／拒绝分类 | 真实非 root chmod 失败，含 wrapped Path/Link/SyscallError 和父复现裸 Renameat2 errno；真实 I/O=6，unknown stored plan=4/INPUT_DRIFT（同文件 chmod0000 仍6），failed/legacy accept=4/TASK_CONFLICT，draft/rootfile 参数=2。不将全部失效输入归 I/O。`b24d3d43`／`16b682c`／`d7d00c2` |
| 父额外 Medium，已发布错误回复 ID | 父真实 CLI inotify＋16MiB fixture 在候选发布后撤销 checkpoint 写权限：旧返回丢 ID。错误回复保留此调用完整身份/intent/use 绑定的 durable ID；未发布不返回 phantom ID，无 latest/别的 plan。父仅凭返回 ID diff→accept 拒绝4→reject→原文。公共 Apply 的 RLIMIT 子进程回归也证实，故意丢 ID 的 clone mutation 真失败。`16b682c`，test-only `2e6fb34` |
| R5 假设独立证实 Medium，记录短写 | 真实 RLIMIT_FSIZE2048，metadata 大新值／父旧 body schema2 状态在恢复 start/result 留精确2048字节，解除后永久 EOF。writeJSON 同目录完整 temp＋fsync＋no-replace 原子发布；新版本同限不半发布，解除后 read/diff/reject。叶 symlink／已有文件不覆盖；旧已截断记录仍硬拒绝，不猜补。`d7d00c2` |
| R6 High，中断回滚再中断 | Droid 400MiB 实际 kill apply／恢复复制，后续永久 IO6；两类 × partial-copy/unpublished-journal 红例。仅把 clearStaging 移到 verified settlement/checkpoint/restore 后、execution 前，未提交垃圾不阻下一回滚；坏 journal/checkpoint 留证据。多次重开精确回滚、failed/not_run、不重执行、diff/reject 成功。`ebeee9aa` |
| 父额外 Medium，恢复先写后拒绝 | 父10旧状态／40调用；协调者26红例：两类 no-start/no-result × missing/malformed/wrong-task/digest/version-use、wrong-start、outer downgrade。已有来源核验移到合成checkpoint/start/restore/result前；legacy 可无 use，**已有 use 必须绑定**。新版全树哈希不变拒绝；两 legacy 正例与16 journal 对照通过。`41ec893f` |
| R8 Medium–High，合成 start 双中断 | 两类真实合成 start 后删除 result 两次，旧下一 Open 错 restore 删除外部漂移；父21调用以真实 RLIMIT 在两记录间 EFBIG 复现。新合成 start 持久 **unstarted** 并同步，只有真实 running 中断回滚；unstarted 只能 failed，execution／taskDigests 同约束，连重试候选SHA不变、diff/reject／归档重开通过。`6c926089` |
| R8 Low，JSON 模式 | `--output -- --json` 及值为 `--json` 的相反输出误判；七反例（成功parse/早期错误/等号/真正分隔符）先红。成功采用 o.json，错误 fallback 同样跳过值参数／尊重--；不改变合法路径值语法。`6c926089` |
| R9 原报告Medium，unsafe候选阻断Open | Droid真实binary添加symlink后，diff/reject/content/export全部IO6；只移除该entry后reject成功。行为实际存在，但M2_B:159原已明确安全文件树才可重开、路径逃逸/坏记录仍拒绝；HashTree、Open和既有unsafe/FIFO/collision测试也故意fail-closed。父确认不扩unsafe-tree审计/归档新模型、不绕过Open，而真实澄清README/CLI_CONTRACT过宽措辞及人工恢复边界。独立metadata/content sentinel对照普通0.184s/race1.507s PASS：两次拒绝后候选目标、start/result、accepted/checkpoint哈希及外部sentinel/链接证据不变，仅移除新增链接后diff/reject成功；scanTree Lstat/default拒绝、不打开链接目标。保留此可用性限制，不宣称该条件能直接拒绝或accepted命令不需全工作区核验 |
| R10 已知Medium，报告库存兼容 | 真实合规 EPUB2/3 的 encoded CJK href 正式 checker exit0，却 Kepub inventory mismatch／incomplete，阻断 accept/export。初始“库存名需要解码”解释被 raw report 和官方源码**反证**：每条 ZIP raw 名／size／checksum 完整精确对应，额外的是零 size、null checksum 的 feature-only 行。NFD 进一步真实证明同时出现完整 raw `café.xhtml`、plain NFC `café.xhtml` 和 URI NFC `caf%C3%A9.xhtml`，不能仅解码查 raw。`360b9cc` 保持完整库存逐字 raw 精确一对一，要求明确 size 和字符串 checksum；仅四字段明确 default（两 size=0、两字符串=null）的辅助行可按冻结 raw 派生的 decoded NFC／canonical URI NFC 唯一关联。未知／歧义／重复／部分证据拒绝，辅助行永不填补库存；不做 exact-first、多次 decode、规范化库存或放宽 checker。反例还独立发现零字节库存缺失 size 的默认0漏洞并同批修复 |
| R11 Medium–High，HTML URL边缘空白 | 真实checker exit0但stylesheet `href="style.css "` 被本地解析为尾空格文件，validate/draft pack/accept/export阻断；独立旧binary再现exit1/backend0，新版exit0/backend0。只对XHTML href/src/nav.href及EPUB3 Navigation.link剥除两端ASCII空白，Edge/Node仍保存原Href，不全局改变BookPath/OPF/NCX/CSS/SVG。字符引用tab/CR/LF、fragment、src反例先红后普通/race绿，%20/NBSP与NCX对照保持。`4ada306` |
| R11 Low–Medium，Create策略拒绝分类 | 目标存在及源symlink/hardlink/directory旧均IO6，独立红例证实。只在Create早期及最后no-replace发布的拥有边界把目标存在转OUTPUT_EXISTS2，保留errors.Is ErrExist；openRegular策略sentinel仅Create源拒绝转INVALID_ARGUMENT2，其他内部调用仍原fail-closed错误。真实缺source/parent等OS错误仍由CLI默认IO6，无全局ErrExist映射。`4ada306` |

**恢复边界澄清：** 无已发布 restore journal 时，损坏身份／use 在任何未提交自动恢复写入前拒绝。但合法已发布 journal 已授权精确 checkpoint/hash，先按原规则核对 task/checkpoint/staging 并完成**仅原 rollback**，再拒绝坏 execution；不产生新 edit-result、不重放。不将 execution 提前到 pub 暂缺的 roll-forward，不推翻持久决定。合法／pub暂缺／malformed／wrong-hash journal 16 对照验证，坏 journal 全树不变、证据保留。

**不可消除的旧窗口兼容边界：** 升级前合成 running 与真实 running 字节形状相同，不能猜测区分；保留旧 running 回滚语义。新 unstarted 防之后双中断，不声称可恢复旧窗口已经丢失的外部漂移。metadata v1 摘要、content schema2/execution2、单操作与正式 checker 门槛均保留。

## 验证与独立证据

R12最终耗时 **1421472ms**，原PID实际exit0，completion与stream末尾均核对。原固定克隆HEAD／cmd/internal／两文档hash重新核对完全不变、工作树干净。Droid全文独立读取全部生产Go及平台变体、完整README／CLI_CONTRACT；DEVELOPMENT_PLAN仅§5／§11.2–11.6，测试仅create_policy、workspace、validation integration及alias相关，没有全文多数测试／testfixture。独立root普通 **101.751/126.006/147.009s**（CLI/workspace/validation）、vet及Darwin全仓build全部通过，真实checker用例运行；本轮**未跑race**，也未做新真实kill/电源故障或性能量测，不把协调者／父race冒充Droid自己的运行。

R12真实CLI闭环：pack→open→BOM同文两节点content定位→正文schema2修改第二节点→plan/apply，重apply明确4/TASK_CONFLICT；diff实际值／matchesExecution正确，accept前content匹配新文0，真实accept成功、正式export成功。协调者再独立比对两个ZIP的**8项完整文件／目录集合及字节**，仅第二`Same &amp; text`替换为`&lt;b&gt;new&lt;/b&gt; &amp; 中`，原书SHA不变，打印R12_INDEPENDENT_ZIP_BYTES_PASS。脚本cmp对预期修改的目标本来应不同、确有差异输出，不以其失败作为全部资源相等；独立逐项精确预期比对另通过。脚本意图CRLF但输入单行未产生CRLF，Droid主动注明，此轮BOM/局部字节通过**不是新的CRLF实验**。

R12追加真实checker探测：CJK brackets／special字符／lowercase percent／%41／./x/../非canonical href通过，aux来自上游派生原名；内部literal tab Kepub INVALID_REFERENCE、checker RSC-020双拒绝，无误接受。初始特殊字符夹具遗漏XML转义`&`实际XML_NOT_WELL_FORMED／checker1，修夹具才是正例，保留失败。unshare探测实际`userns-ok`成功：协调者一次进度消息曾误猜不可用，查真实tool_result后已更正，未作为最终证据。

R11实际耗时1675440ms、无error事件且原PID真实exit0，不能以此两发现轮收尾。Droid独立阅读所有生产代码、README/CLI_CONTRACT，计划§10–12/C3为skim；clone普通root/vet及workspace/cmd/app/archive/publication/xmltext/metadata race通过，**未跑validation race**。首次Darwin `go build -o singlefile ./...` 因多包失败，后来拆分全仓/CLI build及vet通过；未做Mac实机、新电源故障或性能量测。unmanifested CSS检查严格性仅策略观察；发布前内部错误分类是假设，未宣称已复现。

R11修复后的首轮root组合实际exit1：既有CLI lifecycle测试硬编码重复workspace open=6，与本轮明确的新OUTPUT_EXISTS2契约相冲突。test-only `f3a7f4c` 将其改为2并强化code断言；不是抑制产品失败。父同树旧组合也同样失败，双方均等原PID结束后再跑最终静态HEAD，不把中间失败称PASS。定向普通/race已通过，最终新组合另记。

`f3a7f4c` 最终静态树root普通CLI/workspace/validation **100.840/125.798/146.974s**、root race **93.967/146.961/150.383s**、vet无诊断，原PID实际exit0／R11_FINAL_COMBINED_CHECKS_PASS。Darwin arm64全仓build/CLI/workspace test-c均成功，Mach-O未运行。之后只加强NCX控制的test-only `2458e84` 定向普通 **0.027s**／race **1.060s** 另跑通过；先前%20控制不能捕捉错误global trim，改用真正尾ASCII空格并精确断言raw/target/Exists=false。旧root进程未执行新控制，产品代码完全不变，不混称其覆盖了最新测试。

父对应静态树 `39a561b` root普通 **122.547/152.465/177.372s**、root race **115.974/172.986/177.836s**、vet无诊断、Darwin全仓/CLI/test-c均实际exit0／R11_COMBINED_CHECKS_PASS，Mach-O未运行；随后才合入NCX新控制至 `9c30d38`，独立定向普通 **0.007s**／race **1.054s** exit0。当前产品及测试双树完全匹配R12，父38个文档链接/锚点通过；不把其旧root进程算作执行新增测试。

新NCX控制的实际敏感性：父仅用Go overlay把Navigation.link的epub3-nav条件变为if true，定向测试真实exit1／`NCX URL space was trimmed`，不是编译失败；原树未动，变异文件已清理，失败日志保留。

父在 `4ada306` 同产品 `08a12ef` 独立 **33次**真实CLI全部通过：EPUB2/3 validate/backend0，stylesheet/src/fragment/external，原Href、NBSP与%20不同真实路径，EPUB3导航精确目标；各一draft和正文accept/export，**四ZIP**逐资源/目录/原ZIP SHA核对。已有file/dir/link输出OUTPUT_EXISTS2；源link/hardlink/dir/FIFO INVALID_ARGUMENT2；缺source/parent仍IO6，无残留。父脚本初始误取inspect.data而非data.value产生KeyError，修后从未发布状态重跑，保留工具错误不计首次为通过。

R10修复稳定产品 **`511e18d`**：本orb root `go test -count=1 ./...` 普通 CLI/workspace/validation **96.886/118.846/140.279s**、root `go test -race -count=1 ./...` **89.994/139.459/143.589s**、`go vet ./...` 无诊断，全部实际 **exit0**；Darwin arm64全仓 build/workspace test-c成功，Mach-O未执行。父同树local `368169c` 独立普通 **127.241/157.806/182.208s**、race **117.335/173.485/179.614s**、vet无诊断与Darwin build/CLI/test-c全部实际exit0。不是以旧360或6c结果替代。

父最新独立验收 **52＋20＝72次**真实 CLI、EPUB2/3 各 CJK／literal%20／NFD／1539-byte raw长路径，**8次正式接受、8个正式ZIP**逐文件／目录／原ZIP SHA精确比对通过；4545-byte URI辅助表示不再误拒。早先旧binary与不完整半修的失败保留；不靠重建fixture或draft降低门槛。

`.agents/setup` 实际执行成功；bash -lc 下 **Go1.27.1、Java17.0.20.1、EPUBCheck5.3.0**。每批按改变执行定向普通／race；稳定树多次 root `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 全 exit0。新增 R6 红例编辑期间尚运行的旧组合实际 exit1，明确不是最终证据。历史 `6c92608` 组合全部 **exit0**：普通 CLI/workspace/validation **113.591/143.405/154.921s**，race **108.962/165.106/155.753s**，vet 无诊断；`GOOS=darwin GOARCH=arm64 go build ./...` 及 workspace `go test -c` 成功，Mach-O未执行。

有意义 fuzz：`FuzzContentSimpleTextReplacement` **112878**、`FuzzXML` **62028**、`FuzzSimpleTextPreservation` **68203** 次 PASS（合法构造＋独立局部替换／非目标字节校验）；R3 Droid另15832次正文 PASS。R7 Droid 40随机 apply SIGKILL（bad0）、12 pre-settlement＋14 inside-settlement accept kill 恢复/再接受/导出；初始 export 参数错误失败保留解释，不冒充全通过。R8 clone vet及核心/CLI普通 PASS，未跑 race，Linux且 crash窗口以文件状态模拟，并非电源故障证明。

R9新读**7502行**全部生产Go（含平台变体）、完整CLI_CONTRACT及相关README/计划/C3，深入recovery/lifecycle/process而非每个test全文。clone各核心与CLI包普通、recovery/concurrency/interruption race子集、vet、正文fuzz **34478次** PASS；真实CLI正文→plan→apply→diff→真实accept→export逐资源核对（仅预期文本和显式目录条目变化）、旧plan stale、普通字节漂移diff/reject通过。不是单次root/race全仓，不是Mac实机/新电源故障实验。新版unstarted/result篡改与合法journal边界均拒绝错误来源；非Linux/Darwin编译失败仅不承诺平台观察。编码中文href仍明确校验失败，未放宽。

R10独立clone全仓 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 全PASS／无race诊断，真实checker依赖用例未跳过；race workspace约139s／validation132s／CLI92s。Darwin arm64 build成功，仅交叉。1206文件＋RLIMIT_NOFILE1024反驳FD耗尽假设：snapshot的defer在WalkDir回调内、每entry即关闭。`-cover`测量确有失败：两个故意RLIMIT_FSIZE子进程无法写Go覆盖率meta文件（EFBIG），其中产品断言打印PASS；相同两测试plain及race实际通过，保留测量失败，不当产品bug或全绿coverage。未运行真实Amp／用户书／GUI。

R10之后独立差分／修复证据：先真实 EPUB2/3 红例，逐项列 raw inventory 与 extra rows；官方 v5.3.0 OCFZipResources/OCFChecker 原名库存、CheckingReport/ItemMetadata exact-key＋primitive0/String null、OPFItem decoded NFC、ValidationContext/OPSHandler encoded path 均核对。URLUtils.encodePath 官方测试明确 literal `a%20space`→`a%2520space`，不能把 literal% 与空格折叠。直接在已固定官方 JAR 调用 URLUtils/Galimatias 0.1.3，核对全部 ASCII 保留集合及 NFC/%/空格/+/#，使用上游 `!'()` 保留规则而非 Go EscapedPath；最初临时 probe 自定义 opaque scheme 报 NPE，改标准 HTTP 后才得到真实有效数据，不计前次为成功。正例最初带 `*` 被真实 PKG-009 拒绝，去掉非法字符才是合法夹具。第一版单次解码→raw 修复在新增合法 NFD 上真实失败，未启动全量审旧树；最终采用派生关联。32项 adversarial 报告矩阵、EPUB2/3 原 NFD 完整库存＋两种 NFC aux 明确断言（含保留标点与 literal%/空格组合）、`?`仍正式 exit1/PKG-009，原档案 SHA 不变；最终定向普通 **9.199s**／race **10.555s** PASS。原先只有普通 URI 的包级普通85.259/race86.889s PASS不当最新 NFD 完整验证；稳定树组合与完整R11另记录。

父对 `360b9cc` 同树 **52次**真实 CLI：EPUB2/3 各 CJK／literal%20／NFD 正文共6次正式接受、6个导出ZIP逐文件／目录／原书SHA精确比对通过。随后父与协调者独立同时发现新增长表示边界：安全 raw 路径约1.5–1.9KiB，URI辅助名4.5–5.5KiB，仍 backend exit0／完整 raw 库存精确，但把辅助名再套 BookPath 4096-byte 限额错误拒绝。unit及两版真实checker先红，`511e18d` 仅移除此错误检查／import，已冻结安全 raw 的唯一派生关联不受 URI膨胀限额影响，也不增加任意别名接受。显式跨4096并核对实际辅助行的普通 **9.717s**／race **10.207s** PASS。此前360树根普通 **97.853/121.223/143.316s**、race **91.580/141.951/144.942s**、vet／Darwin全exit0是中间证据，不冒充511最终组合。

父始终冻结产品代码，用**旧真实二进制保留状态**独立复现→新二进制验收：R1 **27→18**、R2 **30→24**、R4 **25→23**、R6 **44→40** 次调用；权限/inotify/startup/正文v2真实短写互补，原书SHA及导出逐条目独立字节核对。身份顺序10旧坏状态→20新拒绝，整树hash不变。父同 `41ec893` root普通（CLI/workspace/validation **131.666/162.068/175.655s**）／race（**127.751/192.085/183.479s**）／vet全exit0；本orb同树 **144.478/176.492/190.455s** 与 **139.819/199.289/193.948s** 全exit0。两处 Darwin arm64全仓 build及workspace test-c成功，Mach-O **未执行**。

父最终 local main `4263125` 的 cmd/internal 与审查树 `6c92608` 一致；独立 root普通 **151.085/184.562/196.578s**、race **134.235/191.217/183.094s**、vet全部exit0，Darwin build/test-c成功未运行。R8两类真实RLIMIT、8种输出模式、**20次**修复版CLI验收通过；旧running歧义按上述边界保留，不将旧状态已经丢失漂移当作能修复。

## 未确认观察与限制

- R7 3000章、0/1/2/3历史 content约0.40/2.70/4.66/6.94s：全历史重散列成本，未证明违反已写性能契约，目标尚未量测且并发测试负载。保留观察供后续优化，**不能省历史/manifest/hash验证**或扩成无关重构。
- R7 未证实 mid-command I/O归stale、Unicode空白解释，以及未承诺Windows/FreeBSD；R5 tagged prerelease并非pseudo，development:false不违反tag语义。未把未验证假设宣布为因果或零风险。
- R12未实证观察：不同命令已存在output的code不同但均exit2，属于本地既有分类；非checker deadline映射、approval失败的可达性未复现；XML读／编辑解析器对未知prefix严格度不同，编辑fail-safe。workspace别名假设中，symlink路径由openDir逐祖先Lstat拒绝，既有OutputBoundary包含candidate symlink别名反例，协调者定向普通0.035s再通过；**bind-mount别名仍未实证**，不说已排除，也不将假设称已确认缺陷。无恶意同用户文件系统重配置的OS沙箱承诺。
- 编码／CJK／NFC／长URI报告兼容缺陷已用真实反例修复，稳定树R12实际完整复审通过；此前 C2/C3、R10以及各轮失败保留，不改写历史为通过。原已损坏记录硬拒绝。无Mac实机／电源故障保证，不宣称绝对无bug。
- 最后全量Droid与稳定产品检查已完成；产品／测试增量已以bundle由父本地集成，最终本记录另交报告-only bundle。未推送、发布或部署，父仍负责最终交付核对。
