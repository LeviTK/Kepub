# Orb：Java / EPUBCheck 开发依赖验证

2026-10-04，在 Linux amd64 Debian 12 orb 验证。基线为 origin/main
`77821e6b163f45f81134394873d35f3ede8774ad`。只修改 `.agents/setup`、
`.agents/resume` 和本文；没有运行模型、登录、启动服务或修改核心实现。

## 安装契约

- 保留 Go 1.27.1、Linux amd64 限制、SDK Node 26.10.0 / npm 10.9.9、
  锁文件副本 Go 缓存预热和 `npm ci --prefer-offline` 行为。
- setup 无条件准备正式验证开发依赖。PATH 中 Java 已为 17+ 时复用；否则通过
  Debian `openjdk-17-jre-headless` 安装，不在暖运行做 apt update。若安装后 PATH
  仍被旧 Java 遮蔽，明确拒绝，要求移除 PATH override。Java 不固定 Debian patch；
  本次实际包为 `17.0.20.1+1-1~deb12u1`，runtime 为 OpenJDK 17.0.20.1。
- EPUBCheck 固定 5.3.0，官方下载：
  <https://github.com/w3c/epubcheck/releases/download/v5.3.0/epubcheck-5.3.0.zip>。
  curl connect timeout 15s、总 timeout 120s。验证整个 ZIP 后解压；保留完整目录、
  `epubcheck.jar` 和全部 39 个 `lib/*.jar`，不只复制主 JAR。
- ZIP SHA-256：`6c07e68584b2e2ce2f89fe06e1246dfead3eb36b46b340e7d93524f29dcff6c5`。
  主 JAR SHA-256：`f7f96617c929371821609b88c8484d6dc9f24fe916499863c46094c5fb778a65`。
- 从已验证 ZIP 生成 `SHA256SUMS`（主 JAR + lib，LC_ALL=C 排序、相对路径，共 40 行），
  清单自身固定 SHA-256 为
  `2a5456d59b1a2aebedea1a58afc98fb36546d74be39624c3f79ae624fe34a835`。
  暖运行先验证清单自身，再验证每个 JAR，防止截断清单变成仅主 JAR 检查。
  这是包完整性清单，不是跳过包管理器的自定义缓存 marker。
- 在 HOME 同文件系统 staging 后 rename 发布到
  `$HOME/.local/lib/kepub-epubcheck-5.3.0`。修复时旧损坏目录先移入临时目录，
  再发布完整新目录；没有发布部分解压目录，但不承诺并发运行时无瞬间路径空缺或断电耐久性。
  EXIT trap 清理失败下载、staging 和替换后的旧目录。
- setup 在 `~/.bash_profile` 单次追加 `# Kepub orb EPUBCheck` 标记和绝对路径
  `KEPUB_EPUBCHECK_JAR`，每次只执行一次 checker `--version`。Java 由 PATH 解析。
- resume 不安装、不联网，只检查原工具链、Java 17+、清单及完整 JAR 集和版本；
  缺失、损坏、不合适版本时非零退出并要求 rerun setup。

## 实测

计时命令为 `/usr/bin/time -f '%e s' .agents/setup` / `.agents/resume`；
脚本的阶段计时为整数秒，以下总耗时为墙钟时间。

| 场景 | 总耗时 | 阶段 / 结果 |
|---|---:|---|
| 初次运行：已有 Go/SDK 基线，缺 Java、checker | 25.42s | Java 13s，checker 5s，Go modules 1s，Node 1s，npm 5s |
| 紧接暖运行 | 2.67s | Java/checker 0s，modules 1s，npm 2s |
| 暖运行，curl/sudo 替换成调用即失败的测试 shim | 2.87s | 成功；没有调用下载或 apt |
| lib JAR 追加损坏字节后 setup 修复 | 4.71s | 重新下载、验证完整 ZIP/清单并替换，checker 2s |
| 最终脚本，checker 目录移走后的冷安装 | 3.34s | ZIP/主 JAR/清单均 OK，checker 1s |
| 紧接最终脚本暖运行 | 2.70s | verified reuse，checker 0s；npm 2s |
| 最终 resume | 0.38s | Java 17+ / EPUBCheck 5.3.0 ready |
| 仅完整 JAR checksum | 0.13s | 40 项验证成功，故保留 resume 完整 checksum |

版本输出：

```text
go version go1.27.1 linux/amd64
v26.10.0
10.9.9
openjdk version "17.0.20.1" 2026-08-18
OpenJDK Runtime Environment (build 17.0.20.1+1-1-deb12u1-Debian)
EPUBCheck v5.3.0
```

持久环境从仓库根目录启动新的非交互 login shell，而非 source 当前 shell：

```sh
env -i HOME="$HOME" USER="$USER" PATH=/usr/bin:/bin /bin/bash -lc '
  command -v go node npm java
  go version; node --version; npm --version
  printf "%s\n" "$KEPUB_EPUBCHECK_JAR"
  java -jar "$KEPUB_EPUBCHECK_JAR" --version
'
```

输出分别为 `/home/user/.local/bin/{go,node,npm}`、`/usr/bin/java` 和
`/home/user/.local/lib/kepub-epubcheck-5.3.0/epubcheck.jar`，版本与上表一致。
反复运行后 Go 和 EPUBCheck profile marker 的 `grep -Fxc` 均为 1。

失败测试（临时 shim/备份均在仓库外，测试后清理）：

- 给一个 lib JAR 追加字节：resume 非零退出，输出
  `Managed EPUBCheck is missing or damaged; rerun .agents/setup`。
- 清单仅保留主 JAR 第一行：resume 同样拒绝，证明不是仅主 JAR hash 验证。
- PATH java shim 报 16.0.2：resume 非零退出，输出
  `PATH java must be Java 17+; rerun .agents/setup`，不安装。
- checker 目录移走，curl shim 写入错误内容并返回成功：setup 在 ZIP checksum
  `FAILED` 处退出；受管目标不存在，`.kepub-setup-*` 临时目录已清理，未发布半目录。
- 根 `go.mod`/`go.sum`、SDK `go.mod`/`package-lock.json` 前后 SHA-256 均一致；
  SDK 原本没有 `go.sum`，运行后也未创建。`bash -n`、`git diff --check` 通过；
  两个 lifecycle 脚本保持 100755。

## 限制与交付

这是现有 orb 上 Java 缺失安装、checker 空目录冷安装和暖复用的证据，
不是全新 server snapshot、空 HOME 或 Mac 的验证。apt 只改变当前一次性 orb，
未改共享系统或项目设置。没有验证未来 Q1 CLI 接入或书籍合规结果，这里只验证开发依赖。
代码先本地提交并提供单提交 bundle 给父线程；到达项目默认分支前，不对未来 fresh orbs 生效。
