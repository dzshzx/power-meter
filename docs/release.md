# 发布

> 本页说明 Release 工作流、版本计划、打 tag 与公开产物复验；项目概览与构建见仓根 `README.md`。

项目使用 GitHub Actions 构建和发布：

- 日常改动完成相应本地验证后，用 `land --no-recut` 快进同步 `master`。
  GitHub 保留手动 Windows validation 和正式 Release 的 Windows 构建、真实安装/卸载验收。
  依赖升级由 agent 在 Windows 宿主跑测试并手动触发 Windows validation，通过后合入；只有证据表明升级会出问题时才交维护者。
- 推送符合 `vX.Y.Z` 格式的 tag 时，Release 工作流会校验 tag 与 manifest
  版本一致、tag 为带注解 tag 且位于 `master`，重新构建程序，同时生成便携 EXE、
  当前用户安装包及各自的 SHA-256 校验文件，并随第三方通知和对应源码包创建
  GitHub Release。

发布新版本时，先从远端 tag 历史生成完整的只读版本计划，再修改
`src/BatteryChargeMeter.manifest` 的四段版本号：

```powershell
$planSha = '75bf5ca95a191bd97df2500521bcb7aad6136e7c'
$planDir = Join-Path ([IO.Path]::GetTempPath()) "version-plan-$planSha"
New-Item -ItemType Directory -Force -Path $planDir | Out-Null
foreach ($name in 'version_plan.py', 'version_plan.py.lock') {
    Invoke-WebRequest -UseBasicParsing -OutFile (Join-Path $planDir $name) `
      -Uri "https://raw.githubusercontent.com/dzshzx/agent-skills/$planSha/scripts/$name"
}
uv run --script --locked (Join-Path $planDir 'version_plan.py') plan `
  --repository dzshzx/power-meter `
  --target v=X.Y.Z
```

计划只读输出“基线到目标”。版本档位标准：

- 默认 patch。
- minor（有用户能感知的新能力）须先经维护者确认。
- major（含 0.x→1.0）须先经维护者确认。内部重构、目录搬迁、删兼容层不算破坏性变更。

版本计划脚本由 agent-skills 仓共享（`scripts/version_plan.py` 及其锁文件），
按 40 位提交 SHA 拉取，CI 与本地用同一个 SHA；升级时同时改本段与
`.github/workflows/release.yml` 里的 `$planSha`。

基线未知和降级会直接拒绝；已发布的 tag 不能复用。本地验证通过且提交经 `land`
快进进入 `master` 后，在已授权发版范围内创建匹配的带注解 tag；Release 在该 SHA 上完整验收。
例如从 `v1.2.1` 升到 `v1.2.2` 时，manifest 为 `1.2.2.0`：

```powershell
# 核对 master 为已经本地验证并获准发布的精确 SHA
git tag -a v1.2.2 -m "Release v1.2.2"
git push origin v1.2.2
```

Release 会排除本次 tag 并从远端记录重建计划，基线未知或降级时在创建 Release 前拒绝；
旧 tag 上的 `Version-Approval` trailer 会被忽略。

Release 完成后，在 Windows 交互桌面上从本仓库 checkout 复验公开产物：

```powershell
pwsh -NoProfile -File .\scripts\verify-release.ps1 -Version X.Y.Z
```

脚本下载该版本全部资产，核对 SHA-256 文件、manifest 与安装包版本、`asInvoker`、
自检、导出通知及对应源码包，并在下载目录写出 `verification.json` 与窗口截图。

远端发布 tag 不移动、不复用；若 tag 后才发现失败，修复后发布下一个 patch。

发布产物只存在于 GitHub Release，不直接提交到仓库。
