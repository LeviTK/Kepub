# Calibre CLI 与 EPUB 编辑内核研究

> 核查日期：2026-10-03。用途：优化 Kepub 架构与 CLI，而非复刻 Calibre。
>
> 结论：借鉴 Calibre 的职责分离、出版物容器、引用维护、跨文件检查点和公共操作内核；不把转换器当保存器，不让 Calibre/Qt 成为 Kepub 纯 Go CLI 的必需依赖。

## 1. 阅读范围、版本与证据强度

本次覆盖官方 CLI 索引列出的 16 个有文档命令，梳理 `calibredb` 的 23 个子命令，并登记 2 个未提供独立手册的命令。阅读编辑、转换、差异比较、编辑 API 等官方说明，深入追踪与 Kepub 有关的 CLI → 内核 → 文件修改/检查/提交链路。

这不等于逐行审计 Calibre 全仓库，也不等于对所有格式插件、设备驱动、邮件、联网抓取和第三方扩展完成运行测试。

### 1.1 固定基线

- Kepub 输入基线：`bdd02b56396482924403128037593a11260c9edf`。
- Calibre 固定源码：`a1864306758688f62f386043a2b19cf3cbf38d6a`，通过官方仓库 master ref 读取；源码链接统一固定此 SHA。
- 官方在线手册多数页面显示 9.15.0，部分页面/展开源码显示其他快照版本。在线页面版本不等于上述源码一定对应的正式发行号。
- 本次为文档与静态源码核查，没有在 Apple Silicon 安装并运行 Calibre、Amp 或 Kepub；没有性能、Qt 无窗口环境或端到端兼容性实测结论。

证据分为：公开手册承诺、固定源码实现、由两者推导的设计判断、待实测假设。Kepub 新命令都属于设计，不冒充上游命令。

## 2. CLI 全景与 Kepub 取舍

### 2.1 官方命令入口

每行链接为该命令的官方说明；“取舍”为本项目决策。

