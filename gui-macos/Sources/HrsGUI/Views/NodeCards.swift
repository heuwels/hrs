import AppKit
import HrsGUICore
import SwiftUI

struct StrategyCard: View {
    var strategy: Strategy
    var report: StrategyReport?
    var openCount: Int
    var selected: Bool
    var dropHighlighted: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                EisenhowerDots(important: strategy.important, urgent: strategy.urgent)
                Text(strategy.status.uppercased())
                    .font(.caption2.weight(.semibold))
                    .foregroundStyle(statusColor)
                Spacer()
                Text("#\(strategy.id)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.tertiary)
            }
            Text(strategy.title)
                .font(.headline)
                .lineLimit(2)
            if !strategy.description.isEmpty {
                Text(strategy.description)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            Spacer(minLength: 0)
            Text(meta)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding(12)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background {
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .fill(.regularMaterial)
                .overlay(alignment: .leading) {
                    RoundedRectangle(cornerRadius: 2)
                        .fill(statusColor)
                        .frame(width: 4)
                        .padding(.vertical, 10)
                        .padding(.leading, 4)
                }
                .overlay {
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .strokeBorder(borderColor, lineWidth: selected || dropHighlighted ? 2 : 1)
                }
        }
    }

    private var meta: String {
        var parts: [String] = ["\(openCount) open"]
        if let report {
            parts.append("\(report.goalsDone)/\(report.goalsTotal) done")
            if report.totalHours > 0 {
                parts.append(String(format: "%.1fh", report.totalHours))
            }
        }
        if let ticket = strategy.ticketRef, !ticket.isEmpty {
            parts.append(ticket)
        }
        return parts.joined(separator: " · ")
    }

    private var statusColor: Color {
        switch strategy.status {
        case "completed": return .green
        case "archived": return .secondary
        default: return .accentColor
        }
    }

    private var borderColor: Color {
        if dropHighlighted { return .accentColor }
        if selected { return .accentColor }
        return Color(nsColor: .separatorColor)
    }
}

struct GoalCard: View {
    var goal: Goal
    var selected: Bool
    var dimmed: Bool

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            VStack(spacing: 6) {
                Circle()
                    .fill(goal.isUnaligned ? Color.orange : Color.accentColor)
                    .frame(width: 8, height: 8)
                    .padding(.top, 3)
                Spacer(minLength: 0)
            }
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 6) {
                    EisenhowerDots(important: goal.important, urgent: goal.urgent)
                    Text("#\(goal.id)")
                        .font(.caption.monospacedDigit())
                        .foregroundStyle(.tertiary)
                    Text(goal.date)
                        .font(.caption.monospacedDigit())
                        .foregroundStyle(.secondary)
                    Spacer()
                    if goal.completed {
                        Image(systemName: "checkmark.circle.fill")
                            .foregroundStyle(.green)
                            .font(.caption)
                    }
                }
                Text(goal.text)
                    .font(.subheadline.weight(.medium))
                    .lineLimit(2)
                    .strikethrough(goal.completed)
                if goal.isUnaligned {
                    Text("unaligned")
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(.orange)
                }
            }
        }
        .padding(10)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .opacity(dimmed ? 0.4 : (goal.completed ? 0.6 : 1))
        .background {
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .fill(.regularMaterial)
                .overlay {
                    RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .strokeBorder(borderColor, lineWidth: selected ? 2 : 1)
                }
        }
        .onHover { inside in
            if inside {
                NSCursor.openHand.push()
            } else {
                NSCursor.pop()
            }
        }
    }

    private var borderColor: Color {
        if selected { return goal.isUnaligned ? .orange : .accentColor }
        if goal.isUnaligned { return Color.orange.opacity(0.55) }
        return Color(nsColor: .separatorColor)
    }
}

struct EisenhowerDots: View {
    var important: Bool
    var urgent: Bool

    var body: some View {
        HStack(spacing: 3) {
            Circle().fill(important ? Color.red : Color.secondary.opacity(0.25)).frame(width: 6, height: 6)
            Circle().fill(urgent ? Color.orange : Color.secondary.opacity(0.25)).frame(width: 6, height: 6)
        }
        .help(help)
    }

    private var help: String {
        switch (important, urgent) {
        case (true, true): return "Important and urgent"
        case (true, false): return "Important"
        case (false, true): return "Urgent"
        case (false, false): return "Neither important nor urgent"
        }
    }
}
