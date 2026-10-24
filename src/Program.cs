using System;
using System.IO;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    internal static class Program
    {
        // An elevated launch migrates an unprotected task and refreshes this
        // installation's protected copy. Failures leave the state for the
        // settings panel to report.
        private static void SynchronizeAutostart()
        {
            try
            {
                using (AutostartManager manager = AutostartManager.ForCurrentExecutable(Application.ExecutablePath))
                    manager.Synchronize(true);
            }
            catch (Exception)
            {
            }
        }

        [STAThread]
        private static void Main(string[] args)
        {
            EmbeddedLibraries.Initialize();
            Run(args);
        }

        [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.NoInlining)]
        private static void Run(string[] args)
        {
            StartupRoute route = StartupRoute.Parse(args);
            if (!route.Valid)
            {
                Environment.ExitCode = 2;
                return;
            }
            if (route.Command == "gui")
            {
                using (GuiInstance instance = new GuiInstance())
                {
                    if (!instance.Acquire(Startup.Identity, route.Handoff))
                    {
                        if (!route.StartHidden)
                            GuiInstance.RestoreExisting(Startup.Identity);
                        return;
                    }
                    ElevationResult elevation = Startup.Route(route, Startup.IsElevated(), Startup.TryElevate);
                    if (elevation.Started)
                        return;
                    if (Startup.IsElevated())
                        SynchronizeAutostart();
                    Application.EnableVisualStyles();
                    Application.SetCompatibleTextRenderingDefault(false);
                    Application.Run(new MainForm(true, elevation.Message, route.StartHidden));
                }
                return;
            }
            if (route.Command == "--remove-autostart")
            {
                // 0: removed or absent; 3: task gone, but deleting this
                // installation's protected copy needs an elevated rerun;
                // 6: removed, but a same-name task that is not this program's
                // was left unchanged (see AutostartRemoval).
                try
                {
                    using (AutostartManager manager = AutostartManager.ForCurrentExecutable(Application.ExecutablePath))
                        Environment.ExitCode = (int)manager.Disable();
                }
                catch
                {
                    Environment.ExitCode = 1;
                }
                return;
            }
            if (route.Command == "--sync-autostart")
            {
                // Elevated: apply and return 0. Ordinary token: report what an
                // elevated run would change (3 outdated copy, 4 unprotected
                // task, 5 unused copy) without requesting UAC.
                try
                {
                    using (AutostartManager manager = AutostartManager.ForCurrentExecutable(Application.ExecutablePath))
                        Environment.ExitCode = (int)manager.Synchronize(Startup.IsElevated());
                }
                catch
                {
                    Environment.ExitCode = 1;
                }
                return;
            }
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);

            switch (route.Command)
            {
                case "--screenshot":
                    using (MainForm preview = new MainForm(false))
                        preview.RenderPreview(route.Path);
                    return;
                case "--dpi-preview":
                    using (MainForm preview = new MainForm(false))
                        preview.RenderDpiTransitionPreview(route.Path, route.Dpis);
                    return;
                case "--self-test":
                    bool passed;
                    string report = PowerSelfTest.Run(out passed);
                    File.WriteAllText(route.Path, report);
                    Environment.Exit(passed ? 0 : 1);
                    return;
                case "--third-party-notices":
                    ThirdPartyNotices.WriteTo(route.Path);
                    return;
                case "--power-probe":
                    File.WriteAllText(route.Path, PowerDiagnostics.Run(route.Seconds));
                    return;
                case "--tray-preview":
                    using (MainForm preview = new MainForm(false))
                        preview.RenderTrayIconPreview(route.Path, route.TrayGlyph, route.TrayDischarging);
                    return;
            }

            Environment.ExitCode = 2;
        }
    }
}
