# M2-A 工作区基础验证

日期：2026-10-04（Linux orb）；实现基线为 M1-A 与开发方案 v0.4。修改仅在 `internal/workspace/` 和本文，没有修改 archive/publication、根依赖锁或 CLI。

## 已交付的库边界

`workspace.Create(dir, source, Options{Rootfile: ...})` 创建并持有单写者工作区；`Open` 重开并取得相同所有者锁；`Close` 释放锁。`State` 返回导入来源与初始清单的独立副本。

`NewCandidate` 创建唯一活动候选并返回绝对出版目录；`Candidate` 找回已有目录；`Checkpoint` 返回独立快照的 ID 与清单；`Checkpoints` 枚举并验证已存快照；`Restore(id)` 恢复指定快照。`HashTree` 独立计算目录清单与内容哈希。

这不是 workspace CLI、socket 或任务服务，没有 plan/apply、accept/reject、export、Agent 启动或检查器。`initial` 仅是导入快照名称，不是审核通过；状态和快照中没有伪造的 `passed`、`accepted` 或 `valid` 字段。损坏 XHTML 等普通文件可以被检查点保留供调查。

```text
<workspace>/
  owner.lock                        # 保留 inode，Close 不删除
  state.json                        # version、原书摘要、rootfile、初始树、只读原因
  original/book.epub                # 输入容器字节的独立副本
  revisions/initial/pub/            # M1-A 安全解包后的所有条目
  tasks/active/
    task.json                       # 仅版本和 baseRevision
    work/pub/                       # 唯一可写出版候选
    checkpoints/<random-id>/
      checkpoint.json
      pub/
  staging/                          # 库专用、可清理；不是用户存储目录
  journal/restore.json               # 仅恢复进行中存在
```

不预造 plans、preview、reports、通用任务状态机或数据库；本轮没有候选销毁/接受流程，因此一个工作区最多创建一次 `active`，再次创建返回存在冲突。

## 数据保护与失败处理

- **源书与初始版本**：先将普通输入文件复制到私有 staging，再调用冻结的 `archive.Open`、`publication.Load` 和 `Archive.Unpack`。原书按容器字节保留，未改出版资源按展开条目字节保留，包含空目录和 OPF 未登记文件。多 rootfile 必须显式选择并记录。源路径随后删除不影响重新打开。
- **完整发布**：目标必须不存在，父目录必须已存在。Linux 用 `renameat2(RENAME_NOREPLACE)`，Darwin 实现用 `renameatx_np(RENAME_EXCL)`；拒绝已有普通文件、空/非空目录和符号链接，也拒绝预检后才出现的空目录。创建期间已有工作区不会被覆盖。
- **独立复制**：original、revision、candidate、checkpoint 之间不创建普通硬链接或 APFS 克隆。候选通过普通截断、原位写入、改名、增加或删除文件，不会改变已有基线与检查点；恢复后的候选仍是新复制。
- **清单哈希 v1**：精确 POSIX 路径按字符串字节序排序；每条记录包括 `path/type/size`，普通文件另含内容 SHA-256，目录 size 为 0。以固定 Go JSON 字段顺序编码清单，哈希输入为 `kepub-tree-v1\n` 加编码字节。根自身不作为条目；包含空目录。mtime、权限、遍历顺序不参与；相同内容改路径、大小写、类型或大小均改变清单。不是只串接文件内容或只哈希 OPF manifest。
- **路径和文件类型**：根及其祖先必须是真实目录，调用方需传已解析系统别名的路径（例如 macOS 的 `/private/tmp` 而非符号链接 `/tmp`）。后续读写使用 `os.Root` 限制在根内；出版树拒绝任何符号链接、普通文件硬链接、FIFO/特殊文件、不规范 BookPath 和大小写/Unicode 碰撞。快照 ID 必须是 32 位小写十六进制，不允许路径输入。源 EPUB 本身的符号链接/硬链接也拒绝。
- **所有者锁**：非阻塞 advisory `flock` 绑定打开的 `owner.lock` 文件描述符；第二个句柄/进程收到 `ErrBusy`。不依赖 PID 文件，不 unlink/recreate 锁 inode；正常退出和 SIGKILL 均由内核释放。句柄方法通过 mutex 串行，关闭后拒绝写操作。
- **候选恢复**：先复制并核对完整 checkpoint，原子发布 restore journal，再将旧 pub 移至内部备份，将新 pub 移入固定位置。恢复只替换 `work/pub`；其中在所选检查点之后增加的文件会移除，这是显式恢复的语义。`work/` 旁边的笔记和 workspace 根的其他用户文件不动。该流程可恢复，但不是允许并发读者观察的单次原子目录交换。
- **I/O 失败**：journal 前失败清理本操作 staging，候选不变；journal 后失败保留旧/新树与 journal，句柄返回错误并进入 `ErrRecovery`，必须关闭后重开。`Open` 验证原书、revision、所有 checkpoint，完成已有合法 journal 的恢复，再验证候选文件系统并清理库专用 staging。损坏清单、危险路径或坏 journal 拒绝打开且不删除恢复证据。不把这些错误转成新工作区或绿色状态。
- **用户文件**：非 manifest 资源与候选中新建普通文件均进入树和 checkpoint，不按扩展名丢弃。未知工作区根文件不自动删除；`staging/` 是明确的内部保留区，其遗留内容在成功重开时清理，不能放用户文件。
- **已知不支持输入**：保留原书与 revision，但 M1-A 除 `READ_ONLY_PARTIAL` 以外的 limitation 均限制候选创建。已知书内配置组件 `AGENTS.md`、`CLAUDE.md`、`.amp`、`.claude`、`.mcp.json`、`mcp.json`、`.vscode`、`.git`（不区分大小写，任意深度）同样只读；不复制到可运行的 Agent 工作目录。这不是证明所有未来 Agent 配置发现已隔离，也没有实际启动 Agent。

