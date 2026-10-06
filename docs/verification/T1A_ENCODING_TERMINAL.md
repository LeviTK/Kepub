# T1a：原字节编码与终端查询

2026-10-06，Linux amd64 orb，Go 1.27.1、Java 17.0.20.1、固定 EPUBCheck 5.3.0。这是产品实现与本 orb 检查记录，不是父独立验收、模型批准、完整 T1 或发行声明。

## 基线与所有权

父未发布 main `40640722f5504f686fd08511d768d353ca7df0b6`（tree `89390b3db34b2ee538cae6205059e8994487fc94`）由实际 bundle 导入，在持久 `t1a-approved` 留存。836 输入逐项验证，identity 为 `1e3055f507824e62e58e51380b509347c8bc22bc9413a30a27c072705b0ac1eb`，私有收据及原始导出仅供该历史 checkout 的实际 gate exit 0；没有提交或把旧批准用于本产品树。原 root 初稿、R5/R6 记录均保留。

实施树采用父精确文档补正提交 `d8788d4b41893ac120551c619ae5bd1d85789bad`，不重复交付其文档提交。CLI 契约 SHA-256 为 `1b2c516372eefb8e7b45a33515e4bc07961896aabda2321b495b4fadfb20d801`，计划为 `a4ecf838476ba603c982cf891c852c86589781652bbbfbf67daae75c1f084879`。产品检查点 `f620a53fbe0e69d37bc26f213b2a4d578d1ee82e`（tree `2ae7bc418cfe2e20884202a5b316151a741c1a20`）仅修改 cmd/internal；其 bundle 的真实 ref 为 `refs/heads/t1a-product`，prerequisite 为上述父文档提交。

## 实现范围

- publication 读解析与 metadata/content 局部编辑共享原有严格 RawToken 命名空间索引。新增严格 UTF-8／UTF-16 LE／BE 检测、声明匹配与代理对验证；UTF-16 转为解析用 UTF-8，同时建立原字节映射。替换只编码选中简单元素的内部文本，保留 BOM、声明、标签、目标外和未改资源字节；更改后重新解析并验证实际目标值。
- native `<!DOCTYPE html>` 可读取和保留；其他 DTD／内部实体仍为 T1b 未实现能力，报 `UNSUPPORTED_XML_DTD`（exit 3），不调用外部 resolver。非法编码／声明为 `XML_NOT_WELL_FORMED`，不再笼统称 UTF-16 unsupported。这是明确的诊断兼容变化，不声称旧错误码不变。
- raw 8 MiB 含 BOM、decoded 16 MiB 与原有深度 128、200000 tokens、32 MiB 索引分别计量。索引保留字符字节两次、各祖先聚合与 location 计费；decoded 上限不是正文容量承诺。T1a 无实体展开；在 raw 8 MiB 内纯 UTF-16 转码最多约 12 MiB，故解码跨 8 MiB 正控不声称触到了不可达的 16 MiB 边界。T1b 的实体流／工作预算本批尚未实现。
- `search` 持锁仅扫描所选 rootfile 的 accepted manifest XHTML，复用 content 节点／排除规则。按 manifest 顺序返回，达到条数限制仍扫描／统计后续元素；累计原始扫描 128 MiB、返回文本 1 MiB，坏资源或超限明确失败，不伪造完整匹配数。
- `task status` 查询精确活动或历史 taskId，验证来源与已消费计划，区分 pending、failed、candidate_drift、review_required、accepted/rejected；不选择 latest。检查尝试逐条列出，不以随机 audit 文件名声称时间顺序。
- 非 JSON 输出为转义控制字符的终端摘要；diff 对真实 before/candidate 字节重读并核对 hash，按文件显示实际文本变化。单资源 256 KiB／累计 1 MiB 文本显示预算；二进制、不可解码或超预算明确省略文本而保留路径、类型、大小与 hash。显示不是可应用 patch，不把计划值当实际候选。
- `metadata.set` v1、`content.text.set` v1 结构权限、params 顺序、schema/plan/execution 版本及既有 policy 摘要未修改。没有扩大复杂内容／属性／任意文件写入。

## 规范与真实 checker 分层

直接核对固定 XML §2.8／§4.3.3 的声明和编码规则、Namespaces 保留绑定及 HTML XML §14.1 的语法／DOCTYPE／外部实体说明。EPUB3 §3.9 的一般 XML UTF-8/16 与 §6.1.2 继承 HTML UTF-8 条款分开；共享非验证 XML 解析不冒充 HTML 或全 EPUB 合规判定。

