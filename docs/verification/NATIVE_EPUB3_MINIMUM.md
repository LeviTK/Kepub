# 原生 EPUB3 首增量：验证与交接收据

## 任务、范围与输入

- 用户授权执行计划，先完成一个最小功能闭环；同一问题两次针对性修复失败才转 Claude Code Orb，仍失败才转 high。本轮两个已证契约缺陷均第一次修复通过，没有触发升级。
- 目标：`workspace create --definition` 原生生成正式校验通过的多章 EPUB3 工作区，复用已有读取、正文/标题编辑、plan/apply、diff、accept/reject、export 和重开。
- 非目标：完整 M1/F3、封面/插图、两层目录、创建后新增资源/章节重排、GUI、EPUB2 转换、Mac 实机、断电和发行。没有关闭任何 Issue。
- 风险：共享来源/初始化/恢复，高风险；新接口冻结于 [CLI §2.0.1](../CLI_CONTRACT.md#201-原生-epub3-最小新建-v12026-10-13-冻结)，旧 state v1 与编辑 schema 1–7 不扩权。
- 作者/父执行者：[本线程](https://ampcode.com/threads/T-01a1356d-a27e-72da-a436-9a759b65f3e4)，Linux x64 Orb；独立 reviewer：[独立 Orb](https://ampcode.com/threads/T-01a1357d-5fba-714c-a537-1904bb1b08e9)，独立 checkout，不将作者自审称独立批准。
- base、本地 main 与 origin/main：`692bacb1160e0113362495f946c52c901296cc16`，tree `59f96f878c6e96949312add2e9b324385fb2c52c`。编码分支 `feat/native-epub3-minimum`，未推送。
- 首 fixed：`1587f36f5b13e6d3815f3d20d638e4562b1bde2d`，tree `06ba46bc0b3fb143ade0abee34c06bd863c04d6d`。
- 修复 fixed：`7817541c454a2ef1b2ec7ff148d7a10ed774fdcf`，tree `1a8e063d1c4255be4c471e35b797190022c43a3a`；提交时 tracked clean，独立验证树 `/tmp/kepub-native-fixed`。本收据、README 链接及计划状态是随后单独的文档增量，不是该产品 fixed 的组成部分。
- 测试加强 fixed：`902c321d46fc2816329dd04b8fccc028332890f2`，tree `b5e13964ce07aeeb79580419a87d04451dcee515`；只有 CLI 生命周期测试与 workspace AGENTS/NOTES 改变，产品完全同修复 fixed。
- 最终测试 fixed：`83e1be6384ae893618e605e27b5c3180070b1856`，tree `c8aa5e80fc21589f68b62417e542453f8ba94498`；仅在该生命周期夹具中加入 TAB/CR/LF/双引号/单引号和独立期望，区分 XML 与 HTML escaping；产品仍完全同修复 fixed。
- 原 bundle：15,676 bytes，SHA256 `2cd98ed10b0fb59c0624aa60a2bff046a078290286d19cc760043874efd05a4a`；修复 bundle：19,167 bytes，SHA256 `8da20479704859e0cdad3ff5ddf7431a68b6860547fd1acfbd8f5f5b0c39fa20`；测试加强 bundle：21,347 bytes，SHA256 `f8a87882e964216370893627966bfa6a42a2819356a897cbba972dbe7bf7e676`；prerequisite 均为上述 base。三者均已上传 reviewer 工作区并独立 verify/核收通过。
- 最终测试 bundle：22,143 bytes，SHA256 `8d4d8f31cb9d0216093d7e8582d17cfa2c4d9c1591433dca308970895dc6bef0`，同 base prerequisite，已上传，独立核收与结论见最终 reviewer 报告。
- 当前授权包含编码、本地提交与验证；不包含 push、PR、merge main、关闭 Issue、release、部署或手动 CI。

## 环境与执行归属

Go 1.27.1 linux/amd64、OpenJDK 17.0.20.1、固定 EPUBCheck 5.3.0 与受管依赖；JAR SHA256 `f7f96617c929371821609b88c8484d6dc9f24fe916499863c46094c5fb778a65`。作者普通/race 长套件使用 `GOMAXPROCS=2 GOMEMLIMIT=1GiB`、`-p=1 -parallel=1 -count=1 -timeout=30m -json`，自然运行不取消重启。reviewer 原固定全仓用 `go test [-race] -count=1 -timeout=30m ./...`，没有 `-json/-v`，只能核包数，不能借作者数字冒称独立子测试计数。

| 执行者/输入 | 执行与结果 | 原日志/范围 |
|---|---|---|
| 作者，首 fixed clean 工作树 | 全仓普通 `go test ... ./...`，UTC 13:39:58–13:53:18，exit 0；1879 PASS、1 私有书 SKIP、0 FAIL | `/tmp/kepub-native-checks/test.jsonl`，SHA256 `cfd31a7ec273a39f066c31be8fe6a6240fc1512138585b85c38db009591e5b12`；不是修复树重跑 |
| 作者，原 coding 工作树 | 全仓 race UTC 13:53:18–14:22:46，exit 0，1879 PASS/1 SKIP/0 FAIL；vet 14:22:46–14:22:51 exit 0。修复期间运行，必须标 working-tree/mixed，不称任何固定树证据 | PID 11324 已自然结束；`race.jsonl` SHA256 `254e54b9d2bc653de8cda69ac80c97a8d6bb2c6364b6efe0f7579c95dd8a3821`；vet 空日志 SHA256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| 作者，首 fixed 隔离旧树＋新边界测试 | `TestNativeDefinitionByteBoundary`，exit 1，超限实际 exit 1 与期望 2 不符；真实 checker 的恰好 1 MiB 正控成功 | `rawlimit-red.log` SHA256 `7971803e5f8a09ac491dbbc1cc95c1206af8d9501ed45c342c42d5d9aa9a7c67` |
| 作者，修复提交前工作树 | `go test -count=1 -v ./cmd/kepub ./internal/workspace -run 'TestNative\|TestR1JSONByteBudget\|TestCommandOptionMatrix'`，exit 0；包含真实 CLI 生命周期及定义边界 | `rawlimit-green.log` SHA256 `34f9fca31d4266186a4db96f0ecbfa4148da1dd49cb7e77ba00260f5b17e8169` |
| 作者，首 fixed 隔离旧树＋新兼容测试 | `TestNativeSourceForbiddenInImportedState`，exit 1，`source:null` 被错误接纳 | `source-null-red.log` SHA256 `2e7944232b481e1a57e4f4c66bd30fe2a035066265a179c6c13dc33ae1392b1b` |
| 作者，修复提交前工作树 | 同一 source 兼容测试，exit 0；null、对象、大小写别名拒绝，未改 v1 正控重开成功 | `source-null-green.log` SHA256 `3e54d81b527f955e44b223a2bead01ac9f05d4c40e65b27113ce44787ae40c9e` |
| 作者，修复 fixed 独立 worktree | 完整受影响 `./cmd/kepub ./internal/workspace` 普通 UTC 14:07:18–14:21:01、race 14:21:01–14:39:41，均 exit 0，分别 1030 PASS/1 SKIP/0 FAIL；全仓 vet 14:39:41–14:39:43 exit 0 | PID 39071 已自然结束；`/tmp/kepub-native-fixed-checks/{test,race}.jsonl` SHA256 `3ffed6436ba12048c6fc6e9060a211be151ae022e6072a8f0977663aff760237`、`ae312bf088abd5fed093ff0fcbef4a7c38bdd68e5d25bbefdcc13927d7c66c14`；vet 上述空日志 hash |
| 作者，测试加强 fixed 独立 worktree | `TestNativeBookBinaryLifecycle` 普通 14:17:47–14:18:36、race 14:18:36–14:19:26，均 exit 0；另 `go test [-race] -count=1 ./internal/app` 两者 exit 0 | `/tmp/kepub-native-template-checks/{test,race}.log` SHA256 分别 `b75259a8dbc6ff1f8425778d2e52b425fd8050208ec14a02b0e21b1574d9df5a`、`56f92011a4e5bcd16cba6dc8210987d0546e804de4feb61c2e49579984ae6b80`；app 普通/race hash `1d788601af8f6dd080f5afec9b65580d67d2819cd01e6af1a6254d354f734397`、`663f7174e4355779eb1f4818cb77c280ea291a7eb31d2fd79d140c6d88676c9b`；app 无独立落盘计时 |
| 作者，最终测试 fixed 独立 worktree | 带空白/引号的 `TestNativeBookBinaryLifecycle` 普通 14:39:13–14:39:40、race 14:39:40–14:40:05，均 exit 0；`go vet ./cmd/kepub` exit 0，无独立计时 | `/tmp/kepub-native-whitespace-checks/{test,race}.log` SHA256 `c20a8b2dac5aab0c7dad8f13e337b0631ccdb70959adc7daa21b783512d0180b`、`97de65dbb129983c0ceb7f438204e8bea78f5e276459f2e3dd2a2589f6b03cf2`；vet 空日志 hash |
| 作者/父，修复 fixed 独立 worktree | `go build -race` 实际 CLI create/content/plan/apply/diff/accept/export，exit 0；标准库 ZIP oracle 核全部目录/资源、唯一正文替换及所有目标外 bytes | `/tmp/kepub-native-fixed-demo/*.json`；oracle stdout 无原始落盘日志，工具执行结果保存在线程；最终 EPUB SHA256 `129bb507ff1195c88ba395e4f6f74415605b5794a98c2c256dd8eb230483d7f9` |
| 独立 reviewer，首 fixed 独立 checkout | 全仓普通 13:51:57–14:02:36、race 14:02:36–14:17:18、vet 14:17:18–14:17:20，均 exit 0；普通/race 各 12 包 ok，未单列子测试/SKIP 数 | `/home/user/review/logs/suite-fixed/{test,race,vet}.log`；SHA256 分别 `7feb42f8fdf086119991842518282013af034cdf787256f96e0fc259ac56b133`、`aa477089c60c0946f5f141d242f211b1c4851b09aa03d788fd5e0882b590f91b`、上述空日志 hash；独立执行，不搬用作者计数 |
| 独立 reviewer，修复 fixed | `go test [-race] -count=1 -timeout=30m -v ./internal/workspace ./internal/app ./cmd/kepub`，普通 14:17:30–14:24:27、race 14:24:27–14:34:44，各 1112 PASS/1 私有原书 SKIP/0 FAIL，exit 0；`go vet ./...` 14:34:44–14:34:45 exit 0 | `logs/scoped-fixed2/{test,race}.log` SHA256 `f1d46eb3ebd1fc37cb8e61ce7a29cd43d1f67a3eba1a6268293e569e8938b9f7`、`8012520548f664a0642bd6835f46d9c18fb8e4e791f6940f0fba8d786222e2e6`；vet 空日志 hash |
| 独立 reviewer，测试加强 fixed | CLI 整包普通 14:34:55–14:38:47，exit 0，116 PASS/1 SKIP/0 FAIL；native race 23 PASS；CSS/nav 变异 exit 1，XML→HTML 变异仍 exit 0（保留缺口） | CLI 普通 `b9d46fa2e74f9a911274331dd96955c355d84b756702f61ac4271fb65a19b3d0`；其余完整记录见独立报告 `logs/v3`，不能把该身份记为批准 |
| 独立 reviewer，最终测试 fixed | `TestNative` 普通 14:40:25–14:40:54、race 14:40:54–14:41:29，各 23 PASS/0 FAIL/0 SKIP，exit 0；scoped vet exit 0；CSS/nav 变异 14:41:29–14:41:49、XML→HTML 变异 14:41:50–14:42:09，各 exit 1、22 PASS/1 目标 FAIL | `logs/v4` 普通/race SHA256 `f31dd3ade7f505841053c1155d1d2a30ccd6a05f505492965719c49cc4eb81ed`、`b7484762f94e2344ba50b893a0199597226bd518f5f7a2c1abcbaba18db3c4da`；两变异 `5630567d12bcfcf819898131ff19a479a9e4a72b1021f2c63fdb708a18afb30a`、`ea2b3b653ba9fa76e4703b8dde0c2bdc2fb4331a2f9cfff4608112c9b83cb5e5` |

以上短日志均位于 `/tmp/kepub-native-checks`。哈希证明完整性，不证明审查批准。

独立报告原件在 reviewer Orb `.amp/in/artifacts/kepub-native-review/REPORT.md`，SHA256 `2b2327c6e93f4f322732a776869f8f8f7b958eb239a90b39491d669d9d886224`；证据 manifest SHA256 `4cc9bc3cfcb1d0f1153c6374b730a8f9bd8cce66e8dfee19fee469820bc469dc`。父已下载这两个原件并核 hash，原日志及脚本归属 reviewer，不称父重跑；日志时间均为 2026-10-13 UTC。短 red/green probe 无单独落盘时间戳，不补造计时。

父另下载完整独立证据归档 `.amp/in/artifacts/kepub-native-review.tar.gz`（87,212 bytes，SHA256 `8c29da3da5de8066219e6bcf03128116dcdcedb1e1f24c1a04f5bfba524eec11`），检查归档路径/类型后解包，按 manifest 实核 119/119 成员、共 120 文件；独立原日志计数与上述收据相符。核收日志 `/tmp/kepub-native-checks/reviewer-artifact-verification.log` SHA256 `e73ccad3dcee04f011032592653161d59321b9cd151dfe689ec1a1d99e56c828`。这项是父验证证据完整性与适用性，不是独立产品套件的父重跑。

## 实际控制与保真

- 原生 v2 无生成的 original 归档/目录，完整 definition/template/正式报告与初始库存绑定；后续 candidate/accepted/history 沿已有来源门禁。额外用户根目录文件不被当成 imported provenance，不新增无关根库存限制。
- 永久真实 CLI 生命周期核非对称三章、重复章名、Unicode/emoji、XML 字面转义、ZIP mimetype 首项/store、完整库存、OPF/spine/nav、正文与标题编辑、拒绝和接受、导出后按旧导入接口重开；独立期望不从产品 renderer 派生。
- 作者另执行缺 checker 和运行阶段 SIGTERM 控制，分别 `DEPENDENCY_UNAVAILABLE`/3、`CANCELLED`/130，均无目标/staging；不是断电。`controls.log` SHA256 `e1cf71d4535faff5f00ddda38f10f1896a9d892a1e0c0257ef0770236016a4d7`。
- base 老二进制安全拒绝 state v2，所有工作区文件字节不变；纠正后的 `legacy-corrected.log` SHA256 `facc9da96035dc7d37112cbe3005a8af090fd0c3c318a034bfc41a4f1cbc56d9`。不把老二进制当新建功能支持者。
- Chromium 实际渲染导出的五章合成书，DPR 2；DOM 核横排、serif、章节顺序、重复 nav 的独立目标及 `<标签>`/`&`/emoji；正文与目录截图均 view_media 实际检查。生成模板在两个 fixed 间未改变，可复用显示结果；不是 Mac 阅读系统或 EPUB 全部符合性证明。
- 可供用户查看的 EPUB、正文/目录截图保留在本线程 `.amp/in/artifacts/native-*`；公开夹具在 [newbook-v1.json](../examples/newbook-v1.json)。截图不是门禁的替代证据。

## 复用适用性与未完成项

首 fixed → 修复 fixed 全树路径集合相同，全部 mode/type 相同，只有六个 blob 改变：`cmd/kepub/newbook_test.go`、`internal/workspace/newbook.go`、`internal/workspace/newbook_test.go`、`internal/workspace/workspace.go`、`internal/workspace/AGENTS.md`、`docs/DEVELOPMENT_NOTES.md`。`git ls-tree -r` 清单在 `tree-old.txt`/`tree-fixed.txt`，未只按 Go 后缀比较。构建配置、go.mod/go.sum、模板/示例/已有夹具/checker/依赖/操作契约和环境不变。

修复 fixed → 测试加强 fixed 全树路径集合与 mode/type 相同，仅 CLI 生命周期测试及稳定指导两个文件的 blob 改变；产品字节、构建/依赖/原输入夹具与环境完全相同。最终测试 fixed 再只改 CLI 生命周期的一个章节输入和对应独立字面期望，已实际新跑普通/race；不是对改变后的测试直接复用旧结果。两种变异在最终测试 fixed 均被完整字节断言拒绝，未改模板为 green；测试加强 fixed 的 XML→HTML 变异仍 green 原样保留。

受影响两个完整包在作者修复 fixed 新跑普通/race；reviewer 对修复 fixed 的 workspace/app/cmd 三个完整包新跑普通/race，全仓 vet 新跑。其他 9 包的源码、依赖和包图相同，明确复用首 fixed 独立全仓证据，不将 mixed 作者 race 用作 fixed 证明。reviewer 用 `-trimpath -buildvcs=false` 核修复 fixed 与最终测试 fixed 二进制同 SHA256 `81fccdbcb5ae8207ced106678336bada71f5c8e83c41fdbf58fc89a242e0a273`，并对最终测试 fixed 出具适用性接受。

随后的文档增量只增加收据、修正链接/本地状态，并澄清已测行为：必填文本非空白、v2 不生成或消费伪造 original、禁止非空 originalSHA256。不改产品、测试、构建、schema、预算或验收门槛；最终文档身份仍须 reviewer 单独核适用性，不自动迁移产品批准。

保留的工具错误：bundle 初次目标路径错误后改用 reviewer 工作区；旧 binary scratch oracle 原误要求 `data:null`，按既有 typed envelope 校准后通过；source-null red 首次相对复制路径错误导致“无测试”，纠正后真实目标失败；fixed race CLI 首次漏 task ID，exit 2，保留 `diff-command-error.json`，从已有实际 task 继续正确 diff/accept/export，未重跑冒充原执行。这些不是产品 PASS，也不计同一产品修复两次失败。

Mac、真正断电、发布后 syncDir 故障注入、磁盘满、封面/图片/章节重排/两层目录、完整 F3 样书、发行安装验收未测或未实现。reviewer 未跑 race CLI binary；父实际执行，不混淆归属。私有书 SKIP 单列；首树真实 FAIL 不被全仓 PASS 抵消，分别保留 red/green 与测试缺口。既有嵌套输出位置可破坏另一工作区的问题在 base 同样存在，非本批新增且未修复，留待后续输出位置门禁任务。

## 独立结论与交付状态

- reviewer：接受最终测试 fixed `83e1be6384ae893618e605e27b5c3180070b1856` / tree `c8aa5e80fc21589f68b62417e542453f8ba94498`，仅本批范围；首 fixed 拒绝，修复/测试加强两个中间身份尚未核准，原 FAIL/缺口保留，不倒填批准。
- 父端：接受同一产品/测试 fixed。已核内容保留、完整树差异与依赖/环境适用性，作者及独立原日志归属、两个产品反例的 red/green、两种模板 mutation 判别力；实际 race CLI 完整 ZIP 字节组合控制、受影响普通/race/vet 全部自然完成。随后纯文档身份的适用性以独立线程最终核收为准，不虚构该身份全仓重跑。
- 交付：两个产品提交和两个测试加强提交在本地 feature 分支，收据作为单独文档提交；本地 main/origin/main 均未动，无 PR/CI 订阅、push、merge、release、Issue 关闭。
- 下一步：文档核收后本首增量达到 Accepted locally，不代表全 M1/F3 Done；后续按计划补资源/章节/图片等能力，继续逐批授权与验收。

## 2026-10-13 远端交付补记（原本地执行记录保留）

- 独立 docs-only 结论已接受最终 head [640b3c8](https://github.com/LeviTK/Kepub/commit/640b3c866a9847126173abb9224f2a82132b5fef) / tree `9e510bd0401f51a0d4cde70d94edf0e53149e1ec`。附录 `.amp/in/artifacts/kepub-native-review-docs-addendum.md` SHA256 `76a6f7785269d5458aa5cd4e201dd793be1eedb7cb2e576c3415a0978c37448b`；原独立报告/归档未改，产品证据按适用性沿用，不补称 docs-only 身份全仓重跑。
- 用户随后授权 push/PR/Issue 进度/CI，另明确授权 CI/范围核对通过后的 merge 与关闭 #44/#45。[PR #75](https://github.com/LeviTK/Kepub/pull/75) 已交付并订阅评论/审查及准确 head CI；未删除分支、发布、部署或关闭父项。
- [GitHub Linux CI](https://github.com/LeviTK/Kepub/actions/runs/38424759071)，ubuntu-24.04，准确 head/tree 如上，Go 1.27.1 linux/amd64、Temurin 17.0.20.1、EPUBCheck 5.3.0、固定 JAR/依赖校验通过。实际 `go test [-race] -p=1 -parallel=1 -count=1 -timeout=30m -json ./...`：普通 UTC 15:45:43–16:01:53、race 16:01:53–16:29:05，每轮 1881 测试/子测试 PASS、1 私有书 SKIP、0 FAIL，12 测试包 PASS，另 2 无测试包 SKIP；exit 0/0。vet 16:29:05–16:29:08、普通/race build 16:29:08–16:29:09 均成功。CI build race binary 不等于用它跑全部 CLI；父端真实 race CLI 仍归原父执行。
- artifact `core-ci-640b3c866a9847126173abb9224f2a82132b5fef` 已下载至父 Orb `/tmp/kepub-native-ci-results` 并按 JSON Action/是否有 Test 分开核计数；原始 artifact 保留 14 天，下表和公开 PR 索引永久记录身份/hash/范围，不将短期下载地址作为永久证据。

| CI 日志 | SHA256 |
|---|---|
| input.log | `038c3be53181a822d95c4c6e8c260ce5347a2f0b5b0362f3c1e9863792f99f54` |
| test.jsonl | `cddbc9893d3a8d63608785572f5925996c3714a903604db1e79bfd92f5c852fb` |
| race.jsonl | `60c6dc41887fbfd63425fd24591bae1b6019df198290b293ad73e2bc9b2cc5d8` |
| vet.log | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| binaries.log | `dfa7983c2743b821a8dab6571fe98d4bbd07ad514c5266d81aeb35fc7ce7934e` |

- 合并前 fresh-fetch 核 base 仍为原 `692bacb`，head 未动，无额外 GitHub review/comment；用户授权后用 match-head 正常 merge。[合并提交 3a7d1b4](https://github.com/LeviTK/Kepub/commit/3a7d1b414c5d23a1fbff42890e1c226f097e1a93) 于 UTC 16:30:19 发布，父为原 base 和已获批 head；合并后 fresh-fetch 的 tree 与准确 head 相同，`git diff --exit-code 640b3c866a9847126173abb9224f2a82132b5fef origin/main` exit 0，全部路径/mode/type/blob 一致。
- 父主线复测评估：产品、源码、构建配置、依赖、夹具、验证配置没有组合差异，已核独立/父/CI 证据对同一内容适用，明确复用/无需重复全套；没有手动触发 main CI，不称 merge SHA 新跑 PASS。默认 build 的 VCS metadata 会反映 merge 身份，不能据 tree 相同称默认 binary hash 相同。CI 的 Node20/setup-java v4 弃用提示非失败，本轮未升级供应链。
- 本地 main 已快进至合并提交；#44/#45 四项清单各按证据核准后关闭，#64/#33 等父项仍 OPEN。下一批本地 `feat/core-metadata-v2` 从该 main 建立，仅起草 B3 契约，尚未产品实现/接受/远端交付。Mac、断电、磁盘满及完整 M1/F3/T2/发行未完成状态不变。