所有库创建的目录为 0700、普通文件为 0600（受 umask 收紧）。所谓不可变，是库不修改 original/revision/checkpoint 并在重开/使用前核对清单；同一用户仍可直接改它们。锁、cwd、权限和复制不是同用户恶意进程的 OS 沙箱。调用方必须先停止源书/候选的所有外部写入者；检查点的复制前后哈希能发现一般竞争，但不能证明并发目录扫描是原子快照。

## 已执行验证

环境：Go 1.27.1，Linux x86_64，非 root 用户，临时生成的小型 EPUB；无真实书籍、付费模型或外部检查器。

```text
go test -count=1 ./...
  所有包通过（workspace 约 0.43s）
go test -race -count=1 ./...
  所有包通过（workspace 约 4.91s），无 race 报告
go vet ./...
  退出 0，无诊断
GOOS=darwin GOARCH=arm64 go test -c -o <temporary>/workspace.test ./internal/workspace
  成功生成 Mach-O 64-bit arm64 executable；没有执行它
```

主要回归测试：

| 测试 | 验证区别 |
|---|---|
| `TestIndependentCopiesRestoreAndReopen` | 普通直接写入、删除、改名、新用户文件；原书与基线字节不变；两个检查点恢复；恢复后的写入不改检查点；移除输入后重开 |
| `TestTreeHashContract` | 独立字面量确定哈希编码，已知 SHA-256 样本；mtime 不影响，同大小内容改变影响 |
| `TestTreeOrderPathTypeAndSize` | 反向创建顺序一致；同内容不同精确路径不同；空文件和空目录不同；实际字节大小 |
| `TestTreeRejectsUnsafeEntries` | 文件/目录/根内符号链接、硬链接、大小写与 Unicode 碰撞 |
| `TestExistingDestinationAndConcurrentCreation` / `TestCrossProcessCreateRace` | 已有用户对象不变；8 goroutine 与4独立进程分别竞争创建，仅一者成功、失败者无残留 |
| `TestPublishDoesNotReplaceLateEmptyDestination` | 预检后才出现的空目录仍不替换，stage 保持完整 |
| `TestCrossProcessLockAndExitRecovery` | 同进程和跨进程冲突；子进程不调用 Close 的正常退出与 SIGKILL 后可重开；锁 inode 不变 |
| `TestCreateFailuresLeaveNoPartialDestination` | ZIP/OPF/路径/多 rootfile/缺源/源 symlink 失败，没有目标或 staging 残留 |
| `TestSpecialFilesAndDiskFailure` / `TestLateIOFailuresAndRecovery` | FIFO 拒绝；真实权限拒绝发生在 stage 创建、checkpoint 发布、journal 后旧 pub 移动处；失败后磁盘状态、句柄限制与重开恢复 |
| `TestRestoreInterruptionRecovery` | journal 前、journal 后、移开旧树、发布新树、删除备份、完成这六个真实磁盘状态的重开结果；未使用 mock rename |
| `TestFailurePreservesCandidateCheckpointAndEvidence` | 损坏 checkpoint/恢复 stage/journal、候选和元数据 symlink；失败保留用户内容和恢复证据 |
| `TestCheckpointIDsNeverTraverse` / `TestTamperingAndStateCopies` | ID 路径穿越拒绝；基线改动拒绝；失败 Open 释放锁；State 不泄漏内部可变 slice |
| `TestReadOnlyInputsPreserved` | 书内指令/MCP/加密声明/签名资源保留但不产生可写候选 |

