; Installer steps of Power Meter, included by the NSIS script that
; `mygo build` writes (mygo.json windows.installerInclude).
;
; Exit codes of PowerMeter.exe --sync-autostart and --remove-autostart:
; 0 done or nothing to do; 1 failed; 3 the protected copy needs an elevated
; run (sync: outdated copy; remove: task gone, copy left); 4 sync: the task
; runs a file standard processes can replace; 5 sync: an unused copy;
; 6 remove: a same-name task that is not this program's was left unchanged.

; Power Meter 1.x was a .NET program installed by Inno Setup under this
; AppId, into the same folder. It is not upgraded in place.
!define PM_OLD_UNINSTALL "Software\Microsoft\Windows\CurrentVersion\Uninstall\{FDDC9FC9-109E-4B41-AE4A-BA30420295D0}_is1"
!define PM_EXE "PowerMeter.exe"

LangString PM_OLD_VERSION ${LANG_ENGLISH} "An earlier Power Meter (version 1.x) is installed. Uninstall it in Settings > Apps first, then run this installer again."
LangString PM_COPY_NOT_UPDATED ${LANG_ENGLISH} "Logon startup keeps running the previous version from its protected copy. Open Power Meter as administrator to update it."
LangString PM_DISARMED ${LANG_ENGLISH} "Logon startup ran a file that standard processes can replace, so it has been turned off. Open Power Meter as administrator and enable it again."
LangString PM_DISARM_FAILED ${LANG_ENGLISH} "Logon startup runs a file that standard processes can replace and could not be turned off. Open Power Meter as administrator and enable or disable its logon startup."
LangString PM_CLEANUP_FAILED ${LANG_ENGLISH} "Unable to remove this installation's logon task. Uninstall has been stopped; disable its logon startup as administrator, then retry."
LangString PM_COPY_REMAINS ${LANG_ENGLISH} "The logon task was removed, but its protected copy under Program Files\Power Meter\autostart needs administrator access to delete. It no longer starts automatically."
LangString PM_FOREIGN_TASK ${LANG_ENGLISH} "A task in the Task Scheduler Library root named PowerMeter.Logon. followed by your user SID does not belong to Power Meter, so it was left unchanged and uninstall continues. If you no longer need it, delete it in Task Scheduler (taskschd.msc)."
!ifdef LANG_SIMPCHINESE
LangString PM_OLD_VERSION ${LANG_SIMPCHINESE} "已安装旧版功率计（1.x 版）。请先在“设置 > 应用”中卸载旧版，再运行本安装程序。"
LangString PM_COPY_NOT_UPDATED ${LANG_SIMPCHINESE} "开机自启仍运行受保护副本中的旧版本。以管理员身份打开功率计即可更新。"
LangString PM_DISARMED ${LANG_SIMPCHINESE} "开机自启运行的是普通权限可替换的文件，已将其关闭。请以管理员身份打开功率计后重新启用。"
LangString PM_DISARM_FAILED ${LANG_SIMPCHINESE} "开机自启运行的是普通权限可替换的文件，且未能关闭。请以管理员身份打开功率计，重新启用或关闭开机自启。"
LangString PM_CLEANUP_FAILED ${LANG_SIMPCHINESE} "无法删除此安装的自启任务。卸载已停止，请以管理员身份关闭开机自启后重试。"
LangString PM_COPY_REMAINS ${LANG_SIMPCHINESE} "自启任务已删除，但 Program Files\Power Meter\autostart 下的受保护副本需要管理员权限才能删除。它不会再自动运行。"
LangString PM_FOREIGN_TASK ${LANG_SIMPCHINESE} "任务计划程序库根目录中名为 PowerMeter.Logon. 加你的用户 SID 的任务不属于功率计，已保留未改，卸载继续进行。如不再需要，请在“任务计划程序”（taskschd.msc）中删除。"
!endif

