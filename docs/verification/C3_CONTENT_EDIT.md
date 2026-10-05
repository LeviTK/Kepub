# C3：受限 XHTML 简单文本修改验收

2026-10-05，Linux x86_64 orb；Go 1.27.1、Java 17、真实固定 EPUBCheck 5.3.0。契约来源为 `docs/CLI_CONTRACT.md §2.2`，不改变正式检查语义、依赖或 setup。没有推送、运行真实 Amp／模型或 GUI，也没有创建子线程。

## 实现与交付边界

精确基线 `06474154b92c9b996ed48699370507829145c960` 从父的未发布 bundle 导入，未假定 origin/main 包含它。先独立提交保持 metadata 行为的共享 XML 重构 `1e881ba240c674473b22e4dd6aea4240287dc394`，再提交正文产品代码及测试 `1a0b261e558636de2038bcd577155bcc3a243061`。父已分别复核／集成这两批；本报告作为后续独立提交交付。

- `internal/xmltext.Parse/Replace` 提取原 metadata 的严格 RawToken 索引、命名空间验证、简单文本分类、字节范围、预算和替换后重解析；不是复制第三套 XML 解析器。metadata 继续自行决定唯一直接 OPF metadata 子元素，C2 继续使用原有读解析器和 Location。
- `publication.TextSet` 固定 params 字段顺序：bookPath、revisionId、resourceSha256、locatorVersion、locator、expectedOldValue、newValue。`Validate` 检查精确 BookPath、hash／locator／文本边界；`ApplyText(archive, selectedPublication, TextSet)` 返回替换后的字节、changed 和 error。revision 由受锁 workspace 校验，库重新读原字节核对 SHA-256、所选 manifest XHTML 和结构 locator，再推导安全写区间。`ContentText` 仅观察实际候选简单目标，用于 review，不继承计划值或旧 hash。
- Operation.Params 只分派到 metadata.Set 或 publication.TextSet 两个具体类型，不开放任意 JSON／整文件覆盖；严格解码拒绝缺失、null、重复、未知和跨操作字段。metadata 请求／plan／execution 保持版本 1；正文请求／plan／execution 为 2，operationVersion 为 1，拒绝交叉组合。workspace／revision／task／settlement 外层版本不变。
- 注册独立 available／bounded_edit 的 `content.text.set` v1，复用现有 plan → apply → diff → accept/reject → export。C2 `publication.content` 仍是 read_only。app 组合既有 workspace API，没有新增直接写文件命令或第二个注册表。
- plan、apply、execution 恢复、taskDigests、revisionSource 和 recoverSettlement 均重新推导单资源 writeSet 和完整预期树 diff。历史重算使用该任务 checkpoint／BaseRevision／选定 rootfile，而非已经推进的当前 accepted。执行写入及精确结果 hash 不再固定 OPF；no-op 的空 writeSet 不跳过目标重算。锁、独立副本、checkpoint、回滚、review_required／not_run 及真实接受／正式导出检查保留。
- review 增加可选 content：旧值、计划值和从实际候选读取的 newValue；不可读取／不支持时 newValue=null 并给出 unavailable。候选漂移仍可 diff／reject，不可沿用执行记录接受。metadata review 字段原语义不变。

## 独立预期与边界测试

`publication/set_text_test.go`、`workspace/content_edit_test.go`、`cmd/kepub/content_edit_test.go` 直接写明预期值／转义字节；不使用实现的 EscapeText 或计划新值生成实际 review 预期。

