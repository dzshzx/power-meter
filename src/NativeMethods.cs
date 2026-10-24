using System;
using System.Runtime.InteropServices;

namespace BatteryChargeMeter
{
    internal static class NativeMethods
    {
        internal const int WmDpiChanged = 0x02E0;
        internal const int WmNcCalcSize = 0x0083;
        internal const int WmNcHitTest = 0x0084;
        internal const int WmSysCommand = 0x0112;
        internal const int WmStyleChanging = 0x007C;
        internal const int HtClient = 1;
        internal const int HtCaption = 2;
        internal const int ScMaximize = 0xF030;
        internal const int GwlStyle = -16;
        internal const int WsCaption = 0x00C00000;
        internal const int WsSysMenu = 0x00080000;
        internal const int WsMinimizeBox = 0x00020000;
        internal const int WsMaximizeBox = 0x00010000;
        internal const int WsVScroll = 0x00200000;
        internal const int WsHScroll = 0x00100000;
        internal const int SmCxVScroll = 2;
        internal const int SmCyHScroll = 3;
        internal const uint SwpNoSize = 0x0001;
        internal const uint SwpNoMove = 0x0002;
        internal const uint SwpNoZOrder = 0x0004;
        internal const uint SwpNoActivate = 0x0010;
        internal const uint SwpFrameChanged = 0x0020;
        internal const int DwmUseImmersiveDarkMode = 20;
        internal const int DwmWindowCornerPreference = 33;
        internal const int DwmBorderColor = 34;
        internal const int DwmCornerRound = 2;

        [StructLayout(LayoutKind.Sequential)]
        internal struct Rect
        {
            public int Left;
            public int Top;
            public int Right;
            public int Bottom;
        }

        [DllImport("user32.dll")]
        internal static extern bool GetWindowRect(IntPtr windowHandle, out Rect rectangle);

        [DllImport("user32.dll")]
        internal static extern uint GetDpiForWindow(IntPtr windowHandle);

        [DllImport("user32.dll")]
        internal static extern IntPtr SendMessage(
            IntPtr windowHandle,
            int message,
            IntPtr wParam,
            IntPtr lParam);

        [DllImport("user32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        internal static extern bool SetWindowPos(
            IntPtr windowHandle,
            IntPtr insertAfter,
            int x,
            int y,
            int width,
            int height,
            uint flags);

        [DllImport("user32.dll", EntryPoint = "GetWindowLong")]
        internal static extern int GetWindowLong(IntPtr windowHandle, int index);

        [DllImport("user32.dll", EntryPoint = "SetWindowLong")]
        internal static extern int SetWindowLong(IntPtr windowHandle, int index, int value);

        [DllImport("user32.dll")]
        internal static extern int GetSystemMetricsForDpi(int index, uint dpi);

        [DllImport("user32.dll")]
        internal static extern bool PrintWindow(IntPtr windowHandle, IntPtr deviceContext, uint flags);

        [DllImport("dwmapi.dll")]
        internal static extern int DwmGetWindowAttribute(IntPtr windowHandle, int attribute,
            out Rect rectangle, int size);

        [DllImport("dwmapi.dll")]
        internal static extern int DwmSetWindowAttribute(IntPtr windowHandle, int attribute,
            ref int value, int size);

        [DllImport("user32.dll")]
        internal static extern bool DestroyIcon(IntPtr iconHandle);
    }
}
