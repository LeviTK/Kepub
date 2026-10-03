# Kepub

面向 Amp 的 EPUB 阅读、制作预览与编辑工作台。

**当前状态：开发设计 v0.3。仓库只有规划与研究文档，没有可运行的 GUI、CLI、安装包或已完成的实机测试。**

## 文档

- [开发方案 v0.3](docs/DEVELOPMENT_PLAN.md)：产品范围、MyGo 0.2.0 桌面层、工作区、预览隔离、操作内核、校验与实施阶段。
- [CLI 与操作契约](docs/CLI_CONTRACT.md)：拟定命令、OperationRegistry、plan/apply、机器输出、退出码与验收要求。
- [Calibre CLI 与编辑内核研究](docs/research/CALIBRE_CLI_REVIEW.md)：官方命令全景、关键源码调用链、证据及采用/不采用的设计。
- [开发方案 v0.2 历史原文](docs/history/DEVELOPMENT_PLAN_V0_2.md)：Calibre 研究后形成的上一版设计。
- [开发方案 v0.1 历史原文](docs/history/DEVELOPMENT_PLAN_V0_1.md)：最初的完整方案。

## 产品方向

首发只适配 Apple Silicon Mac，后期再考虑跨平台。桌面端以 **MyGo 0.2.0** 作为当前候选基线：主编辑窗口继续使用 Go + TypeScript + WKWebView，所有网页控制通过新的 `Window.Page()` API；MyGo 纯 Go 原生 UI 只作为独立设置、诊断、检查器等辅助窗口的候选。独立 `kepub` CLI 与 GUI 共用 Go EPUB 核心。

Amp 负责内容理解与编辑；Kepub 提供不依赖模型的确定性 EPUB 操作，并管理工作区、预览、差异、检查、审核和导出。Calibre 是参考和可选适配，不是运行核心功能的必需依赖。

```text
打开 EPUB → 阅读并定位
                ↓
     确定性操作 或 Amp 编辑
                ↓
       候选结果 ↔ 实时预览
                ↓
 差异 + 检查覆盖 → 审核接受 → 导出
```

## v0.3 的关键变化

MyGo 0.2.0 已正式发布。Kepub 因此固定新的桌面接口假设：网页能力从 `Window` 迁移到 `Page`；Preview 的加载、刷新、导航和崩溃恢复统一使用 `Window.Page()`。页面就绪不能只检查 `document.readyState`，而要绑定 workspace/task/generation/bookPath 并完成目标视图握手。

MyGo 新增的纯 Go native UI 在 macOS 使用 Metal/Core Text，但它不是 EPUB 阅读引擎，也不能从“同一应用可混用两种窗口”推导出“同一窗口可任意混排 native UI 与 WKWebView”。首发主窗口继续使用 Web UI + WKWebView；native UI 只作为独立辅助窗口候选。

CEF 没有随 MyGo 0.2.0 正式发布；公开 CEF PR 当前面向 Linux 且仍未合并，因此不进入 Apple Silicon 首发基线。

v0.2 的 Calibre 优化继续保留：转换、整理、结构编辑和只读查询分开；GUI、CLI 与 Agent 共用 OperationRegistry；原始 EPUB、accepted revision 与候选任务分离；`apply`、`accept`、`export` 保持不同语义。

## 基本约束

- 原书默认不覆盖；未修改资源尽量保持原字节，结构修改必须维护引用。
- 书籍页面不可信，不能拥有应用的文件或进程权限。
- 配置、聊天记录、检查点和工具文件不进入 EPUB。
- 正式导出以冻结归档及明确版本的 EPUBCheck 报告为依据。
- 项目名 Kepub 不表示默认输出 Kobo KEPUB，默认仍是普通 EPUB。

## 实施路线

| 阶段 | 目标 |
|---|---|
| M0 | 验证 MyGo 0.2.0 Page API、目标页面 readiness、安全预览、Amp 协议及依赖版本 |
| M1 | EPUB 核心、路径/引用模型、文件级 CLI、覆盖报告 |
| M2 | 工作区与确定性编辑，plan/apply、检查点、审核 |
| M3 | MyGo Page 生命周期、制作预览、Locator 与本机预览服务 |
| M4 | Amp 编辑、工具交接、GUI/CLI审核导出闭环 |
| M5 | Apple Silicon 打包分发和实机验收 |
| M6 | 可选 Calibre 适配及其他受控扩展 |

所有命令与接口以设计文档为准，均待实现；不能将文档示例当作当前安装使用说明。
