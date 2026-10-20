using System.Drawing;
using Xunit;
using static BatteryChargeMeter.Tests.Fixtures;

namespace BatteryChargeMeter.Tests
{
    public sealed class DpiTransitionTests
    {
        // A 1080p screen with a 40-pixel taskbar, as on the CI runner.
        private static readonly Rectangle Area = new Rectangle(0, 0, 1920, 1040);

        [Fact]
        public void SuggestionsStayInsideTheWorkingArea()
        {
            // 168 DPI: 679 x 994; the diagnostic later grows the height up to
            // the margin DpiLayout keeps (1024) without moving the window.
            Size collapsed = new Size(679, 994);
            int expanded = Area.Height - DpiLayout.WorkingAreaMargin;
            Point first = MainForm.DpiTransitionPosition(Area, collapsed, Point.Empty);
            Point second = MainForm.DpiTransitionPosition(Area, collapsed, first);
            foreach (Point position in new Point[] { first, second })
                Check("a suggestion leaves room for the expanded notice: " + position,
                    Area.Contains(new Rectangle(position, new Size(collapsed.Width, expanded))));

            Rectangle secondary = new Rectangle(1920, -200, 1280, 760);
            Point offset = MainForm.DpiTransitionPosition(secondary, new Size(400, 600), Point.Empty);
            Check("a suggestion on a secondary monitor stays on it",
                secondary.Contains(new Rectangle(offset, new Size(400, 600 + DpiLayout.WorkingAreaMargin))));

            Point oversized = MainForm.DpiTransitionPosition(Area, new Size(2000, 1200), Point.Empty);
            Check("a window larger than the area starts at its corner", oversized == Area.Location);
        }

        [Fact]
        public void SuggestionsAlternateWithoutDrift()
        {
            Size size = new Size(679, 994);
            Point first = MainForm.DpiTransitionPosition(Area, size, Point.Empty);
            Point second = MainForm.DpiTransitionPosition(Area, size, first);
            Point third = MainForm.DpiTransitionPosition(Area, size, second);
            Check("each transition moves the window and the third returns to the first",
                first != second && third == first);
            Check("the preview window's start (+20) is neither spot, so following is observable",
                first != new Point(20, 20) && second != new Point(20, 20));

            // Height fills the area: one axis is clamped, the other still alternates.
            Size tall = new Size(679, Area.Height);
            Point a = MainForm.DpiTransitionPosition(Area, tall, Point.Empty);
            Point b = MainForm.DpiTransitionPosition(Area, tall, a);
            Check("a clamped axis does not stop the alternation", a != b
                && MainForm.DpiTransitionPosition(Area, tall, b) == a);
        }
    }
}