- 同文多处、重复 ID，选取第二处结构位置而非首个文本匹配；命名空间前缀、BOM、CRLF、字符引用／实体、非 BMP、引号、CR／LF／tab。结果只替换选中元素内部文本；起止标签、属性、相邻同文、注释、头部和尾部换行逐字节不变。no-op 保留原实体拼写和换行。
- 唯一直接 XHTML body；body 本身、head/script/style/foreign 祖先、命名空间重入、无命名空间元素、重复／嵌套 body、混合／子元素／注释／CDATA／PI／自闭合目标明确拒绝，no-op 同样拒绝。显式空 `<p></p>` 可替换。
- 精确 manifest 路径，不修正编码、fragment、大小写或缺失路径；错误 MIME、未声明／缺失资源、旧值不匹配均失败。非法 UTF-8／XML 字符、locatorVersion、4096／4097 字节 locator、旧／新文本 1 MiB 两侧边界覆盖；沿用 XML 8 MiB、128 深度、200000 token 和 32 MiB 文本／位置索引预算。
- 七个必需 params 分别测试缺失与 null；重复／未知／跨操作字段、多个操作、尾随 JSON、非法 UTF-8、schema／operationVersion 组合拒绝。独立顺序断言及正文 policy digest `8989bb59b95abd9a0434922f62ae7592009a139043a7e0e41ddae8be793d48fd` 固定。
- 陈旧 revision／原资源 hash 返回 INPUT_DRIFT，不建候选；定位、旧值、目标子集和参数错误在 plan 返回 INVALID_OPERATIONS。篡改调用和持久 plan 的 policy、schema、writeSet、旧值；活动执行 version／精确结果 hash；同路径错误正文和额外 CSS 修改均不获成功状态。
- 构造中断状态后的有 start／没有 start 两条恢复路径均产生 version 2 failed 记录并恢复 checkpoint；缺 start 的合成记录不降为 v1。恢复 journal 的 intent／pointer 边界及已归档任务正常重开成功；已接受 revision 入口测试 schema、policy、execution、精确内容 hash 篡改，即使同步修改外围摘要证据也拒绝。已接受历史仍使用旧 checkpoint 重推，新计划使用推进后的 revision。这些是确定性故障状态测试，不是 C3 的断电或实进程强杀证明。
- 中文／空格精确路径做 workspace 与 publication 字节验证；正式闭环使用独立合规 ASCII 资源路径 EPUB2／3。编码中文 href 的既有 checker 报告兼容问题不进入本批，没有放宽库存／正式检查。

`FuzzContentSimpleTextReplacement` 把随机输入映射为合法 XML 文本，实际执行成功替换，核对固定目标外 prefix／suffix，再以 C2 的独立解码路径检查目标文本。空串、实体特殊字符、非 BMP、CR/LF/tab 和组合字符为种子；不是主要随机生成无效 XML 测拒绝。10 秒、2 workers，34337 executions，PASS。

## 旧 v1 二进制兼容证据

在产品改动前，用精确基线构建真实二进制，创建合法 ASCII fixture，生成两个独立工作区：一个保留未用 metadata plan，一个已有待审 metadata task。新版先重开旧 task，diff matchesExecution=true；再 apply 旧未用 plan，返回 execution version 1；两个工作区各自经过新版 diff／真实 accept／正式 export，全通过。

独立保留的基线常量：

| 证据 | SHA-256 |
| --- | --- |
| 旧 metadata operations 规范 JSON | `9b544b3cf576ef62d5b671ed32118c7edc4192121e08ef86bf891f4d22a35891` |
| 既有 accepted-baseline metadata policy | `305b43d37304f964e79396386309a865e10a0f4516873c961d47015e45612935` |
| 未用旧 plan 文件 | `2309cdb8bf5a8275dadaf28f35c90721157702d4333e969801daf7b4d663da8c` |
| 待审旧 task 的 plan 文件 | `75c35330a67f3d824680392ee9e8f6c936f3317203c7c29851838fb478172134` |
| 原 EPUB 与两个 workspace 的 original/book.epub | `ba89723954d53dd68089a32d2805e2555f350583c8160d819cf4e9b437247cbe` |

旧 operations JSON 顺序／digest 和 policy 另有硬编码测试 `TestMetadataV1CanonicalBaselineEvidence`；没有从新版实现重生成其预期。导出后用独立 Python zipfile 遍历原 fixture，预期只有 OPF 中 `Title` → `Legacy Title` 的精确文本替换，全部其他资源字节和本 fixture 的完整条目集合相等；旧 plan 文件 hash 及原书 hash 不变。既有 initial-only policy 支持规则保留。

## 执行检查

定向命令：

```sh
go test ./internal/publication ./internal/workspace ./internal/metadata ./internal/app ./cmd/kepub -run 'TestContentEdit|TestTextSet|TestMetadataV1|TestPlan|TestActualWrite|TestAcceptanceJournal|TestAcceptedLifecycle|TestContentCapability|FuzzContentSimple' -count=1
go test -race ./internal/publication ./internal/workspace ./internal/metadata ./internal/app ./cmd/kepub -run 'TestContentEdit|TestTextSet|TestMetadataV1|TestPlan|TestActualWrite|TestAcceptanceJournal|TestAcceptedLifecycle|TestContentCapability|FuzzContentSimple' -count=1
go test ./internal/publication -run '^$' -fuzz FuzzContentSimpleTextReplacement -fuzztime=10s -parallel=2
```

PASS：普通 publication 0.171s／workspace 37.154s／CLI 23.078s，race publication 2.673s／workspace 46.442s／CLI 25.420s。真实 CLI 用现有 workspaceBinary 构建并 processJSON 启动独立进程；每次 stdout 恰好单 envelope，stderr 空，验证错误 exit／code，而非仅调用 run。