## 尚未证明与后续约束

- **macOS 未实测**。Darwin 平台文件仅交叉编译；APFS 大小写/Unicode 行为、`flock`、目录 fsync 与 rename 语义仍需 Apple Silicon 实机验证。Linux 碰撞 fixture 不能直接证明默认大小写不敏感文件系统的行为。
- **断电耐久性未证明**。实现检查文件/目录 Sync 并按 journal 顺序发布；未证明突然断电、磁盘控制器缓存、NFS、文件系统损坏或 macOS `F_FULLFSYNC`。中断测试重建操作边界，不冒充真实断电实验。
- **ENOSPC 未注入**；实测故障是权限拒绝及实际损坏文件，不能将其写成所有磁盘故障均已覆盖。若失败清理本身失败，会返回合并错误并保留残留。
- 创建阶段若进程在最终发布前被强杀，目标仍不存在，但父目录中可能保留私有 `.kepub-create-*`。本轮不会扫描删除用户目录中这些兄弟项；需人工核实后删除，不能把名字前缀当作删除授权。已发布工作区内的候选/检查点 staging 则由 `Open` 清理。
- 候选可能是无效出版物；本库不运行引用检查/EPUBCheck，不保证审核可接受。Open 遇到候选危险文件系统条目会拒绝，保留磁盘供人工检查，不静默移除链接。没有容量淘汰策略、空间配额或性能验收。
- 原书签名/加密、配置隔离和全部出版结构的生产编辑政策仍由后续应用层负责；本轮保守只读策略不是完整安全认证。

## 独立后续：SDK 实验的 orb setup 验证

本小节对应四条工作线集成后的限定 setup 增量，不改变 M2-A 工作区库或 SDK 实验实现。只修改 `.agents/setup`、`.agents/resume` 和本小节；两脚本保持 100755。没有修改根或实验依赖锁、README、方案、实验代码，也没有推送此增量。

### 安装策略

