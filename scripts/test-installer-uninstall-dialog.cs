using System;
using System.Runtime.InteropServices;
using System.Text;
public static class BcmUninstallDialog {
    private delegate bool Callback(IntPtr window, IntPtr state);
    [DllImport("user32.dll")] private static extern bool EnumWindows(Callback callback, IntPtr state);
    [DllImport("user32.dll")] private static extern bool EnumChildWindows(IntPtr parent, Callback callback, IntPtr state);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] private static extern int GetWindowText(IntPtr window, StringBuilder text, int count);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] private static extern int GetClassName(IntPtr window, StringBuilder text, int count);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(IntPtr window);
    [DllImport("user32.dll")] private static extern IntPtr GetDlgItem(IntPtr window, int id);
    [DllImport("user32.dll")] private static extern int GetDlgCtrlID(IntPtr window);
    [DllImport("user32.dll")] private static extern bool PostMessage(IntPtr window, uint message, IntPtr wParam, IntPtr lParam);
    public static string Describe(int[] owners) {
        StringBuilder result = new StringBuilder();
        EnumWindows(delegate(IntPtr window, IntPtr ignored) {
            uint owner;
            GetWindowThreadProcessId(window, out owner);
            if (Array.IndexOf(owners, (int)owner) < 0 || !IsWindowVisible(window)) return true;
            EnumChildWindows(window, delegate(IntPtr child, IntPtr unused) {
                StringBuilder text = new StringBuilder(256);
                GetWindowText(child, text, text.Capacity);
                if (IsWindowVisible(child)) result.AppendFormat("[{0}: {1}]", GetDlgCtrlID(child), text);
                return true;
            }, IntPtr.Zero);
            return true;
        }, IntPtr.Zero);
        return result.ToString();
    }
    public static bool Click(int[] owners, int id) {
        bool clicked = false;
        EnumWindows(delegate(IntPtr window, IntPtr ignored) {
            uint owner;
            GetWindowThreadProcessId(window, out owner);
            if (Array.IndexOf(owners, (int)owner) < 0 || !IsWindowVisible(window)) return true;
            IntPtr button = GetDlgItem(window, id);
            if ((button == IntPtr.Zero || !IsWindowVisible(button)) && id == 1) {
                // Windows may assign IDCANCEL to the sole acknowledgement
                // button of an exception message box. Restrict this fallback
                // to native dialogs, never the progress form's Cancel button.
                StringBuilder windowClass = new StringBuilder(64);
                GetClassName(window, windowClass, windowClass.Capacity);
                if (windowClass.ToString() == "#32770" && GetDlgItem(window, 6) == IntPtr.Zero && GetDlgItem(window, 7) == IntPtr.Zero)
                    button = GetDlgItem(window, 2);
            }
            if (button == IntPtr.Zero || !IsWindowVisible(button)) return true;
            clicked = PostMessage(button, 0x00F5, IntPtr.Zero, IntPtr.Zero);
            return !clicked;
        }, IntPtr.Zero);
        return clicked;
    }
}
