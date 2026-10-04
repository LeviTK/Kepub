# M1-A 验证记录

日期：2026-10-04。执行环境：本 Medium orb，Linux amd64；不是 Apple Silicon Mac。

## 工具链与范围

- `go version`：`go version go1.27.1 linux/amd64`。
- 官方 `go1.27.1.linux-amd64.tar.gz` SHA-256：`63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445`，setup 下载时 `sha256sum -c` 输出 OK。
- 锁定 `x/text v0.29.0`、`x/sys v0.36.0`，摘要在 go.sum；headless 无桌面、模型或外部检查器依赖。
- setup 在临时 HOME/空仓库的最终版本冷启动 3.287s、热启动 0.078s；有模块热启动 0.498s，`all modules verified`，锁文件 SHA-256 校验保持不变。
- `bash -n .agents/setup .agents/resume` 成功；resume 约 0.017s；`env -i HOME="$HOME" USER=user PATH=/usr/bin:/bin bash -lc 'command -v go; command -v gofmt; go version'` 得到 `~/.local/bin` 和固定 Go 版本。

## 可复现检查

```sh
go test ./...
go test -race ./...
go vet ./...
go test ./internal/bookpath -fuzz=FuzzResolve -fuzztime=3s
go test ./internal/publication -fuzz=FuzzXML -fuzztime=3s
go test ./cmd/kepub -run TestCLIProcessSmoke -v
go list -deps ./cmd/kepub
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/kepub-darwin-arm64 ./cmd/kepub
```

单测/race/vet 通过。短时 fuzz 首轮路径约 123,608 次、XML 约 88,135 次，均 PASS；这是有界探索而非安全证明。CLI smoke 编译真实二进制并检查中文/空格/短横线路径、`--`、成功和 I/O 失败的真实退出码、单 envelope/无额外 stdout。

测试生成 EPUB2/3，明确验证：深层 OPF；命名空间/metadata refinement；percent-encoded href 原值与精确 BookPath；manifest 与 spine 非同顺序、linear=no、nav 不在 spine；多 rootfile 显式选择；所有文件原字节和空目录；原书/已有输出不覆盖；两个并发 unpack 仅一个成功；staging 失败无发布目录/临时发布残留。

对抗输入覆盖路径穿越/绝对/Windows 分隔符、重复、父目录大小写/NFC-NFD 碰撞、file-directory 冲突、symlink/FIFO、目录 payload、实际单文件/总量边界、CRC、伪造 size、ZIP 加密标志、损坏 XML/重复属性/多根、DOCTYPE/外部与自定义实体、深度上限、UTF-16 与 xml:base 拒绝。查询限制覆盖 fixed-layout/scripted、加密算法声明、签名和大小写错误引用。

## 未证明与限制

- 没有 EPUBCheck、完整 EPUB/OCF 合规检查、nav/NCX、引用图、pack、任何编辑或 rendering 验证。metadata 树是只读索引，不用于保真重序列化。
- XML 仅 UTF-8；目录输入、xml:base、远程 manifest 引用不支持。未知词汇/资源保留字节，但不解释，报告始终带 READ_ONLY_PARTIAL。
- 解包失败不发布输出；突然终止可能遗留私有 staging，未实现 journal/崩溃清理与 fsync 断电耐久性。不会自动加载书内 AGENTS/配置或脚本。
- macOS 只交叉编译：MyGo 0.2.0 API、Apple Silicon/WKWebView release 隔离、实际文件系统发布语义、性能和打包签名仍待 M0/M5 实机验证。
- 完整 registry schema/help 生成待后续；planned 项不是可执行操作，JSONL/timeout/workspace 等未来选项当前明确拒绝。
