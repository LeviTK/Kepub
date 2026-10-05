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
