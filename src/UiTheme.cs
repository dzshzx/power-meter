using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Drawing2D;

namespace BatteryChargeMeter
{
    /// <summary>
    /// Single dark palette for the instrument window and the settings popover.
    /// Semantic hues are reserved for the supply state and errors; everything
    /// structural is neutral. The tray badge keeps its own solid colours.
    /// </summary>
    internal static class UiTheme
    {
        internal const string TextFont = "Microsoft YaHei UI";

        // Surfaces.
        internal static readonly Color Canvas = Color.FromArgb(27, 30, 31);
        internal static readonly Color FooterBand = Color.FromArgb(23, 25, 26);
        internal static readonly Color Popover = Color.FromArgb(39, 43, 45);
        // 7% white over the canvas.
        internal static readonly Color Hairline = Color.FromArgb(43, 46, 47);
        internal static readonly Color SegmentTrack = Color.FromArgb(38, 41, 42);
        internal static readonly Color SegmentOn = Color.FromArgb(52, 55, 56);
        internal static readonly Color SegmentHover = Color.FromArgb(44, 47, 48);
        internal static readonly Color ButtonHover = Color.FromArgb(44, 48, 49);
        internal static readonly Color ButtonActive = Color.FromArgb(52, 56, 57);
        internal static readonly Color CloseHover = Color.FromArgb(196, 43, 28);
        internal static readonly Color ClosePressed = Color.FromArgb(160, 36, 24);
        internal static readonly Color ActionFill = Color.FromArgb(45, 48, 49);
        internal static readonly Color ActionFillHover = Color.FromArgb(56, 60, 61);
        internal static readonly Color PopoverAction = Color.FromArgb(58, 62, 63);
        internal static readonly Color PopoverActionHover = Color.FromArgb(68, 72, 73);
        // Outline of the settings popover in composed previews.
        internal static readonly Color PopoverBorder = Color.FromArgb(58, 62, 64);
        // One ring of the popover's soft shadow in composed previews.
        internal static readonly Color PopoverShadow = Color.FromArgb(18, 0, 0, 0);
        internal static readonly Color Field = Color.FromArgb(52, 56, 57);
        internal static readonly Color FieldBorder = Color.FromArgb(66, 70, 71);
        internal static readonly Color GridLine = Color.FromArgb(36, 39, 40);
        internal static readonly Color CellTrack = Color.FromArgb(50, 53, 54);
        // Trend chart: value grid lines and minute ticks below the axis.
        internal static readonly Color TrendGrid = Color.FromArgb(38, 41, 42);
        internal static readonly Color TrendTick = Color.FromArgb(52, 55, 56);
        // Dotted leaders between a supporting metric and its value.
        internal static readonly Color Leader = Color.FromArgb(70, 74, 75);

        // Text.
        internal static readonly Color Ink = Color.FromArgb(236, 238, 238);
        internal static readonly Color Muted = Color.FromArgb(154, 161, 163);
        internal static readonly Color Faint = Color.FromArgb(100, 107, 110);
        internal static readonly Color NumericMuted = Color.FromArgb(212, 216, 217);

        // Supply-state hues tuned for the dark surface.
        internal static readonly Color Charging = Color.FromArgb(76, 196, 143);
        internal static readonly Color Discharging = Color.FromArgb(242, 165, 74);
        internal static readonly Color ErrorRed = Color.FromArgb(242, 122, 111);
        internal static readonly Color Idle = Color.FromArgb(185, 191, 193);
        internal static readonly Color AppTile = Color.FromArgb(5, 150, 105);
        internal static readonly Color ToggleKnob = Color.FromArgb(16, 36, 26);

        // Tray tiles keep the original solid colours on any taskbar theme.
        internal static readonly Color TrayCharging = Color.FromArgb(5, 150, 105);
        internal static readonly Color TrayDischarging = Color.FromArgb(217, 119, 6);
        internal static readonly Color TrayNeutral = Color.FromArgb(107, 114, 128);

        private static readonly Dictionary<string, bool> installed = new Dictionary<string, bool>();

        /// <summary>
        /// Bahnschrift named instance for figures, falling back to Segoe UI
        /// where the variable font is absent.
        /// </summary>
        internal static string NumericFamily(string instance)
        {
            bool present;
            if (!installed.TryGetValue(instance, out present))
            {
                using (Font probe = new Font(instance, 10f))
                    present = String.Equals(probe.Name, instance, StringComparison.OrdinalIgnoreCase);
                installed[instance] = present;
            }
            return present ? instance : "Segoe UI";
        }

        internal static Font Numeric(string instance, float points)
        {
            return new Font(NumericFamily(instance), points, FontStyle.Regular, GraphicsUnit.Point);
        }

        internal static Font Text(float points, FontStyle style)
        {
            return new Font(TextFont, points, style, GraphicsUnit.Point);
        }

        /// <summary>
        /// Accent for readings and state text. Idle keeps a neutral light
        /// grey: with no battery flow there is nothing to signal.
        /// </summary>
        internal static Color Accent(BatteryAccentKind kind)
        {
            switch (kind)
            {
                case BatteryAccentKind.Charging:
                    return Charging;
                case BatteryAccentKind.Discharging:
                    return Discharging;
                case BatteryAccentKind.Error:
                    return ErrorRed;
                default:
                    return Idle;
            }
        }

        internal static Color TrayTile(BatteryAccentKind kind)
        {
            switch (kind)
            {
                case BatteryAccentKind.Charging:
                    return TrayCharging;
                case BatteryAccentKind.Discharging:
                    return TrayDischarging;
                default:
                    return TrayNeutral;
            }
        }

        internal static Color Tone(ReadoutTone tone, BatteryAccentKind accent)
        {
            switch (tone)
            {
                case ReadoutTone.Muted:
                    return Muted;
                case ReadoutTone.Unavailable:
                    return Faint;
                case ReadoutTone.NumericMuted:
                    return NumericMuted;
                case ReadoutTone.Accent:
                    return Accent(accent);
                default:
                    return Ink;
            }
        }

        internal static Color Alpha(Color color, int alpha)
        {
            return Color.FromArgb(Math.Max(0, Math.Min(255, alpha)), color.R, color.G, color.B);
        }

        internal static string Hex(Color color)
        {
            return "#" + color.R.ToString("x2") + color.G.ToString("x2") + color.B.ToString("x2");
        }
    }

    internal static class WidgetPath
    {
        internal static GraphicsPath Rounded(Rectangle bounds, int radius)
        {
            return Rounded(new RectangleF(bounds.X, bounds.Y, bounds.Width, bounds.Height), radius);
        }

        internal static GraphicsPath Rounded(RectangleF bounds, float radius)
        {
            GraphicsPath path = new GraphicsPath();
            if (radius <= 0)
            {
                path.AddRectangle(bounds);
                return path;
            }
            float diameter = Math.Min(radius * 2, Math.Min(bounds.Width, bounds.Height));
            path.AddArc(bounds.X, bounds.Y, diameter, diameter, 180, 90);
            path.AddArc(bounds.Right - diameter, bounds.Y, diameter, diameter, 270, 90);
            path.AddArc(bounds.Right - diameter, bounds.Bottom - diameter, diameter, diameter, 0, 90);
            path.AddArc(bounds.X, bounds.Bottom - diameter, diameter, diameter, 90, 90);
            path.CloseFigure();
            return path;
        }
    }
}