真实 EPUB2／3 二进制闭环覆盖 content 取同一绑定 → 参数拒绝／陈旧绑定 → plan／apply（busy 拒绝）→ accepted 隔离 → 实际候选 diff → missing checker 接受失败且不推进 → 真实接受 → 正式导出逐资源完整字节核对 → 推进基线上的 no-op → 候选真实文本漂移 → diff 显示实际值 → 接受拒绝 → reject／重复应用拒绝。最终原书不变、其他文件及 OPF 时间戳不变。

产品代码稳定后先上传单提交代码 bundle（15427 bytes，SHA256 `754e69de36b81ff8c93528faa0981bfc20cf5b72791bd2b2548549778237547a`，唯一 prerequisite 为共享重构提交），供父并行复核，然后同一代码只运行一轮全仓普通／race／vet：

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

三项全部 PASS，vet 无诊断；真实 checker 就绪，未跳过正式检查测试。全仓普通 CLI 105.074s、app 0.027s、metadata 0.306s、publication 0.613s、validation 132.609s、workspace 97.859s；race CLI 99.778s、app 1.072s、metadata 4.124s、publication 5.386s、validation 131.632s、workspace 108.490s。报告本身不改产品代码，不因此重跑测试。

父另报告已复核集成代码，并独立执行 67 次真实二进制调用全部通过，包括其旧 binary 生成的 v1 plan apply/reject 和待审 task accept/export、旧 execution 摘要不变、独立 Python 规范 JSON／policy 摘要、同文第二节点、正文正式闭环、empty/no-op、陈旧绑定及实际候选漂移。该证据属于父 orb，不代替上述本 orb 的测试；父的全仓检查单独记录。

临时基线／重构／新版二进制、兼容夹具与测试工作区、日志和传输 bundle 在本批完成后清理；保留产品测试与本报告。未改根依赖、setup、validation 或 experiments；未推送或发布。

## 父 orb 独立组合验收

父在本地 `main` 先集成共享重构，再集成相同正文代码；未改写 Medium 的产品代码。Linux amd64、Go 1.27.1、Java 17.0.20.1、完整固定 EPUBCheck 5.3.0，干净 login shell 下执行一次：

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go test ./internal/publication -run='^$' -fuzz='^FuzzContentSimpleTextReplacement$' -fuzztime=5s -parallel=2
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o "$SCRATCH/kepub-c3-darwin" ./cmd/kepub
```

普通／race 全通过，vet 无诊断；CLI 122.663s／118.179s，workspace 113.823s／125.695s，validation 153.891s／153.373s，publication 0.703s／5.932s。父 fuzz 21666 次执行 PASS；Darwin 产物经 `file` 确认为 Mach-O 64-bit arm64，没有执行，不算 Mac 验收。共享重构集成时另跑 metadata fuzz 55816 次 PASS。

父的独立 Python 驱动从不对称合成 EPUB3 开始：BOM/CRLF、两处相同解码文本但不同实体字节、混合／空／自闭合／CDATA／注释节点、CSS 和空目录。改动前的真实二进制留下未消费 v1 plan 和待审 v1 task，先核对全部工作区文件及计划 SHA-256，再用新版继续处理。v2 请求／policy 的规范 JSON 摘要由 Python 独立计算，没有调用 Go 实现生成预期值。

67 次真实二进制调用的最终输出为 `PASS: 67 independent CLI calls`：v1 计划 apply/reject 与旧 task accept/export、旧 execution 摘要保持；v2 参数拒绝／过期绑定／busy、同文第二节点精确替换、接受前 accepted 隔离、缺 checker 拒绝、正文真实接受／正式导出、no-op／清空文本／显式空元素、实际候选漂移 diff/reject 全通过。每次均检查单 JSON envelope、空 stderr 和退出码；按原 ZIP 计算期望资源，只有预期文本字节变化，OPF 时间戳与其他资源不变，原书 SHA-256 保持。另核对 capabilities 的 `content.text.set` 为 available／bounded_edit、不要求模型或 GUI。

父同步 README、主方案与 CLI 契约，记录 C3 本地交付、C4 待真实模型授权。传输 bundle 与父临时验证文件清理；源码与证据本地提交，未推送、发布或启动真实 Amp。

## 限制

本批是 revision/hash 绑定的单操作、单 XHTML 简单文本替换，不是富文本／DOM 编辑、全文搜索、资源重命名或任意文件写入。C2 可返回混合内容不意味着该父目标可编辑。正式 EPUB 合规由真实检查器决定，apply 不宣称 conformance；缺 checker 不自动接受或降级草稿。协作锁要求外部写者停止，不声称提供抵抗不合作外部进程的 OS 级隔离。未执行真实模型、Mac 运行或 GUI 验收。
