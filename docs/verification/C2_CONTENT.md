# C2 publication 文本读取核心验证

日期：2026-10-05。基线：父线程 local main `fdb3a5b1ec10a1b479f0e9f08f11c35ab507364f`（第二轮及 v0.7），不是仍未发布这些工作的 origin/main。输入 bundle 的 SHA-256 已核对为 `24c10db014ac2db242894a2b6509269032ed3238e99daab4a22d495e94eaf075`，导入后已删除传输文件。

本阶段只交付 `internal/publication` 独立读核心，依据 [CLI 契约 §2.1](../CLI_CONTRACT.md#21-c1c2-本批实施契约)；尚未注册 content CLI，不改变 capabilities、编辑或工作区流程。

## API 与接入边界

```go
func ReadContent(a *archive.Archive, p *Publication,
    resource bookpath.BookPath, o ContentOptions) (Content, error)

type ContentOptions struct {
    Query *string // nil = 全部；显式空字符串非法
    Limit *int    // nil = 50；显式值须 1–200
}

func (o ContentOptions) Validate() error
```

- `Validate` 可在打开工作区／获取锁之前调用，校验 query 为 1–4096 UTF-8 字节及 limit 范围；`ReadContent` 自身也校验。
- Archive 和已选 Publication 必须来自同一稳定输入；核心不打开工作区、不选择 revision、不释放调用者持有的 archive。后续 app 接入须持已有协作锁、使用 `AcceptedSnapshot`，补充真实 workspaceId、revisionId、rootfile 并关闭工作区／快照。
- `Content` 的 JSON 字段为 `bookPath`、`resourceSha256`、`locatorVersion`（1）、`matchedCount`、`returnedCount`、`truncated`、`nodes`。成功无命中时 nodes 是空数组，不是 null。
- 节点字段为 `namespace`、`localName`、可选 `id`（仅无 namespace 的 id 属性）、`locator`、`text`、`hasChildElements`。重复 id 不导致默认选择首个；定位仍依据结构位置。
- 仅按 manifest 的精确 BookPath 匹配，不 URL 解码、改大小写或去 fragment，不把 href／本机路径转成目标。所有同路径声明都必须是 `application/xhtml+xml`。

## 提取与限制

复用 Archive.Read 的 8 MiB 限制和现有 `parseXML`：UTF-8、内建 XML 实体、128 层、200000 tokens、32 MiB 聚合文本／位置索引；不复制解析器、不执行脚本、不解析外部实体或 DTD，不支持 xml:base。

要求 XHTML html 根及唯一直接 XHTML body；没有 body、间接 body、重复 body、嵌套 body 明确 `CONTENT_STRUCTURE`。这只是本提取器的结构边界，不是 EPUB 合规验证。

结果按文档先序：body 内 XHTML 叶元素（包括空／纯空白叶元素），或有非空白直接文本的非叶元素；body 容器自身、head、script/style、外来命名空间子树均排除。包含这些子树的祖先也不返回，但仍遍历其受支持子元素；排除子树内重新声明 XHTML namespace 不能重新进入结果。

text 为 XML 解码后的完整后代文本，不 trim 或 Unicode／空白归一化；XML 自身会将原始 CRLF 解码为 LF。BOM 和 CRLF 的原字节仍参与资源 hash。query 区分大小写、字面子串：混合父元素自身 text 可跨其内联子元素匹配，重叠父子分别计数；不同结果元素之间不拼接搜索。

计数包含 limit 之后的全部匹配，返回前 50（默认）或显式 1–200 个。累计**返回的** text 上限为 1 MiB，重叠父子各计一次返回字节；超限 `CONTENT_LIMIT`，不剪文本、不返回部分结果。limit 之外未返回的匹配文本不计返回预算，但仍受 XML 输入／索引上限约束。

新增错误：`INVALID_CONTENT_QUERY`／`INVALID_CONTENT_LIMIT`／`CONTENT_RESOURCE_NOT_DECLARED`（exit 2），`UNSUPPORTED_CONTENT_TYPE`（exit 3），`CONTENT_STRUCTURE`／`CONTENT_LIMIT`（exit 1）。缺失、过大资源及 XML 错误沿用既有错误。locator 是现有 `Element.Location`，以 sibling localName 计序，不是 XPath 执行器、字节偏移或编辑授权；混合内容可读不代表允许写。

## 已执行验证

开发 orb 执行未修改的 `bash .agents/setup`，新 login shell 确认 `go version go1.27.1 linux/amd64` 和 `EPUBCheck v5.3.0`。固定 checker 的完整包及 integrity list 核验通过；未启动真实 Amp、模型或 GUI。

```sh
go test ./internal/publication
go test ./...
go test -race ./...
go vet ./...
go test ./internal/publication -run '^$' -fuzz '^FuzzContentLiteralAndExclusion$' -fuzztime=10s -parallel=2
go test ./internal/publication -run '^$' -fuzz '^FuzzXML$' -fuzztime=5s -parallel=2
```

全部通过；root test/race 包括真实 checker 回归。content fuzz 为 43891 次执行、XML fuzz 为 52999 次执行，无失败。最后补充重复 id、namespace prefix／CDATA、不做 Unicode／空白归一化和 body-only 空结果测试后，publication 普通与 race 测试再次通过。

测试使用独立手写节点／locator／数量预期，不从实现推导期望：

- 同文多处及重复 id、父子重叠、混合内容顺序、空白直接文本不选非叶祖先、叶元素空文本、无命中空数组。
- BOM、CRLF、内建／数字实体、非 BMP、前缀 namespace 和 CDATA；字面／大小写／空白／Unicode 匹配及不同结果不拼接。
- 多层受污染祖先、脚本子元素、style/head、外来／空 namespace 和外来子树重入 XHTML，不泄漏祖先 Content。
- 精确路径、大小写／URL 编码／fragment／href 错误目标、未列资源、不同选定 manifest、错误 MIME、缺失资源、错误根及嵌套／重复 body。
- query 缺省／显式空、1 和 4096／4097 字节（包括多字节字符）、非法 UTF-8；limit 0／1／200／201，默认 49／50／51 和显式 199／200／201 匹配数。
- 返回 1 MiB 恰好成功，单节点／累计／重叠超限明确失败，limit 外的大节点只影响总数；8 MiB 资源及 XML 深度／token／索引超限。
- 每个 archive fixture 在 cleanup 独立比对所有资源和整本 ZIP 原字节；资源 hash 从原始 fixture 字节独立计算。
- fuzz 将合法 UTF-8 XML 文本安全转义后同时置入 script 和正常 p：始终检验完整读取 text 与原输入一致、只返回正常 p，再检验字面查询数量及 locator。不是仅随机测试输入拒绝。

## 未覆盖的后续接入

这里交付的是受限单资源提取，不是全文搜索、浏览器可见文本、EPUB 合规报告或正文编辑 API。accepted 冻结、活动候选隔离、busy、取消、JSON envelope 以及 CLI 参数／能力注册留待 C1 的准确集成基线移交后在 app/CLI 层验证；本阶段不声称这些流程已由核心测试覆盖。

## C2 app／CLI 接入追加验证

同日接入阶段从父 local main `5c25cd3fcd4cb90ead58f2430b03bb9ba9a7eb7c` 新建 `c2-content-cli` 分支；该基线含 C1、已复核的 C2 核心及查询措辞澄清。核验完整集成 bundle（398769 bytes）SHA-256 为 `f1a217aa020f6577ad14ffb9a60da2bee631456351d83affa108325b1034f170`，导入后删除传输文件。不继续在原核心分支开发，不依赖 origin/main 包含未发布工作。

### 已实现入口

```sh
kepub content --workspace DIR --resource BOOK_PATH [--query TEXT] [--limit N] --json
```

app 的 `ContentWorkspace(dir, resource string, o publication.ContentOptions) (WorkspaceContent, error)` 先调用选项 Validate，再解析用户 BookPath（路径参数错误映射 exit 2），之后依次 `workspace.Open`、`ID`、`AcceptedSnapshot`、`publication.Load`、`ReadContent`。整个查询保持现有协作锁；Load 使用 accepted revision 持久 rootfile，与文本／hash 来自同一快照。defer 先关闭 archive，再关闭 workspace，失败也释放；没有修改 workspace 实现，没有内部目录猜测或检查器调用。

`WorkspaceContent` 在读核心结果之外增加 `workspaceId`、`revisionId`、`rootfile`，JSON 扁平组合。能力注册为独立 `publication.content` v1、available／read_only、mutatesPublication=false，GUI／model／network 均不需要。输入 schema 的 required 是 workspace/resource，query 描述及 `x-maxUtf8Bytes:4096` 明确 UTF-8 字节上限（标准 maxLength 不是字节计量）；limit 默认 50、1–200。沿用 C1 capabilities → commandSchemas → help／选项检查来源，增加独立命令矩阵预期，没有第二个注册表。未注册任何正文写操作，metadata.set 原有边界不变。

query／limit 存储使用核心指针选项，未混入 output；缺省 nil 与显式空／0 分开。沿用原 parser 的下一个 argv 即值语义，不重新解释以短横线开头的文本值；真正缺失尾部值的 envelope 测试把 `--json` 放在该选项之前。`--query=TEXT`／`--limit=N` 也已验证。帮助不能让未知选项、非法 query／limit 或非法 BookPath 变成有效参数。命令不接受 BOOK、rootfile 覆盖、task、output 或 timeout。

### 执行证据

代码稳定后先本地提交并上传代码增量 bundle，供父线程并行 review／验收；之后同一代码继续完成全仓检查。仅新增／修改 cmd/kepub 和 internal/app，publication、workspace、validation、依赖和 setup 无变更。

```sh
go test ./internal/app ./cmd/kepub -run 'TestContent|TestCommandOptionMatrix|TestHelp' -count=1
go test -race ./internal/app ./cmd/kepub -run 'TestContent|TestCommandOptionMatrix|TestHelp' -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

全部通过。最后一轮定向普通 app 0.020s／CLI 0.637s；race app 1.061s／CLI 2.501s。全仓普通 CLI 68.086s、publication 0.461s、validation 116.250s、workspace 78.310s；全仓 race CLI 64.168s、app 1.077s、publication 4.264s、validation 113.124s、workspace 84.720s；vet 无诊断。真实固定 checker 环境保持就绪。报告追加不改变代码，不再重跑已完成的检查。

`TestContentBinarySmokeAndBusy` 通过现有 `workspaceBinary` 构建真实 Go 二进制并用进程执行（不是只调用 run）：

- 父测试持锁时 content 返回 exit 4／WORKSPACE_BUSY；释放后成功。
- 把 PATH 置为空工具目录、checker 路径设为不存在后仍成功：两个同文元素，limit=1，matchedCount=2／returnedCount=1／truncated=true；文本 `Binary & 😀`、首节点 locator `/html[1]/body[1]/p[1]` 正确，资源 hash 独立由原 fixture 字节计算。
- 重复 query 与非 UTF-8 query 返回 exit 2；每次 stdout 恰好一个 envelope，stderr 空，requestId 存在，没有额外输出。未运行真实 Amp、模型或 GUI。

另外 app／CLI 定向测试实测：

- 活动候选正文改为不同文本、候选 OPF 改为不可解析内容，仍返回 accepted 的真实 ID、revision、rootfile、文本和原字节 hash；候选文本无命中。
- 连续查询后逐文件比对整个 workspace（含 original、revisions、candidate、持久状态／指针／锁文件）原字节，并比对外部原 ZIP，均未变化。
- 多 rootfile workspace 使用持久选中的 alternate.opf，不默认第一本；其他 publication 独有的 manifest 资源不能查询。
- BOM／CRLF／实体／非 BMP，完整节点字段、文档顺序、两个同文位置、父子重叠定位、混合父元素 `Alpha emphasis` 字面匹配，脚本／祖先排除，大小写及不跨结果拼接，无命中成功空数组。
- 独立选项矩阵交叉检查 query／limit 仅属于 content，help 与 capability 描述一致。错误参数在不存在 workspace 前失败；app 还验证了参数错误优先于 busy，以及成功／错误后锁都可重新获取。
- 缺 workspace/resource、未知／重复／尾部缺值、显式空 query、非法 UTF-8 query/resource、4096 字节 query 接受／4097 字节拒绝、非整数／溢出／0／201 limit、BOOK／rootfile／task／output／timeout 参数拒绝，均通过单 envelope 断言。
- 201 个匹配时默认返回 50、显式返回 1／200，计数及 truncated 正确；精确路径的 URL 编码／fragment／大小写错误不被修正，未声明资源、错误 MIME、缺失资源及 1 MiB 返回文本超限保留明确核心错误。

当前可用的是受限单资源 accepted 文本读取，不是全书全文索引、浏览器可见文本、EPUB 合规验证或编辑授权。本次没有加入 content 的新取消／超时协议，也不声称验证了不合作外部写者的 OS 级隔离。上述已验证的 CLI 接入取代前一阶段“待接入”状态；正文写入、C3 及真实外部 Agent 运行仍未开放。