匹配 BOM/声明的 UTF-16 XHTML 核心解析与编辑成功，但固定真实 JAR 正式 accept/export 因 **HTM_058** 失败，accepted 不推进、不生成正式产物；显式 draft 保留原 accepted 文件字节，没有转码／降级。LE／BE 的大 CJK 注释样本 decoded 超过 8 MiB 而索引合法，仍完整经历编辑、重算、重开、checker 拒绝、draft 字节对照及失败回滚。

声明指定相反端序时，核心报 **XML_NOT_WELL_FORMED**，真实 JAR 另报 **RSC-016**（fatal XML parsing，可能同时报 HTM_058）；不能将其等同于匹配编码后的 HTML 限制。该两类原始 JSON 保留于定向日志。UTF-16 OPF/container＋UTF-8 XHTML 的 metadata 修改则非 strict 正式接受／导出成功，RSC-027 warning 在 strict 下仍拒绝；LE/BE 均验证历史重开与 ZIP 每资源原字节。

## 独立预期与实际检查

初始独立红例保留：旧代码拒绝 UTF-16 局部替换和大注释，并接纳错误 XML 声明；search/status 尚不可用。UTF-16 预期由测试独立编码器及明确 prefix/suffix 构造，不由产品 decoder／offset map 生成。端序、BOM、声明、代理对、同文第二元素、原始 8 MiB／+2、编辑后超限、CJK 正文和多层聚合索引反例都执行。新增 encoded fuzz 实际做成功替换并逐字节比较；既有 content fuzz 改用直接标准 xml.Unmarshal 作独立文本预期，而非已合并的 publication 解析器。

授权原始附件 SHA-256 `91b9d80c84258c89f47f6faac43eff6477b7c6649140ac848b3dc78924761e4b`、3328634 bytes，仅留私有目录。没有去掉 DOCTYPE 或修改原书。opt-in 二进制测试实际完成 literal search、单资源编辑、reject、第二任务真实 accept、历史重开、正式 export、全条目逐资源字节比较及原附件末次 hash；正文不入仓库／证据报告。

检查使用持久 TMPDIR 与无 portal 的 managed service `t1a-checks-v2`；原子 mkdir 守卫防自动重启重复检查，不以此承诺抵抗整机重建。产品在该轮检查期间未改变，提交检查点不重启已运行过程。

唯一原包装器 PID 85011 从 12:34:08 UTC 跟至 12:58:38，九步均实际 exit 0，`complete.exit=0` 且服务首次记录 Deactivated successfully。命令／原日志／各步 exit 均保存在持久 `t1a-runs/checks-v2`；下述检查全部针对上述同一产品检查点，不包括父仍在进行的组合或独立审计。

| 实际命令 | 终局 |
| --- | --- |
| `go test ./internal/xmltext ./internal/publication ./internal/workspace ./cmd/kepub -run 'Test(Encoded\|MalformedEncoding\|RawBoundary\|Search\|TaskStatusExact\|TaskStatusExactAndHuman\|T1)' -count=1 -v` | exit 0；CLI 133.433s |
| 上述定向命令加 `-race` | exit 0；CLI 368.554s |
| `go test ./... -count=1` | exit 0；CLI 302.467s、validation 277.933s、workspace 163.347s |
| `go test -race ./... -count=1` | exit 0；CLI 510.516s、validation 285.691s、workspace 213.280s |
| `go vet ./...` | exit 0，无诊断 |
| `go test ./internal/xmltext -run '^$' -fuzz '^FuzzEncodedReplacement$' -fuzztime=30s -parallel=2` | exit 0；126448 executions |
| publication 同参数 `FuzzContentSimpleTextReplacement` | exit 0；89429 executions |
| publication 同参数 `FuzzContentLiteralAndExclusion` | exit 0；592487 executions |
| publication 同参数 `FuzzXML` | exit 0；279199 executions（既有无 panic／非空根探针，不称自然语言或 XML 完整语义证明） |

全部普通／race 流程均设置真实 checker 及授权原书路径，未跳过上述 opt-in 原书闭环。固定 JAR 文件 SHA-256 为 `f7f96617c929371821609b88c8484d6dc9f24fe916499863c46094c5fb778a65`；既有完整工具集合 fingerprint `158b7c2778c3b64d5dbd87151b7d1f870b03beda3bee973d52b3fe58bea16266`／backend 参数未修改。文档补充不修改产品代码，不重跑／冒称改变后的代码已经受测。最终交付只追加本记录，仍待父全组合和独立代码审计。

## 保留失败与剩余范围

