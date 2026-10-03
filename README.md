# Kepub

面向 Amp 的 EPUB 阅读、制作预览与编辑工作台。

**当前状态：开发方案阶段。仓库目前仅包含规划文档，尚未提供可运行的 GUI、CLI 或安装包。**

## 开发方案

完整方案见 **[Kepub 开发方案](docs/DEVELOPMENT_PLAN.md)**，包含架构、CLI 契约、EPUB 校验边界、安全要求、任务快照、实施里程碑和验收矩阵。

## 产品方向

第一阶段只适配 Apple Silicon Mac，采用 MyGo + Go + TypeScript + WKWebView。Amp 负责编辑出版内容；Kepub 负责工作区、阅读定位、实时预览、变更审核、规范校验和安全导出。后期再考虑跨平台。

```text
打开 EPUB → 建立工作区 → 阅读并定位章节
                         ↓
                  Amp 编辑任务副本
                         ↕
                      实时预览
                         ↓
             查看差异 → 校验 → 接受或撤销
                         ↓
                   导出独立 EPUB
```

GUI 和独立 `kepub` CLI 共用 Go EPUB 核心；无界面命令不依赖桌面窗口。MVP 首选 Amp CLI 的结构化 JSON 流，不额外强制安装 Python SDK 或 Node SDK。

## 基本约束

- 原始 EPUB、已接受版本和 Agent 候选结果分离，默认不覆盖原书。
- 书籍页面是不可信内容，不能拥有应用的文件操作或进程启动权限。
- 应用配置、聊天记录和工具文件不进入导出的 EPUB。
- 制作预览、EPUBCheck 通过和完整阅读系统一致性分别说明，不混为一谈。

## 实施顺序

| 阶段 | 交付目标 |
|---|---|
| M0 | 验证 MyGo 预览隔离、Amp 协议和依赖版本 |
| M1 | EPUB 核心、文件级 CLI 和校验 |
| M2 | 工作区、快照、任务和共享会话 |
| M3 | MyGo 阅读预览与 CLI 预览服务 |
| M4 | Amp 编辑、审核和导出闭环 |
| M5 | Apple Silicon 打包、分发与实机验收 |

具体任务及每阶段通过条件以 [开发方案](docs/DEVELOPMENT_PLAN.md) 为准。文档中的命令和接口均为拟实现设计，不是当前可执行功能。
