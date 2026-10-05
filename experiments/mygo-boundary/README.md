# MyGo v0.2.0 集成边界实验

只验证真实 MyGo core + 上游 fake backend；另提供真实 WebView 的追加测试供原生环境执行。不实现 Kepub GUI，不改根模块或上游实现。

固定上游 [v0.2.0 / d51d2e2](https://github.com/egoist/mygo/commit/d51d2e28dff5b351bc67cf2280b5eef00f1267e8)，工具链 `go1.27.1`。已阅读该版本 `AGENTS.md` 与 `docs/architecture.md`。runner 获取 tag 后校验完整 commit，向临时 checkout 添加测试文件，复用上游 `TestMain`、`testWindow/readyWindow/page/call` 或 `newWindow/waitFor`。测试经真实 `WindowHandler.Message`、`NavigationCommitted` 和全局 `BindAs` 分发；不复制信任策略、路由或 secret 校验。

## 重跑

在 Kepub 仓库根目录执行：

```sh
bash experiments/mygo-boundary/run.sh core
bash experiments/mygo-boundary/run.sh native-compile
# 仅在已具备 WebKitGTK、Xvfb、D-Bus 的 Linux 原生测试环境执行：
dbus-run-session -- xvfb-run -a bash experiments/mygo-boundary/run.sh native
```

需要 Git、可获取 Go1.27.1 的 Go、网络下载上游及 Go 依赖；race 还需要宿主 C 编译器（Go race instrumentation 要求 `CGO_ENABLED=1`，MyGo 源码自身不使用 cgo）。原生模式不安装依赖、不启动 Xvfb、不禁用 WebKit 沙箱。

覆盖源码以 `.go.txt` 保存，避免根模块 `go test ./...` 扫入两个上游测试包；runner 将它们复制为上游模块中的 `.go` 文件。所有 checkout、临时测试和 native 编译产物在退出时删除；Go 工具链/模块缓存保留在正常的用户缓存中。每次 `-count=1` 重新执行，不使用测试结果缓存。错误会直接退出，不强行跳过或硬编码通过。

`core` 运行四轮：

```sh
CGO_ENABLED=0 go test -count=1 -v .
CGO_ENABLED=1 go test -race -count=1 -v .
CGO_ENABLED=0 go test -ldflags '-X github.com/egoist/mygo.production=1' -run '^TestKepub' -count=1 -v .
CGO_ENABLED=1 go test -race -ldflags '-X github.com/egoist/mygo.production=1' -run '^TestKepub' -count=1 -v .
```

前两轮保留开发模式运行全部上游 core；后两轮只运行追加边界测试，避免 `TestWindowDefaults`、开发 localhost 等上游预期与 production 冲突。runner 清除继承的 MyGo 开发 URL、环境模式及 helper-process 开关。`native` 同样链接 `production=1`，设置 `MYGO_E2E=1`，只选择追加 `TestKepub*`；`native-compile` 只执行 `go test -c`，不会把 TestMain 的 E2E 跳过当成通过。

## 已执行证据（2026-10-05，Linux amd64 orb）

最终文件版本重跑 `run.sh core`，退出码 0；四轮分别输出：

```text
Development normal: ok github.com/egoist/mygo 1.192s
Development race:   ok github.com/egoist/mygo 7.049s
Production normal:  ok github.com/egoist/mygo 0.310s
Production race:    ok github.com/egoist/mygo 1.366s
PASS: real MyGo core + upstream fake backend only; NOT Kepub GUI acceptance.
```

production 两轮日志均为 `CORE CONFIG: IsDev=false`，开发两轮为 `true`；无 race 报告。下表所有 core 用例在四轮均通过，**通过表示观察与断言吻合，不表示危险配置安全**。

| 用例 | 实际观察 / 断言结果 |
|---|---|
| `TrustMatrix` 的 empty / explicit × app | `mygo://localhost` 成功调用，返回实际 caller ID |
| 外部 HTTPS、origin 后缀伪造、未注册 scheme | 拒绝且 probe 计数不增加 |
| 列出的 HTTPS origin | 空列表拒绝；显式列表允许 |
| `file` / `about` / 注册 `kepub-book` | 空列表和显式列表都成功调用：`DANGER REPRODUCED` |
| localhost 控制 | dev 允许；production 拒绝，不能拿开发例外解释上述内建信任 |
| `WindowAuthorizationAndNavigation` 的第二窗口 | book 窗口访问同一全局服务，caller ID 是 book 窗口 ID：危险反例 |
| `UIOnly` 服务授权 | UI 窗口允许、第二窗口明确拒绝，拒绝不增加 probe 计数 |
| UI 窗口导航到注册 book scheme | 旧的实际 bound-call context 取消，新 context 存活且 caller 不变；ID-only 服务仍放行：危险反例 |
| UI 窗口导航到外部 HTTPS | trust 更新为不可信，调用被 core 拒绝 |
| `Secrets` 的 missing / wrong / cross-window | 都被丢弃且 probe 不执行；各窗正确 secret 的控制调用成功 |

`run.sh native-compile` 退出码 0，输出 `PASS: native coverage compiles; no native execution or frame isolation claim.`；`bash -n` 与 `gofmt -d` 检查通过。根模块 `go list -mod=readonly ./...` 通过，未扫描本实验的 `.go.txt` 覆盖文件。本线程**未执行新增 native 用例**；下面真实 Linux 结果由父线程独立运行并回报，未执行 macOS/WKWebView。

## 新增 native 覆盖与父线程 Linux 观测

父线程 [MyGo 集成验证](https://ampcode.com/threads/T-01a108ce-a094-768d-9deb-73b3b52b86bc) 下载并阅读本目录最终测试源码和 runner 后，在 Linux WebKitGTK 4.1 `2.50.6-1~deb12u2` / Go1.27.1 / Xvfb 环境执行 `bash run.sh native`，链接 production 模式。2026-10-05 回报全部 PASS：`ok github.com/egoist/mygo/internal/e2e 0.947s`。未修改上游实现，未禁用 WebKit 沙箱。下列断言均在该 Linux 环境实际满足；**包括危险配置成功调用的复现，不代表这些配置安全**。

`TestKepubNativeParentBridgeBoundary` 使用相同 srcdoc 子页面和只计数/返回 caller ID 的唯一服务：

- 无 sandbox：子 frame `parent.mygo.call` 成功、probe 增加、返回宿主 ID，标记 **DANGER REPRODUCED**。
- `sandbox="allow-scripts"`，不带 `allow-same-origin`：宿主必须收到由该 frame `postMessage` 发出的 `started` 和 `denied/SecurityError`；同时计数不增加。不是仅断言“没收到调用”。
- `sandbox=""`：srcdoc 设置好再连接 iframe，宿主 `load` 与文档 `complete` 证明加载结束，frame 脚本无报告、probe 不增加。相同脚本在前两个用例有执行证据；各例再以宿主调用作 runtime/binding 正控制。

`TestKepubNativeCSPDoesNotRemoveBridge`：严格 `script-src 'none'` 下页面 inline sentinel 和 probe 都不执行；加载结束后 `Page.Eval` 仍看见 `mygo.call` 且能成功调用 probe。它验证 CSP 抑制页面脚本，**不是**卸载 bridge。

## 结论与限制

已证伪“空 TrustedOrigins 拒绝全部”“第二窗口天然无权限”“注册 book scheme 天然隔离”。`TrustedOrigins` 是增加许可而非削减 `file/about/注册协议` 的内建信任。桥接不能被当作可信 UI 与书籍内容之间的自动权限边界。

`CallerWindow` 服务授权可限制第二窗口，但窗口 ID 不等于文档身份：同一可信 UI 窗口被不可信 book 文档替换时 ID 不变，注册 book 协议仍获 core 信任。必须另有导航限制/文档授权与内容隔离；context 取消只是信号，不回滚已经发生的副作用，服务仍须主动响应取消。

网页窗口的 bridge 是顶层文档注入；macOS 默认 `ForMainFrameOnly=true` 且原生 handler 拒绝非 main-frame 消息。**同源子 frame 借用 parent 的顶层 bridge** 是不同路径，不能由“子 frame 直接伪造无 secret 消息被拒”推出它被阻止。上游 `TestIframeCannotCall` 只测试后者（data iframe 向 handler 发送无 secret 消息）。这里 fake 测试不证明 frame 隔离、secret 保密或真实浏览器执行；新增 native 用例的已执行结果仅来自父线程上述 Linux 环境，不能声称 macOS 通过。所有结果也不计为 Kepub GUI 验收。