- 初次红例 test encoder 使用不存在的 ByteOrder 方法、首次 green 未使用 import，均为测试构建失败，修正后保留原日志。
- 一次定向断言误认为 immutable-source drift exit 4（实际既有 IO exit 6），另一处误称 diff kind removed（实际 deleted）；修正测试预期，不改既有错误行为。
- 新 checker 对照最初预设 `RSC_005`，真实返回 `RSC-016`，导致 checks-v1 exit 1；同时合成 ZIP 的 mimetype 顺序／替换正文丢链接带来额外诊断。之后仅修正预期、条目顺序与只改声明的样本，保留原日志；没有弱化正式 checker。
- checks-v1 服务在失败后尝试重启，原子守卫拒绝，没有重复测试；随后停止该服务。checks-v2 完成后即使守卫 exit 0，受管服务仍按其 restart 策略反复启动包装器，最后 start-limit-hit；原检查没有重跑，该管理状态不等于测试失败。首次完整执行的九步 exit 0／complete.exit 与后续守卫日志分别保留，已显式停止服务。没有宣称 exit 0 可以阻止该服务重启。
- 文档 bundle 初次省略 ref 的 fetch 失败，显式 ref 成功；从持久目录直接 upload 被工具的 workspace 限制拒绝，复制新证据到 root .agents 后真实上传成功，未改 root 初稿。
- 首次证据 tar 将输出置于被归档目录，虽排除自身文件，仍出现 `file changed as we read it` 的目录 mtime 警告；没有记录该次 tar 独立 exit，不把外层命令的 exit 0 充作 tar 通过。改为目录外生成独立 v2 包，检查其实际 exit／解包清单；原首包与警告记录保留。这只是包装修正，产品和检查日志未改变。
- T1b 内部子集、实体、默认／固定属性及媒体绑定外部标识符仍待下一批；T1a 不等于完整 T1。T2、UI／阅读系统／Mac 实机测试未执行。没有 Droid、合成批准、push、发布或自动接受。

## 父独立验收与本地集成（后续追加，2026-10-06）

**批准 T1a 增量并本地集成，不等于完整 T1 或完整产品树复审通过。** 上文“仍待”保留为编码交付时点的记录。父在 detached 产品 checkout `f620a53fbe0e69d37bc26f213b2a4d578d1ee82e` 独立构建、测试和验收；随后合并原产品及两次 verification 提交到本地 main，集成提交 `f8642f676e9aeae6195efe61ba7e803b73b9efc9`。合并后 `cmd`／`internal`／`go.mod`／`go.sum` 与受审产品逐项无差异；后续本记录与三份主文档仅描述实际交付状态和既有接口语义。未推送、发布或调用 Droid。

### 父实际运行结果

同一包装进程 PID 1450227 顺序执行以下检查，已实际 exit 0，最后输出 `PARENT_T1A_F620_ROOT_NORMAL_RACE_VET_DARWIN_PASS`。所有检查设置固定 checker 及私有原书 opt-in 路径，无需重跑相同产品树来将文档提交冒充新产品验证。

| 命令 | 终局 |
| --- | --- |
| `go test -count=1 -timeout=20m ./...` | exit 0；CLI 323.236s、workspace 177.851s、validation 298.758s |
| `go test -race -count=1 -timeout=30m ./...` | exit 0；CLI 524.333s、workspace 221.264s、validation 289.070s |
| `go vet ./...` | exit 0，无诊断 |
| `GOOS=darwin GOARCH=arm64 go build ./...` 及 CLI 单独构建 | exit 0；`file` 确认为 Mach-O 64-bit arm64，未实机运行 |

父用独立 Python ZIP／CLI harness 执行 **205 次 JSON＋3 次 human＝208 次调用**，正例成功、负例按预期拒绝；不包含早期三次失败 harness 的部分调用。完整成功集合为：UTF-8 原生 DOCTYPE 25 次，UTF-16LE／BE metadata 各 17 次，search 7 次，真实旧二进制 v1/v2 计划及任务 22 次，私有原书 11 次，UTF-16LE／BE XHTML 普通及大注释四组各 22 次，实际候选／历史状态／终端显示 21 次。

- UTF-8 样本真实拒绝／接受／历史／正式导出；UTF-16 OPF 正式接受，strict 保留 warning 拒绝，原编码局部替换精确等于独立计算字节。
- UTF-16 XHTML 四组覆盖 BOM、代理对、解码跨 8 MiB、大注释、局部编辑、失败恢复、历史重开；正式检查保留 HTM_058，accepted 不推进、正式无产物；draft 仅含 accepted 原字节。
- 旧二进制真实生成 v1 元数据／v2 正文计划及活动任务，新二进制继续接受、重开、正式导出，旧计划哈希未变；不是手造旧 schema 冒充旧版本兼容。
- 未预处理授权原书直接 search／精确节点修改／真实 accept/export，最终 ZIP 的 **53 个条目**与独立预期逐项比对，只有指定文本变化；原始附件 SHA-256 保持。正文／书籍／完整回复留私有目录，不进入仓库。
- search 为 literal、大小写敏感和 accepted-only；达到 limit 后的坏资源仍失败，返回预算失败不伪装截断成功。实际候选手动漂移后，终端 diff 显示五个变化文件及真实文本，二进制／超预算省略有说明；状态准确区分旧任务、活动任务和未知 ID，接受漂移仍拒绝。human 输出实际读取检查，非阅读器渲染证据。

