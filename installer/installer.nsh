; Installer steps of Power Meter, included by the NSIS script that
; `mygo build` writes (mygo.json windows.installerInclude).
;
; Exit codes of PowerMeter.exe --sync-autostart and --remove-autostart:
; 0 done or nothing to do; 1 failed; 3 the protected copy needs an elevated
; run (sync: outdated copy; remove: task gone, copy left); 5 sync: an unused
; copy; 6 remove: a same-name task that is not this program's was left
; unchanged.

; Power Meter 1.5 and earlier was a .NET program installed by Inno Setup
; under this AppId. It is not upgraded in place: nothing of it is migrated.
!define PM_OLD_UNINSTALL "Software\Microsoft\Windows\CurrentVersion\Uninstall\{FDDC9FC9-109E-4B41-AE4A-BA30420295D0}_is1"
!define PM_EXE "PowerMeter.exe"
!define PM_NOTICES "THIRD-PARTY-NOTICES.txt"

LangString PM_OLD_VERSION ${LANG_ENGLISH} "An earlier Power Meter (version 1.5 or earlier) is installed. Uninstall it in Settings > Apps first, then run this installer again."
LangString PM_COPY_NOT_UPDATED ${LANG_ENGLISH} "Logon startup keeps running the previous version from its protected copy. Open Power Meter as administrator to update it."
LangString PM_CLEANUP_FAILED ${LANG_ENGLISH} "Unable to remove this installation's logon task. Uninstall has been stopped; disable its logon startup as administrator, then retry."
LangString PM_COPY_REMAINS ${LANG_ENGLISH} "The logon task was removed, but its protected copy under Program Files\Power Meter\autostart needs administrator access to delete. It no longer starts automatically."
LangString PM_FOREIGN_TASK ${LANG_ENGLISH} "A task in the Task Scheduler Library root named PowerMeter.Logon. followed by your user SID does not belong to Power Meter, so it was left unchanged and uninstall continues. If you no longer need it, delete it in Task Scheduler (taskschd.msc)."
!ifdef LANG_SIMPCHINESE
LangString PM_OLD_VERSION ${LANG_SIMPCHINESE} "已安装旧版功率计（1.5 及更早的版本）。请先在“设置 > 应用”中卸载旧版，再运行本安装程序。"
LangString PM_COPY_NOT_UPDATED ${LANG_SIMPCHINESE} "开机自启仍运行受保护副本中的旧版本。以管理员身份打开功率计即可更新。"
LangString PM_CLEANUP_FAILED ${LANG_SIMPCHINESE} "无法删除此安装的自启任务。卸载已停止，请以管理员身份关闭开机自启后重试。"
LangString PM_COPY_REMAINS ${LANG_SIMPCHINESE} "自启任务已删除，但 Program Files\Power Meter\autostart 下的受保护副本需要管理员权限才能删除。它不会再自动运行。"
LangString PM_FOREIGN_TASK ${LANG_SIMPCHINESE} "任务计划程序库根目录中名为 PowerMeter.Logon. 加你的用户 SID 的任务不属于功率计，已保留未改，卸载继续进行。如不再需要，请在“任务计划程序”（taskschd.msc）中删除。"
!endif

; The installer stops while Power Meter 1.5 or earlier is installed. Its
; Inno Setup installer only ever installed for the current user.
!macro customInit
  ReadRegStr $0 HKCU "${PM_OLD_UNINSTALL}" "UninstallString"
  StrCmp $0 "" pm_old_none
    MessageBox MB_OK|MB_ICONSTOP "$(PM_OLD_VERSION)" /SD IDOK
    SetErrorLevel 3
    Quit
  pm_old_none:
!macroend

; After the files are in place: logon startup runs an administrator-only
; copy of the program, which an upgrade refreshes with one UAC prompt.
!macro customInstall
  ; The notices and licences the program embeds, beside it, as the LGPL
  ; asks for IntelMSR.bin. The uninstaller removes the whole folder.
  ExecWait '"$INSTDIR\${PM_EXE}" --third-party-notices "$INSTDIR\${PM_NOTICES}"'
  ClearErrors
  ExecWait '"$INSTDIR\${PM_EXE}" --sync-autostart' $0
  IfErrors pm_sync_done
  IntCmp $0 3 0 pm_sync_done pm_sync_done
    ExecShellWait "runas" "$INSTDIR\${PM_EXE}" "--sync-autostart" SW_HIDE
    ; The elevated run returns no exit code here: ask an ordinary run
    ; whether the copy is still outdated.
    ExecWait '"$INSTDIR\${PM_EXE}" --sync-autostart' $0
    IntCmp $0 3 0 pm_sync_done pm_sync_done
    MessageBox MB_OK|MB_ICONINFORMATION "$(PM_COPY_NOT_UPDATED)" /SD IDOK
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
