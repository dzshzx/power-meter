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

1. 非 dry_run 时要求运行在 `master`，且该提交还没有 `vX.Y.Z` tag，否则失败并说明「这个提交已发过版」。
2. 算出新版本号（见下节）。
3. 在 Linux runner 上跑门禁 `bash scripts/check.sh`；在 Windows runner 上把新版本写进 manifest，
   安装固定版本的 Inno Setup 7，运行 `scripts/test.ps1 -RequireInstaller`（含 `--self-test` 等烟测与
   真实安装/卸载验收）和依赖许可检查。
4. 生成便携 EXE、当前用户安装包及各自的 SHA-256 校验文件、第三方通知与对应源码包，
   再用 `scripts/assert-release-version.ps1` 核对 EXE 内嵌 manifest 与安装包文件版本都是新版本。
5. 全部通过后先建含全部资产的草稿 Release，再在该提交上打附注 tag `vX.Y.Z` 并推送，最后发布草稿；
   Release 说明写明 `source_commit`。

## 版本号

- 基线是仓里所有纯 `vX.Y.Z` tag（三段数字，无前后缀）中按语义化版本比较最高的那个，按 `bump` 升一档。
  版本因此只升不降。
- 仓里 `src/BatteryChargeMeter.manifest` 的版本固定为开发占位值 `0.0.0.0`，本地构建的 EXE 显示的
  就是这个占位版本。Release 工作流只在构建时把它改成 `X.Y.Z.0`，不提交回仓库。
- 安装包的 `AppVersion` 是 `X.Y.Z`，文件版本是 `X.Y.Z.0`，都由打包时的 tag 给出。安装包升级按固定的
  `AppId` 覆盖原安装，文件都带 `ignoreversion`，不比较版本号；本地占位版本不会进入正式包。
- 没有另外的构建号。

## 出错后怎么办

- 任何一步失败都不会留下 tag：推 tag 之前失败时，工作流会删掉已建的草稿 Release。修好后用同一个
  `bump` 重跑即可。
- 若 tag 已推送而发布草稿这一步失败，Release 停在草稿：核对后在 GitHub 上发布该草稿，或运行
  `gh release edit vX.Y.Z --draft=false`。
- 已发布的 tag 不移动、不删除、不复用；发布后才发现问题时向前修，修代码后发下一个 patch。

## 公开产物复验

Release 完成后，在 Windows 交互桌面上从本仓库 checkout 复验公开产物：

```powershell
pwsh -NoProfile -File .\scripts\verify-release.ps1 -Version X.Y.Z
```

脚本下载该版本全部资产，核对 SHA-256 文件、manifest 与安装包版本、`asInvoker`、
自检、导出通知及对应源码包，并在下载目录写出 `verification.json` 与窗口截图。

发布产物只存在于 GitHub Release，不直接提交到仓库。