父保留的三次 harness 失败分别为：ZIP 显式化原先隐式父目录导致比较预期错误；调用不存在的顶层 `metadata` 命令；将历史 reject 存档中的副本错误计为活动候选。修正预期／调用和查找范围后才计上述成功集合，没有为测试改产品。父证据清单最初记录“root／审计仍进行”，该文件原字节保留，本节追加实际终局，不覆盖历史状态。

### 独立 DeepSeek 结论及边界

[独立审计线程](https://ampcode.com/threads/T-01a110c5-dd31-748d-83f3-1c398f01c097) 使用 `deepseek-v4.1-flash`，实际最后 assistant 为 `complete/end_turn`，唯一决定 `scope:t1a-code-demo`／`decision:approved`；输入 identity 为 `e03797d865001725e8e145997c85a1c8078c5e19631f75dc9c89635c3d43c8b6`。父核验 actual final 与 export 中的模型／终局状态，不以另一线程的口头批准替代证据。

审计完整读取 26 文件 diff 与相关实现，不宣称 navigation／archive／validation 等未改模块全文复审；普通 5 包、race 4 包、定向 18 PASS／1 SKIP 及真实 EPUBCheck CLI demo 通过。私有原书因该 orb 无输入显式 skip，root／vet／fuzz／Darwin／旧二进制由父或编码方另列证据，不冒充审计者执行。审计保留两次自己的断言错误（human XML 转义、误以为 draft 导出候选），修正后 demo 全过。

**无阻塞性实质问题，但 findings 不是空数组：** 非阻塞观察是历史已结算任务 `matchesExecution` 固定 false；它只定义活动候选比较，不代表历史来源失败。历史仍核验 decision／revision／计划消费。父在 CLI §2.4 与 README 明示该语义，以及已结算 `task diff` 为 exit 4、draft 为 accepted-only；不以文档补述扩大执行权限或改变产品。

### 证据指纹与未测范围

父已解包校验编码方清单 31 项与独立审计包清单 32 项，全部 SHA-256 匹配。以下指纹用于定位保留证据；私有 Amp export、授权书籍及完整原书 CLI 回复不提交。

| 证据 | SHA-256 |
| --- | --- |
| 产品 bundle（24,114 B） | `d295685b09651faca1e09a6249a82a7bf9dec508ecac81e28da7809d6674c628` |
| 最终 verification bundle（6,904 B） | `11b6937d63944e5332ef12de4c3d9884c33681b418b78dd1bffbd8e1f278ed74` |
| 编码方 v2 证据包（26,719 B） | `a79f2d245bc5cfe3bd62ae5ca89763265c4767a41e640a0b681f352be652ac53` |
| 独立审计包（70,137 B） | `84ab583c6aeb5c49149ca7c17f1bfd83dfd13ce9627a497cefa99c194eb0246d` |
| 审计实际 final（8,028 B） | `91343a5964806986a9602e26274fada1b2fecc7ebe53151a5a533f31769dd3ae` |
| 私有审计 export | `e544e360a96ed0c2f09e2440ab09a4d668db929c58e6d824a061db43cf843572` |
| 父 root 组合日志 | `0ecda53c76902dc137bc8bd1ecd98584c7ddca1a675576372148247a3c1da6b8` |
| 父 CLI 证据清单 | `dcac381994b909f65457f71a055507f23d1d2497ce0f21000c7a220e9c95c61d` |
| 父独立 probe.py | `0cb36b8f625974c7c0a3f77168157334230bc7820ebb1bfe8a16d9c0400c9b46` |
| 父受测 Linux 二进制 | `0670c00cd7182106ebd8a4b9312f084114dd59f51099d2fbb596fd3b90ba18aa` |

T1b、T2～T6、阅读系统渲染、人工无障碍和 Mac 实机仍未验收；S0 原批准保留在原固定 checkout，修改主文档后原 receipt 拒绝新 identity 属预期，不将本次 T1a 审计改称新 complete-S0 批准。新增产品仍限单操作／简单文本，其他 DTD／内部实体暂报能力不足；该临时限制不取消 T1b 必需集。