| 命令 | 职责与要点 | Kepub 取舍 |
|---|---|---|
| [calibre](https://manual.calibre-ebook.com/generated/en/calibre.html) | 主 GUI/书库入口 | 不建立第二套书库管理产品 |
| [calibre-customize](https://manual.calibre-ebook.com/generated/en/calibre-customize.html) | 插件管理和自定义 | 不把任意第三方代码执行带入 MVP |
| [calibre-debug](https://manual.calibre-ebook.com/generated/en/calibre-debug.html) | 调试、运行脚本、解包重建等 | 调试与用户命令分开；可选 helper 不是轻量核心 |
| [calibre-server](https://manual.calibre-ebook.com/generated/en/calibre-server.html) | 书库内容服务 | 只借鉴服务边界，Kepub serve 只做受限本机预览 |
| [calibre-smtp](https://manual.calibre-ebook.com/generated/en/calibre-smtp.html) | 邮件发送 | 不纳入范围 |
| [calibredb](https://manual.calibre-ebook.com/generated/en/calibredb.html) | 本地/远程书库数据操作 | 借鉴子命令与统一后端，不复制数据库结构 |
| [ebook-convert](https://manual.calibre-ebook.com/generated/en/ebook-convert.html) | 按输入/输出格式选择转换能力 | 延后、独立导入工件，不作为编辑保存 |
| [ebook-edit](https://manual.calibre-ebook.com/generated/en/ebook-edit.html) | 启动编辑 GUI，可定位书内文件 | 借鉴定位；不是 headless 编辑/校验器 |
| [ebook-meta](https://manual.calibre-ebook.com/generated/en/ebook-meta.html) | 文件级元数据读写 | 将读取与有范围的修改分开，并保留未知元数据 |
| [ebook-polish](https://manual.calibre-ebook.com/generated/en/ebook-polish.html) | 尽量小改动的整理动作 | 借鉴操作边界；每个动作须声明副作用 |
| [ebook-viewer](https://manual.calibre-ebook.com/generated/en/ebook-viewer.html) | 阅读 GUI 与起始位置选项 | 借鉴 Locator；不嵌入整个阅读器 |
| [fetch-ebook-metadata](https://manual.calibre-ebook.com/generated/en/fetch-ebook-metadata.html) | 在线元数据获取 | 默认不联网，不能自动覆盖用户书目 |
| [lrf2lrs](https://manual.calibre-ebook.com/generated/en/lrf2lrs.html) | 旧 LRF/LRS 转换 | 不纳入范围 |
| [lrfviewer](https://manual.calibre-ebook.com/generated/en/lrfviewer.html) | LRF 阅读 GUI | 不纳入范围 |
| [lrs2lrf](https://manual.calibre-ebook.com/generated/en/lrs2lrf.html) | 旧 LRS/LRF 转换 | 不纳入范围 |
| [web2disk](https://manual.calibre-ebook.com/generated/en/web2disk.html) | 网页抓取 | 不自动开启网络和远程内容导入 |

官方索引另列 `ebook-device`、`markdown-calibre` 为未文档化命令；本次只确认其登记，不声称核验了完整使用接口。源码 `entry_points` 还登记 `calibre-parallel`、`calibre-complete` 等内部入口，不应按名字直接公开为 Kepub API。[D1][C1]

### 2.2 calibredb 子命令的边界

| 分组 | 命令 |
|---|---|
| 查询 | list、search、list_categories、show_metadata、custom_columns |
| 书目/格式 | add、remove、add_format、remove_format、set_metadata、set_custom、embed_metadata |
| 书库组织 | saved_searches、add_custom_column、remove_custom_column |
| 导出/维护 | export、catalog、restore_database、check_library、backup_metadata、clone |
| 全文检索 | fts_index、fts_search |

`check_library` 检查书库数据与文件，不是验证 EPUB；书库 metadata 和书内 metadata 也是不同对象。`list --for-machine` 可以输出 JSON，但不能据此推断全部 Calibre 命令都接受同一 JSON 开关。`export` 默认可能把书库元数据写入输出文件，不能当成原书字节复制。[D2]

源码 `DBCtx` 把同一命令实现分派给本地数据库或远程服务；本地写路径检查单实例，远程操作带命令版本。Kepub 借鉴“同一用例、不同入口、单写者”，不照搬网络书库服务。[C2]

### 2.3 适配时容易误用的参数

| 错误理解 | 已核实事实 | Kepub 控制 |
|---|---|---|
| 所有 `-o` 都是输出 | `ebook-polish -o` 是 OPF 元数据输入，输出是第二位置参数 | 每命令有独立 argv builder 和测试 |
| 一份 convert help 适用所有格式 | 输入/输出插件影响参数和默认值 | 对目标格式组合探测并固定能力 |
| `ebook-edit` 是命令行改书工具 | 它是 GUI 编辑器启动入口 | 标记 requiresGUI，不能作 headless 后备 |
| explode/implode = 合规导出 | 它们提供解包/重建，不替代 EPUBCheck 和安全发布 | Kepub 自有安全归档与最终检查 |
| Calibre check = EPUBCheck | 内部检查目的和覆盖不同 | 两类来源独立，不互相覆盖 |
| CLI 启动就没有 GUI 运行时 | 某些内部检查依赖 QtWebEngine | 按功能探测，不按是否弹窗判断 |

以上对应 [D3][D4][D5][D6] 与 [C3]～[C8]。

## 3. 关键源码链路与设计后果

### 3.1 转换与整理不是同一条路径

转换 CLI 的 `create_option_parser()` 创建 `Plumber`，添加 input/output 与 pipeline 选项；`main()` 合并参数后调用 `plumber.run()`。[C3]

```text
ebook-convert
 → 输入/输出格式对应的参数
 → Plumber / 中间出版表示 / 转换步骤
 → 新输出工件
```

官方转换文章描述 input、parsed、structure、processed 调试阶段。它有助于定位哪一步引入变化，但不应把这些阶段当原 EPUB 的可逆解包。[D7]

整理侧 `polish_one()` 被 GUI/编辑器和 CLI 复用；CLI 的 `polish()` 打开 container、执行选定动作、commit 输出。该调用链没有把 EPUBCheck 设为最终门槛，因此进程完成不能推导出版物合规。[C4]

**Kepub 决策：** 默认保存不进入 convert；polish 是一组显式操作，不是“全部修复”按钮。外部转换永远产生独立工件，重新安全导入、比较、检查后再审核。调试时保留阶段清单和哈希，不默认上传整本书。

### 3.2 容器与引用操作是核心，不是 UI

Calibre 编辑 API 将 publication 内文件用 canonical name 表示，提供 URL 互转、原始数据、解析缓存和修改登记。GUI/CLI 可共用这些能力。[D8][C5]

本次追读的关键差别是：低层单文件改名与全书引用改名不是一回事。`replace.rename_files()` 先检查目标冲突等，再调用 container 改名，最后遍历引用；`LinkRebaser` 处理移动文件相对路径基准，`LinkReplacer`/`IdReplacer` 处理目标及锚点。[C6]

```text
rename request
 → 碰撞/循环/目标存在检查
 → 移动文件并重定内部相对引用
 → 更新其他文件、OPF 等入站引用
 → 重新检查
```

**Kepub 决策：** 不让 Amp 用全局字符串替换完成改名。新增 BookPath、ReferenceGraph、coverage 和受限 rename 操作；只在已支持引用语法覆盖充分时执行。移动 OPF、多 rootfile 和未知动态引用先拒绝。

### 3.3 不能直接复制 Calibre 的硬链接快照

固定版本的 `clone_dir()` 尝试硬链接，`clone_container()` 构造克隆容器。`get_file_path_for_processing()` 和提交路径在写入前检测链接数并主动断开；安全性依赖所有修改经过这些 API。[C5]

Amp 可以使用普通文件写入、脚本和 shell，不会自动调用上述断链逻辑。因此，把这种克隆目录直接交给 Amp 编辑会产生污染基线的风险；这是对代码条件的推论，并非声称 Calibre 在其自身正常 API 使用中有缺陷。

**Kepub 决策：** 初版复制候选，不使用普通硬链接；APFS 文件克隆另做测试。检查点是跨文件状态，不依赖一个文本编辑器的 Undo。

### 3.4 零条诊断可能只是后续检查没有运行

`run_checks()` 先检查解析和部分资源；如果出现高于 warning 的错误，会提前返回。后面的 CSS、链接、字体、ID、markup、OPF 检查可能没有执行。[C7]

`fix_errors()` 又有解析修复、逐问题修复和 CSS 修复等路径。修复本身是修改内容的操作，不是只读检查的必然部分。

**Kepub 决策：** 诊断之外记录 checker 状态与依赖；`blocked/unavailable/not_run` 不能冒充 `passed`。FixProposal 指向版本化操作和前置哈希，先预演，再审核；不得盲目执行“自动修复全部”。

### 3.5 calibre-debug 不会消除内部 Qt 依赖

固定版本 `check/css.py` 导入 `QApplication`、`QWebEnginePage/Profile/Script`，构造 WebEngine worker，注入 Stylelint 脚本，worker 初始化要求 Qt。[C8]

所以 `calibre-debug -e helper.py` 调内部 `run_checks()`，不应被描述为一个纯 Python、无浏览器运行时的轻量服务。是否在目标机器无可见窗口运行成功，需要额外实测；静态阅读不能回答所有部署问题。

**Kepub 决策：** Go 快检与 EPUBCheck 是正常路径；Calibre 检查是可选适配。即使 Python 调用入口可用，也要分别探测 CSS 检查、Qt/WebEngine 及失败行为。不能因适配失败自动降低正式导出的合规门槛。

### 3.6 保存和导出也可能改变元数据

`EpubContainer.commit()` 对 EPUB 3 更新 modified timestamp，并处理字体混淆的恢复等事项；未指定输出路径时还可能使用原路径。[C5]

`ebook-polish` 通常尽量少改，并不等于对每个资源字节和时间戳都保持不变。`calibredb export` 的元数据更新政策又与此不同。[D2][D3]

**Kepub 决策：** 自己定义明确的保存政策；任何外部工具结果都做全文件哈希比较。用户请求只压缩图片，却连带改了 OPF，应显示该变化，而非隐藏为“无变化”。

### 3.7 旧式解包重建不能取代 Publication Core

`tweak.py` 的旧式 ZIP exploder 会扫描寻找 OPF；rebuilder 枚举目录并写 mimetype。explode/implode 还使用一个格式标记文件来约束重建格式。[C9]

这适合解释“解包后编辑再重建”的工作方式，但不能证明 rootfile 选择、安全解包、输入碰撞、工具文件排除、引用一致性、签名或最终规范校验都已解决。

**Kepub 决策：** 仍以 container.xml 为入口、以经验证出版文件清单导出。不会将 `calibre-debug --explode-book` 直接包装成 Kepub 所有 EPUB 导入的实现。

### 3.8 文件 Undo 与全书检查点不同

`GlobalUndoHistory` 管理容器状态、比较/回退和失败后的 rewind；它是跨资源检查点，不只是某一编辑页的字符撤销。[C10]

**Kepub 决策：** deterministic、agent、external 操作统一先建立检查点。失败时恢复整组变更；初版整体接受/拒绝，避免跨 XHTML/CSS/OPF 的操作只应用一半。持久 revision 与检查点容量/清理政策由 Kepub 单独实现，不复制固定历史上限。

## 4. 官方说明文章中值得采用的交互经验

[编辑说明](https://manual.calibre-ebook.com/edit.html) 的检查点、书内链接维护、差异检查和实时预览，支持一个原则：高风险工具先保留可回退状态，再让用户看到内容变化。其容错 HTML 修复和格式化可能改变源码，不能放在 Kepub 默认打开流程。[D9]

[差异说明](https://manual.calibre-ebook.com/diff.html) 将文件增删、改名与内容差异放在同一审核上下文。Kepub 应同时显示路径变化、字节差异、正文变化提示和诊断；可读性归一化 diff 仅用于展示，不能替代真实写集合。[D10]

预览定位与样式诊断可作为后续增益：把当前章节/锚点交给 Agent，展示规则来源，而不是让模型看到任意宿主文件。制作预览正常仍不保证所有阅读器一致。

## 5. 明确的采用、改造与拒绝清单

| 对象 | 采用 | 必须改造/不采用 |
|---|---|---|
| CLI 分工 | inspect/edit/polish/convert 分离 | 不创建同等规模书库、邮件、设备命令集 |
| 编辑内核 | 统一容器和引用操作 | Go 独立实现，不把 Python/Qt 拖入默认依赖 |
| 缓存 | 有类型索引与失效 | Amp 外部写使缓存失效；禁止过期 DOM 回写 |
| 改名 | 入站/出站引用一起处理 | partial coverage 时拒绝危险操作 |
| 元数据 | 明确字段读取与修改 | 保留未知字段/refinement，不展平整份 OPF |
| 检查 | 分阶段、有可修复问题 | 加 coverage 和来源；不能替代 EPUBCheck |
| polish | 单动作与副作用说明 | 不默认改标点/子集字体/升级/删除CSS |
| 历史 | 全书检查点 | 不让 Amp 直接编辑普通硬链接克隆 |
| 数据库协调 | 一处执行者，多入口调用 | 本机 Unix socket，不照搬远程数据库协议 |
| 扩展 | 可选工具进程与能力探测 | 不默认捆绑 Calibre 或动态执行未知插件 |
| 调试 | 阶段产物、结构化报告 | 报告不自动包含/上传私有整书与凭证 |

“无损图片压缩”等上游操作描述只针对其目标数据，不扩大成“整本 EPUB 绝无其他变化”的保证。

## 6. 对 Kepub v0.2 的具体落点

主方案与 CLI 契约已经据此设置以下设计门槛：

1. M1 建立 BookPath、只读引用图和检查覆盖，不只做 unzip + WebView。
2. M2 增加 OperationRegistry、计划绑定、局部元数据与安全 rename 子集；先于复杂 Agent 自动化。
3. `plan` 不改出版内容；`apply` 只创建候选；`accept` 创建新 revision；`export` 检查最终归档。这四步不能互相偷做。
4. 所有任务类型都使用同一锁、快照、检查、diff 和状态机。
5. Calibre 每项能力独立发现，不支持的命令/运行时返回 unavailable，不悄悄改用转换器。
6. 机器输出和操作 schema 由 Kepub 自己维护；不让上游非统一 stdout 成为对外协议。
7. 未知引用、CSS 动态状态、受损 XML 都有明确阻断路径。
8. 保留原书、出版条目字节、独立导出及 MyGo 预览权限隔离；本次不扩大跨平台范围。

### 6.1 一个关键验收例

用户要求将 `EPUB/Text/ch01.xhtml` 移到 `EPUB/Chapters/ch01.xhtml`。

Kepub 必须先展示：OPF/nav/NCX 和其他章节有哪些引用要更新；该文件自身的 CSS/图片路径是否需要重定基准；有没有未支持的引用语法；目标文件是否冲突。仅移动文件后让 Amp 猜着修，不算完成该操作。

接受前检查所有预期变更，不允许在 Agent 仍运行时并发执行。失败恢复操作前候选状态，原始 EPUB 和 accepted revision 均保持原样。

## 7. Calibre 可选适配的实施界限

### 7.1 首期不需要安装 Calibre

最小运行路径仍是 Kepub 原生核心、WKWebView、可选 Amp、外部 EPUBCheck。`doctor` 发现 Calibre 后也不能自动开启其所有能力，更不能读取用户书库、启用在线 metadata 或发送邮件。

### 7.2 后续适配流程

先在 M6 选择一个操作，例如单项图片优化，使用应用提供的样本验证：真实 CLI 参数、输出路径、副作用、退出码、进程回收、修改范围、归档检查。全部通过后才设置 `available`。

`calibre-debug` helper 必须是应用受控代码，使用选定 Calibre 运行环境，不从书中加载脚本。私有 API 调用需要版本范围和兼容测试；调用失败要可恢复，不能写 accepted。

需要格式转换时，只处理隔离副本并输出新的待导入工件。Kobo KEPUB 专用转换显式选择、另有测试；不因为项目叫 Kepub 而自动触发。

### 7.3 许可与分发

本次读取的多个 Calibre 源文件明确标注 GPLv3。[C3]～[C10] 本研究只引用入口、行为与设计经验，没有复制实现代码进 Kepub，也没有为 Kepub 新增许可证。

将来直接复用代码或分发运行时/工具包时，需要单独检查相应许可与分发条件；不能把“子进程调用”当作所有许可问题都已解决的结论。

## 8. 来源索引

### 官方手册与说明

- [D1] [CLI 索引](https://manual.calibre-ebook.com/generated/en/cli-index.html)：命令覆盖及 macOS app bundle 位置。
- [D2] [calibredb](https://manual.calibre-ebook.com/generated/en/calibredb.html)：子命令、JSON、书库检查、metadata/export 行为。
- [D3] [ebook-polish](https://manual.calibre-ebook.com/generated/en/ebook-polish.html)：操作边界、参数及注意事项。
- [D4] [ebook-convert](https://manual.calibre-ebook.com/generated/en/ebook-convert.html)：格式相关参数。
- [D5] [ebook-edit](https://manual.calibre-ebook.com/generated/en/ebook-edit.html)：GUI 与书内定位。
- [D6] [calibre-debug](https://manual.calibre-ebook.com/generated/en/calibre-debug.html)：脚本、explode/implode、专用调试入口。
- [D7] [转换说明](https://manual.calibre-ebook.com/conversion.html)：转换管线与调试阶段。
- [D8] [编辑 API](https://manual.calibre-ebook.com/polish.html)：Container、路径、操作与缓存接口。
- [D9] [编辑说明](https://manual.calibre-ebook.com/edit.html)：工具、预览、检查点及修复风险。
- [D10] [差异比较](https://manual.calibre-ebook.com/diff.html)：文件和内容审核。
- [D11] [ebook-viewer](https://manual.calibre-ebook.com/generated/en/ebook-viewer.html)：阅读定位参数。
- [D12] [EPUBCheck CLI](https://www.w3.org/publishing/epubcheck/docs/cli/)：与 Calibre 检查区分的正式检查入口。

### 固定版本源码与实际阅读范围

以下均固定 Calibre commit `a1864306758688f62f386043a2b19cf3cbf38d6a`。完整阅读“关键函数/调用链”不表示完整阅读该文件所有无关内容。

| 编号 | 文件与定位 | 本次阅读范围 |
|---|---|---|
| C1 | [src/calibre/linux.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/linux.py) | entry_points，CLI/GUI 入口登记 |
| C2 | [db/cli/main.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/db/cli/main.py) | 文件全文，COMMANDS、DBCtx、本地/远程分派和锁 |
| C3 | [ebooks/conversion/cli.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/ebooks/conversion/cli.py) | CLI文件全文，动态选项构造至 Plumber 调用；未声称审计全部转换插件 |
| C4 | [ebooks/oeb/polish/main.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/ebooks/oeb/polish/main.py) | 操作注册、共享polish_one、CLI解析、polish及提交主链 |
| C5 | [ebooks/oeb/polish/container.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/ebooks/oeb/polish/container.py) | clone/path helpers、写入断链、commit、EpubContainer时间戳/字体处理；结合API文档读模型 |
| C6 | [ebooks/oeb/polish/replace.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/ebooks/oeb/polish/replace.py) | 文件全文，LinkReplacer/IdReplacer/LinkRebaser/rename_files等 |
| C7 | [ebooks/oeb/polish/check/main.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/ebooks/oeb/polish/check/main.py) | 文件全文，run_checks/fix_errors及提前返回条件 |
| C8 | [ebooks/oeb/polish/check/css.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/ebooks/oeb/polish/check/css.py) | imports、Stylelint装载、profile/Worker及Qt初始化依赖 |
| C9 | [ebooks/tweak.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/ebooks/tweak.py) | 文件全文，explode/implode、格式标记、旧式zip重建 |
| C10 | [gui2/tweak_book/undo.py](https://github.com/kovidgoyal/calibre/blob/a1864306758688f62f386043a2b19cf3cbf38d6a/src/calibre/gui2/tweak_book/undo.py) | 文件全文，跨容器历史、checkpoint、失败rewind |

这份证据足以支持当前架构修订；目标平台实际可执行性、内部 API 稳定范围、版本差异、渲染隔离与性能仍由后续 M0/M6 验证记录回答。