; The installer stops while Power Meter 1.x is installed, per user or for
; all users.
!macro customInit
  ReadRegStr $0 HKCU "${PM_OLD_UNINSTALL}" "UninstallString"
  StrCmp $0 "" 0 pm_old_found
  SetRegView 64
  ReadRegStr $0 HKLM "${PM_OLD_UNINSTALL}" "UninstallString"
  SetRegView 32
  StrCmp $0 "" 0 pm_old_found
  ReadRegStr $0 HKLM "${PM_OLD_UNINSTALL}" "UninstallString"
  StrCmp $0 "" pm_old_none
  pm_old_found:
    MessageBox MB_OK|MB_ICONSTOP "$(PM_OLD_VERSION)" /SD IDOK
    SetErrorLevel 3
    Quit
  pm_old_none:
!macroend

; After the files are in place: logon startup runs an administrator-only
; copy of the program, which an upgrade refreshes with one UAC prompt. A
; task that still runs a user-writable file is turned off when it cannot be
; fixed.
!macro customInstall
  ClearErrors
  ExecWait '"$INSTDIR\${PM_EXE}" --sync-autostart' $0
  IfErrors pm_sync_done
  IntCmp $0 3 pm_sync_elevate
  IntCmp $0 4 pm_sync_elevate
  Goto pm_sync_done
  pm_sync_elevate:
    ExecShellWait "runas" "$INSTDIR\${PM_EXE}" "--sync-autostart" SW_HIDE
    ; The elevated run returns no exit code here: ask an ordinary run what
    ; is still left to do.
    ExecWait '"$INSTDIR\${PM_EXE}" --sync-autostart' $0
    IntCmp $0 3 pm_sync_stale
    IntCmp $0 4 pm_sync_unprotected
    Goto pm_sync_done
  pm_sync_stale:
    MessageBox MB_OK|MB_ICONINFORMATION "$(PM_COPY_NOT_UPDATED)" /SD IDOK
    Goto pm_sync_done
  pm_sync_unprotected:
    ExecWait '"$INSTDIR\${PM_EXE}" --remove-autostart' $1
    IntCmp $1 1 pm_sync_disarm_failed
    MessageBox MB_OK|MB_ICONINFORMATION "$(PM_DISARMED)" /SD IDOK
    Goto pm_sync_done
  pm_sync_disarm_failed:
    MessageBox MB_OK|MB_ICONSTOP "$(PM_DISARM_FAILED)" /SD IDOK
  pm_sync_done:
!macroend

; Logon startup is removed when the uninstaller starts, before anything is
; deleted, so a failed cleanup stops the uninstall and keeps the program.
Function un.onInit
  IfFileExists "$INSTDIR\${PM_EXE}" 0 pm_un_done
  ClearErrors
  ExecWait '"$INSTDIR\${PM_EXE}" --remove-autostart' $0
  IfErrors pm_un_failed
  IntCmp $0 0 pm_un_done
  IntCmp $0 6 pm_un_foreign
  IntCmp $0 3 0 pm_un_failed pm_un_failed
  ; The task is gone; only this installation's inert protected copy is
  ; left, which an elevated run deletes.
  ExecShellWait "runas" "$INSTDIR\${PM_EXE}" "--remove-autostart" SW_HIDE
  ExecWait '"$INSTDIR\${PM_EXE}" --remove-autostart' $0
  IntCmp $0 0 pm_un_done
  IntCmp $0 6 pm_un_foreign
  MessageBox MB_OK|MB_ICONINFORMATION "$(PM_COPY_REMAINS)" /SD IDOK
  Goto pm_un_done
  pm_un_foreign:
    MessageBox MB_OK|MB_ICONINFORMATION "$(PM_FOREIGN_TASK)" /SD IDOK
    Goto pm_un_done
  pm_un_failed:
    MessageBox MB_OK|MB_ICONSTOP "$(PM_CLEANUP_FAILED)" /SD IDOK
    SetErrorLevel 1
    Quit
  pm_un_done:
FunctionEnd
