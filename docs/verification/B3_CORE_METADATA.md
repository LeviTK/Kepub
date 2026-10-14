# B3 核心元数据：验收、合并与首个 Linux 预发行

本收据仅覆盖 [B3 #46](https://github.com/LeviTK/Kepub/issues/46) 和 [CLI §2.12](../CLI_CONTRACT.md#212-b3-核心元数据-v2-实施契约已冻结尚未实现)，不代表完整 F3/M1/T2、Mac 或发行稳定性验收。2026-10-14 用户分别授权合并、关闭、发布及规则维护。

## 固定输入与验收

- 行为准入：b3f717b904698ec18d6d7061256139026642d30a / f07760e5b1c1611ed13aa8ed98be2d9b4eff6150；冻结文本214deec，RFC资产d8892e5；不重签S0批准。
- 产品 head：[7840155](https://github.com/LeviTK/Kepub/commit/7840155c5cdcb877dad381b4837fdd7f54652b4e)，tree e0f888514e76e16ff19d86821c149a1d37f3affd。作者、独立Claude Code reviewer与父验收均覆盖该输入，仅限B3。
- 原f1dc6a3因逐操作保留完整OPF树被拒；拒绝与OOM事故证据保持不变。修复每资源共享解析、逐次核绑定，不改变权限、schema或产品预算。
- reviewer独立普通/race定向各120 PASS；CLI基础99/99、预算8/8、生命周期2/2；混合比较原oracle错误纠正后相符，原记录保留；15/15变异被杀死；独立旧红新绿。原256操作峰值约191 MB，exit0，无OOM。
- 作者固定树普通/race各1986 PASS/1私有原书SKIP/0 FAIL，vet空日志exit0；父混合两序真实checker接受/导出/重开、完整9个ZIP成员字节oracle及顺序无关性3 PASS。作者原256操作185272 KiB、完整5文件oracle PASS。
- 独立报告SHA256：9d1a6ab857ed779bd4345bbe717d26abf0c938ec4a043829c1c1f44dfea675af；作者归档286259 bytes / 9bfb0f4125f7663084d040eb1533ef942875922b84d770fc83e796ccece42533，43/43核验；父验收附录c5c0ef3a58f4ad2cbea13db44d5b6b64d652344795801859fe4180e770736454。原日志保留于获授权的Amp orbs，不公开私有书、凭据或敏感日志。

## GitHub CI 与合并适用性

[PR #76](https://github.com/LeviTK/Kepub/pull/76) 的 [CI run 38504525762](https://github.com/LeviTK/Kepub/actions/runs/38504525762) 为success，03:06:29Z–03:55:40Z。父下载artifact并解析原JSON；input中的事件ref为refs/pull/76/merge，但实际checkout明确是7840155/e0f88851，不是合成merge输入。

| GitHub ubuntu-24.04执行 | 结果 | 原日志SHA256 |
|---|---|---|
| go test -p=1 -parallel=1 -count=1 -timeout=30m -json ./... | 1986 PASS/1 SKIP/0 FAIL；12测试包PASS、2无测试包SKIP | 0d44858ed3a6a2b28d7de29e5291582fed9f87607addb5aee8c8ca85f1d6d0bf |
| 同参数 go test -race | 同计数，无DATA RACE | b50d7a7b7ba102b3638c40cdca692ed20e20a29a5d974ecaf400a31ab3edfba9 |
| go vet ./... | 无诊断、空日志 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| 普通/race CLI build | 均成功，保留build info | 见run artifact binaries.log |

Go1.27.1、Java17、EPUBCheck5.3.0；GOMAXPROCS2/GOMEMLIMIT1GiB/Java Xmx512m。唯一测试SKIP是TestT1AuthorizedOriginalLifecycle，不记为私有书PASS。

03:59:31Z合并为 [217d42b](https://github.com/LeviTK/Kepub/commit/217d42b46798f9039d8df3cd2e8bdc0fe0005b16)。完整tree仍为e0f88851，源码、依赖、测试/资产及构建配置与reviewed head相同；父核内容后复用准确head的产品/CI证据，不称main新跑。#46于04:03:06Z按原三条清单关闭；#33/#31及其他未完成模块保持开放。

## v0.1.0-alpha.1 的实际发行验证

[Linux amd64预发行](https://github.com/LeviTK/Kepub/releases/tag/v0.1.0-alpha.1) 于04:07:46Z公开；annotated tag指向217d42b，不是后续文档提交。干净隔离checkout执行 `go build -trimpath -o kepub ./cmd/kepub`，Go1.27.1；binary为静态Linux x86-64，version实际返回v0.1.0-alpha.1、revision217d42b、modified=false。发行build metadata与旧review二进制不同，不冒称二进制逐字节相同。

实际tagged binary于04:04:37Z–04:05:30Z在一次性1536MiB、swap0、timeout240s进程组中执行：原父混合控制3 PASS；原256操作Plan6.5s/Apply3.5s/diff10.4s、OPF oracle PASS，exit0；整轮wall52.55s、maxrss189944 KiB。输入记录SHA256 5ec1eed5ab03c2d4527df10eb7d3a1f5ce7d00efc41eddfd76b96f89736192ef；probe日志3976473eb40521cfc583ea4b4dfd49b0e244070176429644c40a9d620d450a9d。这不是新跑全仓套件。

- 发行binary SHA256：3695a5b8df1e2d3188e0f75078e19ff74dcad98b3ca88edf55cdc764bf5553f4。
- tar.gz：4869672 bytes / e4c588c6134aaa546e433ac88e30850386a41d243fcea672d78625eb4fc4c38a；包内7项checksum、安装说明、公开合成definition及Go/x/sys/x/text许可通知。
- 上传后重新下载公开资产，外部checksum相符，解压binary hash及tagged version相符。Java/EPUBCheck未捆绑，正式操作仍要求完整固定checker发行包。
- 仅Linux alpha预览；没有Mac/Windows包、没有部署，不把方案v0.11当软件版本。没有7840155新fuzz，Mac、断电、磁盘满、syncDir注入、私有书、reviewer race CLI、独立recovery中断注入及完整F3/M1/T2仍未测。

## 历史压力文件归属

`original-perf.*` 是HEAD为f1但已有未提交修复的working-tree运行，不是未修复f1压力测试；`fixed-perf.*`才是7840155固定提交运行。red是隔离f1加移植回归测试的断言失败，不是OOM；提交前green/focused日志不改称固定提交执行。新文档不修改原归档、历史失败或任何批准身份。
