# 发布

> 本页说明怎样发版、版本号从哪里来、出错后怎么办，以及公开产物复验；项目概览与构建见仓根 `README.md`。

日常改动完成相应本地验证后合入并快进同步 `master`。依赖升级由 agent 在 Windows 宿主跑测试并手动触发
Windows validation，通过后合入；只有证据表明升级会出问题时才交维护者。

## 发版

发版只有一条命令，手动触发 `.github/workflows/release.yml`：

```sh
gh workflow run release.yml --ref master -f bump=patch
```

输入只有两个：

- `bump`：`patch`（默认）、`minor` 或 `major`。默认 patch；minor（有用户能感知的新能力）与
  major（含 0.x→1.0）须先经维护者确认。内部重构、目录搬迁、删兼容层不算破坏性变更。
- `dry_run`：默认 `false`。设为 `true` 时可在任意分支运行，完整构建和核对，但不打 tag、不发布；
  运行摘要列出将要发布的版本、源提交和各资产的 SHA-256。

工作流依次：

1. 非 dry_run 时要求运行在 `master`。先判断这个提交是否已发过版：提交上有纯 `vX.Y.Z` tag、且同名
   Release 已发布，才算发过版，非 dry_run 直接失败并说明「这个提交已发过版」。提交上只有 tag、
   同名 Release 没有发布时，视为上次发版没做完，沿用这个版本继续发布，不再升档。这一步在门禁之前。
2. 算出新版本号（见下节）。
3. 在 Linux runner 上跑门禁 `bash scripts/check.sh`；在 Windows runner 上用
   `go run ./tools/release set-version` 把新版本写进 `mygo.json`，取 MiSans 字体，跑 Windows 上的
   `go test`（含提权的自启实测），`go tool mygo build` 构建 EXE 与安装包，再跑
   `tools/release smoke`（`--self-test`、通知导出、截图与预览等烟测）和 `tools/release install-test`
   （拒绝覆盖 1.5 及更早的版本、安装、覆盖安装、卸载）。
4. `tools/release package` 生成便携 EXE、当前用户安装包、第三方通知与对应源码包，每个资产旁各带
   一个 `.sha256` 校验文件，全部放在一个只放发布资产的目录里，并核对 EXE 的文件版本是新版本、
   每个校验文件都对、源码包是 `third_party/NOTICE.md` 固定的那一份。
5. 全部通过后依次：删掉同名的旧草稿 Release，建含全部资产的草稿，经 GitHub API 在该提交上打附注
   tag `vX.Y.Z`（tag 已指向本提交就跳过，指向别的提交就失败），最后发布草稿；Release 说明写明
   `source_commit`。

## 版本号

- 基线是仓里所有纯 `vX.Y.Z` tag（三段数字，无前后缀）中按语义化版本比较最高的那个，按 `bump` 升一档。
  版本因此只升不降。
- 仓里 `mygo.json` 的 `version` 固定为开发占位值 `0.0.0`，本地构建的 EXE 与安装包显示的就是这个
  占位版本。Release 工作流只在构建时把它改成 `X.Y.Z`，不提交回仓库。
- EXE 的文件版本是 `X.Y.Z.0`，安装包登记的 `DisplayVersion` 是 `X.Y.Z`，都由 `mygo build` 按
  `mygo.json` 写入。安装包按固定的标识 `io.github.dzshzx.powermeter` 覆盖原安装，不比较版本号；
  本地占位版本不会进入正式包。1.5 及更早的版本（C# 程序，Inno Setup 安装）不能就地升级：
  安装包发现它就停下，请用户先卸载。
- 没有另外的构建号。

## 出错后怎么办

- 任何一步失败，恢复办法都是修好后用同一条触发命令重跑。打 tag 之前失败时，重跑按同一个 `bump`
  算出同一个版本，并替换上次留下的草稿；打 tag 之后失败时，重跑认出这个提交上的 tag，沿用该版本
  继续发布。不需要手工删 tag、删草稿或发布草稿。
- 已发布的 tag 不移动、不删除、不复用；发布后才发现问题时向前修，修代码后发下一个 patch。

## 公开产物复验

Release 完成后，在 Windows 交互桌面上从本仓库 checkout 复验公开产物：

```sh
go run ./tools/release verify -version X.Y.Z -out <空目录>
```

它下载该版本全部资产，核对每个资产的 `.sha256` 文件、EXE 文件版本、`asInvoker`、烟测、导出的
通知与发布的通知一致及对应源码包，并在输出目录写出 `verification.json` 与窗口截图。

发布产物只存在于 GitHub Release，不直接提交到仓库。