- Go 仍固定 1.27.1；分别复制根模块及存在的 `experiments/amp-sdk/go.mod/go.sum` 到临时目录，再以 `GOWORK=off GOTOOLCHAIN=local` 下载并验证缓存，不改原锁。本轮 SDK Go 模块仅标准库，因此会输出 `no module dependencies to download`，不是漏跑。
- 仅当 `experiments/amp-sdk/package-lock.json` 存在才检查/准备 Node **26.10.0**、npm **10.9.9**。base 精确匹配时复用；否则下载到 `$HOME/.local/lib/kepub-node-26.10.0-npm-10.9.9`，先校验并验证版本，再从同文件系统 staging 发布，不覆盖系统 Node/npm。
- Node Linux x64 tar.xz 使用官方 [SHASUMS256.txt](https://nodejs.org/dist/v26.10.0/SHASUMS256.txt) 的固定 SHA-256：`ca70e9e349de048b9522abb3adc05b3bd6f43c5ffd3ec57916c7da292f59f022`。npm tarball 使用官方 [registry npm/10.9.9](https://registry.npmjs.org/npm/10.9.9) 的固定 SHA-512 integrity，脚本保存其十六进制值。不是下载后临时相信一个浮动版本，也没有声称执行 GPG 签名验证。
- `node/npm/npx` 与已有 `go/gofmt` 共用 `$HOME/.local/bin` 和既有 login PATH hook；重建链接前解析真实目标，避免热运行创建自引用链接。profile marker 没有重复追加。
- 执行 `npm ci --prefix experiments/amp-sdk --include=dev --include=optional --ignore-scripts=false --prefer-offline --no-audit --no-fund --update-notifier=false`，保留开发依赖和固定 CLI 的平台安装脚本。先实测 npm 单独冷/热安装 **2.99/1.81 秒**，热路径足够便宜，因此**没有自造依赖指纹/缓存标记**。setup 重跑使用 npm 的锁与完整性校验缓存；精确 orb 快照直接保留安装好的依赖。
- 没有 SDK 锁时不探测或安装 Node/npm，核心仍不需要它们。resume 只修复既有 Go 链接并快速检查已装 Node/npm/SDK；缺失即明确报错要求重跑 setup，绝不下载或安装依赖。setup/resume 均不认证、不启动服务、不调用真实 Amp 命令或模型任务。

### 实测结果

Linux x86_64、非 root、Go 1.27.1。各冷运行使用独立空 HOME、空 Go/npm 缓存及复制的仓库锁；强制不匹配路径由 PATH 前置的版本 0.0.0 测试替身触发，随后使用真实官方归档完成安装。时间来自 `/usr/bin/time -f '%e'`，不是性能 SLA。

| 场景 | setup 冷 | setup 热 | resume |
|---|---:|---:|---:|
| base Node/npm 精确匹配；空 HOME 下载 Go 与全部依赖 | 5.85s | 2.20s | 0.08s |
| 版本不匹配；额外下载并校验局部 Node/npm | 8.96s | 2.24s | 0.10s |
| 无 SDK 锁、仅核心；禁止 Node/npm 探测的替身未被触发 | 3.24s | 0.28s | <0.01s |
| 实际仓库/既有 orb；Go 已装 | 2.96s | 2.25s | 0.10s |

所有热运行用 `npm_config_offline=true GOPROXY=off`（核心路径仅需后者）完成，锁文件 SHA-256 前后完全一致；系统 `/usr/local/bin/node` 与 npm CLI 文件 SHA-256 也完全一致。

额外验证：

- `bash -n .agents/setup .agents/resume` 与 `git diff --check` 通过。
- 使用最小环境 `env -i HOME=<isolated-home> PATH=/usr/bin:/bin /bin/bash -lc ...`，分别验证 base 复用、局部安装和核心路径；`command -v` 命中用户 bin，实际版本为 Go 1.27.1、Node 26.10.0、npm/npx 10.9.9。profile marker 恰好一次。没有用当前 shell `source` 假装持久环境生效。
- 独立安装中 TypeScript 实际输出 `Version 7.0.2`，真实 SDK import 成功，固定本地 CLI 二进制已存在且可执行；检查没有运行该 CLI。
- 注入损坏的 Node 下载内容：SHA-256 拒绝，setup 非零退出，局部 Node 目标没有发布，临时 staging 清理完成。
- 临时移走 `node_modules`：resume 非零退出并提示重跑 setup，没有重新创建依赖目录。
- 实际仓库的干净 login shell 中，`go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 和 `npm --prefix experiments/amp-sdk run check`（TypeScript noEmit + 独立 Go vet）全部通过。此 setup 增量没有重做真实 Amp 联调或运行 SDK 的模型任务。

这是当前 Linux orb 的安装/环境证据，不是新项目快照服务端生成的实测，也不是 macOS 安装证据。增量提交需经集成并到达项目默认分支后，才会成为未来 orbs 的新 setup；本工作线未获推送授权，未推送。
