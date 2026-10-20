using System;
using System.Drawing;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    /// <summary>
    /// Source details render the same view as the selected headline, as three
    /// supporting rows; the selected fourth value is the headline.
    /// </summary>
    internal sealed partial class MainForm
    {
        private readonly PowerSources powerSources = new PowerSources();
        private readonly ToolTip sourceTip = new ToolTip();

        private readonly Label[] sourceNames = new Label[4];
        private readonly ReadoutLabel[] sourceValues = new ReadoutLabel[4];

        // A dotted leader from a row's caption to its value, in content
        // coordinates (the painter applies the scroll offset).
        private struct Leader
        {
            public int From;
            public int To;
            public int Y;
        }

        // Measured when a row's text or layout changes, so painting (which
        // the transparent labels also trigger) only draws.
        private readonly Leader[] leaders = new Leader[4];

        /// <summary>
        /// Builds the source rows. Called from the constructor before
        /// <see cref="DpiLayout"/> captures the design bounds, so these controls
        /// scale with everything else.
        /// </summary>
        private void BuildPowerSourceRows()
        {
            AddSourceRow(MeterView.BatteryRow, PowerSample.LabelFor(PowerBoundary.BatteryTerminal));
            AddSourceRow(MeterView.CpuPackageRow, PowerSample.LabelFor(PowerBoundary.CpuPackage));
            AddSourceRow(MeterView.PlatformRow, PowerSample.LabelFor(PowerBoundary.Platform));

            // The last row answers the whole-machine question, which is a
            // different boundary on battery than on external power, so its
            // caption is rewritten each tick from the view.
            AddSourceRow(MeterView.WholeSystemRow, PowerSample.LabelFor(PowerBoundary.EstimatedSystemInput));
        }

        private void AddSourceRow(int index, string caption)
        {
            int y = RowsTop + index * RowHeight;

            Label nameLabel = NewLabel(caption, 18, y, 200, RowHeight, 9.75f, FontStyle.Regular);
            nameLabel.ForeColor = UiTheme.Muted;
            Controls.Add(nameLabel);

            ReadoutLabel value = Readout(222, y, 148, RowHeight, "Bahnschrift", 11.25f, true);
            value.Text = "--";
            value.UnitScale = 12f / 15f;
            value.TextAlign = ContentAlignment.MiddleRight;
            sourceNames[index] = nameLabel;
            sourceValues[index] = value;
        }

        private void LayoutSourceRows()
        {
            if (sourceNames[3] == null) return;
            float scale = LayoutScale;
            int selected = session.Mode == DisplayMode.Battery ? MeterView.BatteryRow : MeterView.WholeSystemRow;
            int row = 0;
            for (int index = 0; index < sourceNames.Length; index++)
            {
                bool visible = index != selected;
                sourceNames[index].Visible = visible;
                sourceValues[index].Visible = visible;
                if (!visible) continue;
                int y = RowsTop + row++ * RowHeight;
                Point offset = AutoScrollPosition;
                Rectangle name = DpiLayout.ScaleBounds(new Rectangle(18, y, 200, RowHeight), scale);
                Rectangle value = DpiLayout.ScaleBounds(new Rectangle(222, y, 148, RowHeight), scale);
                name.Offset(offset);
                value.Offset(offset);
                sourceNames[index].Bounds = name;
                sourceValues[index].Bounds = value;
            }
            UpdateLeaders();
        }

        private void UpdateLeaders()
        {
            // Measuring needs a device context; OnHandleCreated lays out again.
            if (!IsHandleCreated) return;
            float scale = LayoutScale;
            Point scroll = AutoScrollPosition;
            using (Graphics g = CreateGraphics())
            {
                for (int index = 0; index < sourceNames.Length; index++)
                {
                    Label name = sourceNames[index];
                    ReadoutLabel value = sourceValues[index];
                    Leader leader = new Leader();
                    if (name.Visible)
                    {
                        leader.From = name.Left - scroll.X
                            + TextRenderer.MeasureText(name.Text, name.Font, Size.Empty, AlignedLabel.Flags).Width
                            + DpiLayout.ScaleValue(10, scale);
                        leader.To = value.Right - scroll.X - value.Measure(g).Width - DpiLayout.ScaleValue(10, scale);
                        leader.Y = name.Top - scroll.Y + name.Height / 2 + DpiLayout.ScaleValue(4, scale);
                    }
                    if (leader.From == leaders[index].From && leader.To == leaders[index].To && leader.Y == leaders[index].Y)
                        continue;
                    InvalidateLeader(leaders[index], scale);
                    leaders[index] = leader;
                    InvalidateLeader(leader, scale);
                }
            }
        }

        // The leader runs under the transparent caption and value labels,
        // which paint it through this form, so they are invalidated too.
        private void InvalidateLeader(Leader leader, float scale)
        {
            if (leader.To <= leader.From) return;
            int pen = Math.Max(1, (int)Math.Ceiling(scale));
            Rectangle area = Rectangle.FromLTRB(leader.From - pen, leader.Y - pen, leader.To + pen, leader.Y + pen + 1);
            area.Offset(AutoScrollPosition);
            Invalidate(area, true);
        }

        private void UpdatePowerSources(MeterView view)
        {
            for (int index = 0; index < sourceValues.Length; index++)
                Apply(sourceValues[index], view.Rows[index], view.Accent);
            sourceNames[MeterView.WholeSystemRow].Text = view.WholeCaption;
            sourceTip.SetToolTip(sourceNames[MeterView.WholeSystemRow], view.WholeCaptionTooltip);
            UpdateDiagnostics(view);
            LayoutSourceRows();
        }

        private void RefreshDiagnostics()
        {
            UpdateDiagnostics(lastView ?? session.Render());
        }

        // Elevation and startup notices lead the measurement reasons.
        private void UpdateDiagnostics(MeterView view)
        {
            System.Text.StringBuilder reasons = new System.Text.StringBuilder();
            if (!String.IsNullOrEmpty(elevationMessage))
                reasons.AppendLine(Strings.Diagnostic(elevationMessage));
            if (!String.IsNullOrEmpty(autostartStateMessage))
                reasons.AppendLine(Strings.Diagnostic(autostartStateMessage));
            else if (!String.IsNullOrEmpty(autostartMessage))
                reasons.AppendLine(Strings.Diagnostic(autostartMessage));
            foreach (string line in view.Diagnostics)
                reasons.AppendLine(line);
            errorLabel.Text = reasons.ToString().TrimEnd();
            UpdateElevateAction();
        }

        private void Apply(ReadoutLabel target, ReadoutText row, BatteryAccentKind accent)
        {
            target.Text = row.Text;
            target.ForeColor = UiTheme.Tone(row.Tone, accent);
            sourceTip.SetToolTip(target, row.Tooltip);
        }

        private void DisposePowerSources()
        {
            if (powerSources != null)
                powerSources.Dispose();
            if (sourceTip != null)
                sourceTip.Dispose();
        }
    }
}
