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
- T1b 内部子集、实体、默认／固定属性及媒体绑定外部标识符仍待下一批；T1a 不等于完整 T1。T2、UI／阅读系统／Mac 实机测试未执行。没有 Droid、合成批准、push、发布或自动接受。
