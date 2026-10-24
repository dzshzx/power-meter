using System;
using System.Drawing;
using System.Reflection;
using System.Windows.Forms;
// Deterministic fixture readings. Every reading goes through the production
// PowerSnapshot.Compose and MeterSession; only the final one is bound to the
// window, so a thirty-minute trend costs one render.
public static class UiFixture {
    const BindingFlags All = BindingFlags.Instance | BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic;
    static double Noise(int tick, int salt) {
        unchecked {
            uint x = (uint)(tick * 374761393 + salt * 668265263);
            x = (x ^ (x >> 13)) * 1274126177u;
            return ((x ^ (x >> 16)) & 0xffff) / 65535.0 - 0.5;
        }
    }
    static object Sample(Assembly assembly, string boundary, double watts) {
        Type sample = assembly.GetType("BatteryChargeMeter.PowerSample");
        object kind = Enum.Parse(assembly.GetType("BatteryChargeMeter.MeasurementKind"), "Measured");
        return sample.GetMethod("FromValue", All).Invoke(null, new object[] {
            Enum.Parse(assembly.GetType("BatteryChargeMeter.PowerBoundary"), boundary), kind, watts, "UI acceptance fixture", TimeSpan.FromSeconds(1) });
    }
    static object Missing(Assembly assembly, string boundary, string reason) {
        Type sample = assembly.GetType("BatteryChargeMeter.PowerSample");
        return sample.GetMethod("Unsupported", All).Invoke(null, new object[] {
            Enum.Parse(assembly.GetType("BatteryChargeMeter.PowerBoundary"), boundary), reason });
    }
    // The full, warm-up and partial trends all record a charging laptop.
    static bool Charging(string scenario) {
        return scenario == "charging" || scenario == "early" || scenario == "warming";
    }
    // Platform power: a slow drift, a busy spell peaking near tick 1200 and an
    // N/A spell (ticks 520-639) in the full charging trend.
    static double PlatformWatts(string scenario, int tick, int last) {
        if (tick == last) return 19.39;
        if (Charging(scenario))
            return 18.6 + 1.6 * Math.Sin(tick / 97.0) + 0.9 * Math.Sin(tick / 23.0 + 1) + 0.9 * Noise(tick, 1)
                + 13.5 * Math.Exp(-Math.Pow((tick - 1200) / 38.0, 2)) + (last - tick < 60 ? 0.6 * Math.Sin(tick * 0.43) : 0);
        return 19.39 + Math.Sin(tick * 0.43) * 2.4;
    }
    static double BatteryWatts(string scenario, int tick, int last) {
        if (Charging(scenario)) return tick == last ? 39.08 : 40.2 - 1.2 * tick / 1800.0 + 0.25 * Noise(tick, 2);
        switch (scenario) {
            case "discharging":
                if (tick == last) return -17.50;
                return -(16.4 + 1.1 * Math.Sin(tick / 61.0) + 0.6 * Math.Sin(tick / 13.0) + 0.5 * Noise(tick, 3)
                    + 9.0 * Math.Exp(-Math.Pow((tick - 1080) / 30.0, 2)));
            case "supplemented": return -5.0;
            default: return 0.0;
        }
    }
    public static void Feed(Form form, Assembly assembly, string scenario, int last) {
        object session = form.GetType().GetField("session", All).GetValue(form);
        MethodInfo observe = session.GetType().GetMethod("Observe", All);
        MethodInfo compose = assembly.GetType("BatteryChargeMeter.PowerSnapshot").GetMethod("Compose", All);
        Type readingType = assembly.GetType("BatteryChargeMeter.BatteryReading");
        Type stateType = assembly.GetType("BatteryChargeMeter.BatterySupplyState");
        string state = Charging(scenario) ? "ExternalPowerCharging"
            : scenario == "discharging" ? "BatteryDischarging" : scenario == "supplemented" ? "ExternalPowerSupplemented" : "ExternalPowerIdle";
        DateTimeOffset end = DateTimeOffset.Parse("2026-09-23T14:38:15+08:00");
        object cpu = Sample(assembly, "CpuPackage", 10.1);
        // The elevation-gated reason, so the notice offers the restart action.
        string elevationRequired = (string)assembly.GetType("BatteryChargeMeter.PawnIoSensor").GetField("ElevationRequired", All).GetValue(null);
        for (int tick = 0; tick <= last; tick++) {
            object battery = Activator.CreateInstance(readingType, true);
            double watts = BatteryWatts(scenario, tick, last);
            readingType.GetField("SupplyState").SetValue(battery, Enum.Parse(stateType, state));
            readingType.GetField("PowerWatts").SetValue(battery, watts);
            // The discharge trend has an N/A spell where the firmware rate is missing.
            readingType.GetField("RateAvailable").SetValue(battery, !(scenario == "discharging" && last >= 1800 && tick >= 300 && tick < 400));
            readingType.GetField("VoltageAvailable").SetValue(battery, true);
            readingType.GetField("CurrentAvailable").SetValue(battery, true);
            readingType.GetField("VoltageVolts").SetValue(battery, 17.51);
            readingType.GetField("CurrentAmps").SetValue(battery, watts / 17.51);
            readingType.GetField("Percentage").SetValue(battery, scenario == "discharging" ? 76 : 98);
            object platform = scenario == "unavailable" || (scenario == "charging" && last >= 1800 && tick >= 520 && tick < 640)
                ? Missing(assembly, "Platform", elevationRequired)
                : Sample(assembly, "Platform", PlatformWatts(scenario, tick, last));
            object snapshot = compose.Invoke(null, new object[] { end.AddSeconds(tick - last), (double)tick, battery, cpu, platform });
            if (tick < last) observe.Invoke(session, new object[] { snapshot });
            else form.GetType().GetMethod("ShowReading", All).Invoke(form, new object[] { snapshot });
        }
    }
}
public static class FooterTextProbe {
    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern int GetWindowLong(IntPtr window, int index);
    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern IntPtr SendMessage(IntPtr window, int message, IntPtr wParam, IntPtr lParam);
    public static int HitTest(Form form, int clientX, int clientY) {
        Point screen = form.PointToScreen(new Point(clientX, clientY));
        return SendMessage(form.Handle, 0x84, IntPtr.Zero, new IntPtr((screen.Y << 16) | (screen.X & 0xffff))).ToInt32();
    }
    public static void Maximize(Form form) { SendMessage(form.Handle, 0x112, new IntPtr(0xF030), IntPtr.Zero); }
    public static void AssertReadoutBaseline(Control control) {
        string text = control.Text;
        Color color = control.ForeColor, back = control.BackColor;
        try {
            control.Text = "88.88 W";
            control.ForeColor = Color.Black;
            control.BackColor = Color.White;
            using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
                control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
                int[] bottoms = new int[bitmap.Width];
                for (int x = 0; x < bitmap.Width; x++) {
                    bottoms[x] = -1;
                    for (int y = 0; y < bitmap.Height; y++) {
                        Color pixel = bitmap.GetPixel(x, y);
                        // The unit keeps its quieter grey, so any clearly non-white pixel is ink.
                        if (pixel.R + pixel.G + pixel.B < 600) bottoms[x] = y;
                    }
                }
                int right = bitmap.Width - 1;
                while (right >= 0 && bottoms[right] < 0) right--;
                int unitLeft = right;
                while (unitLeft >= 0 && bottoms[unitLeft] >= 0) unitLeft--;
                int unitBottom = -1, numberBottom = -1;
                for (int x = unitLeft + 1; x <= right; x++) unitBottom = Math.Max(unitBottom, bottoms[x]);
                for (int x = 0; x < unitLeft; x++) numberBottom = Math.Max(numberBottom, bottoms[x]);
                int tolerance = Math.Max(2, (int)Math.Ceiling(1.5 * control.Font.SizeInPoints / 44));
                if (numberBottom < 0 || unitBottom < 0 || Math.Abs(numberBottom - unitBottom) > tolerance)
                    throw new InvalidOperationException("Readout/unit baseline mismatch: " + numberBottom + " / " + unitBottom);
            }
        } finally { control.Text = text; control.ForeColor = color; control.BackColor = back; }
    }
    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);
    public static void AssertHover(Control control) {
        var flags = System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.NonPublic;
        var leave = control.GetType().GetMethod("OnMouseLeave", flags);
        var move = control.GetType().GetMethod("OnMouseMove", flags);
        var selected = control.GetType().GetProperty("SelectedIndex", flags);
        object selection = selected.GetValue(control, null);
        leave.Invoke(control, new object[] { EventArgs.Empty });
        using (Bitmap before = new Bitmap(control.Width, control.Height))
        using (Bitmap hover = new Bitmap(control.Width, control.Height))
        using (Bitmap after = new Bitmap(control.Width, control.Height)) {
            Rectangle bounds = new Rectangle(Point.Empty, before.Size);
            control.DrawToBitmap(before, bounds);
            move.Invoke(control, new object[] { new MouseEventArgs(MouseButtons.None, 0, control.Width * 3 / 4, control.Height / 2, 0) });
            control.DrawToBitmap(hover, bounds);
            leave.Invoke(control, new object[] { EventArgs.Empty });
            control.DrawToBitmap(after, bounds);
            if (DifferingPixels(before, after) != 0)
                throw new InvalidOperationException("Segment hover did not clear on mouse leave.");
            if (DifferingPixels(before, hover) == 0 || !Object.Equals(selection, selected.GetValue(control, null)))
                throw new InvalidOperationException("Segment hover must render feedback without changing the selection.");
        }
    }
    public static void AssertIcon(Control control) {
        int ink = 0;
        using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
            control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
            for (int y = control.Height / 4; y < control.Height * 3 / 4; y++)
                for (int x = control.Width / 4; x < control.Width * 3 / 4; x++) {
                    Color pixel = bitmap.GetPixel(x, y);
                    if (pixel.R + pixel.G + pixel.B > 240) ink++;
                }
        }
        if (ink < 4) throw new InvalidOperationException(control.AccessibleName + " icon did not render.");
    }
    public static void Click(Control control) { ((IButtonControl)control).PerformClick(); }
    public static int DifferingPixels(Bitmap first, Bitmap second) {
        if (first.Size != second.Size) return -1;
        int count = 0;
        for (int y = 0; y < first.Height; y++)
            for (int x = 0; x < first.Width; x++)
                if (first.GetPixel(x, y) != second.GetPixel(x, y)) count++;
        return count;
    }
    public static void Capture(Control control, string path) {
        using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
            control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
            bitmap.Save(path, System.Drawing.Imaging.ImageFormat.Png);
        }
    }
    public static void AssertLayout(Control parent) {
        foreach (Control control in parent.Controls) {
            if (!control.Visible) continue;
            Type type = control.GetType();
            while (type != null && type.Name != "AlignedLabel") type = type.BaseType;
            if (type != null) {
                // Every AlignedLabel-derived control (plain labels, the status
                // label and the readouts) reports its rendered content size,
                // including glyphs, captions and smaller affixes.
                Size required = (Size)type.GetMethod("MeasureContent",
                    System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.NonPublic).Invoke(control, null);
                if (required.Width > control.Width || required.Height > control.Height)
                    throw new InvalidOperationException("Clipped label: " + control.Text + "; required=" + required + "; actual=" + control.Size);
            }
            foreach (Control other in parent.Controls)
                if (other.Visible && other != control && control.Bounds.IntersectsWith(other.Bounds))
                    throw new InvalidOperationException("Overlapping controls: " + control.Text + " " + control.Bounds + " / " + other.Text + " " + other.Bounds);
            if (control.HasChildren) AssertLayout(control);
        }
    }
    public static int InkTop(Control control) {
        string text = control.Text;
        Color back = control.BackColor, fore = control.ForeColor;
        LinkLabel link = control as LinkLabel;
        Color linkColor = link == null ? Color.Empty : link.LinkColor;
        LinkBehavior behavior = link == null ? LinkBehavior.SystemDefault : link.LinkBehavior;
        try {
            control.Text = "Ag09";
            control.BackColor = Color.White;
            control.ForeColor = Color.Black;
            if (link != null) {
                link.LinkColor = Color.Black;
                link.LinkBehavior = LinkBehavior.NeverUnderline;
            }
            using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
                control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
                for (int y = 0; y < bitmap.Height; y++)
                    for (int x = 0; x < bitmap.Width; x++) {
                        Color pixel = bitmap.GetPixel(x, y);
                        if (pixel.R < 96 && pixel.G < 96 && pixel.B < 96) return control.Top + y;
                    }
            }
            throw new InvalidOperationException("Footer text did not render.");
        } finally {
            control.Text = text;
            control.BackColor = back;
            control.ForeColor = fore;
            if (link != null) {
                link.LinkColor = linkColor;
                link.LinkBehavior = behavior;
            }
        }
    }
}
