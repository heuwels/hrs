import HrsGUICore
import SwiftUI

struct InspectorPane: View {
    @Bindable var store: GraphStore

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("Inspector")
                .font(.headline)
                .padding(16)
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    if let strategy = store.selectedStrategy {
                        strategyInspector(strategy)
                    } else if let goal = store.selectedGoal {
                        goalInspector(goal)
                    } else {
                        emptyState
                    }
                }
                .padding(16)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            Divider()
            footer
        }
        .background(Color(nsColor: .controlBackgroundColor))
    }

    private var emptyState: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Select a strategy or a goal.")
                .foregroundStyle(.secondary)
            Text("Drag a tactical goal onto a strategy to draw a link. Drop it in the Unaligned column to clear the link.")
                .font(.callout)
                .foregroundStyle(.secondary)
            stats
        }
    }

    private var stats: some View {
        VStack(alignment: .leading, spacing: 6) {
            labeled("Strategies", "\(store.visibleStrategies.count)")
            labeled("Open goals", "\(store.openCount)")
            labeled("Unaligned", "\(store.unalignedCount)")
        }
        .padding(.top, 8)
    }

    @ViewBuilder
    private func strategyInspector(_ strategy: Strategy) -> some View {
        titleBlock(strategy.title, subtitle: "Strategic · #\(strategy.id)")
        if !strategy.description.isEmpty {
            Text(strategy.description)
                .font(.callout)
        }
        labeled("Status", strategy.status)
        labeled("Priority", priorityLabel(important: strategy.important, urgent: strategy.urgent))
        if let ticket = strategy.ticketRef, !ticket.isEmpty {
            labeled("Ticket", ticket)
        }
        if let report = store.reports[strategy.id] {
            labeled("Goals", "\(report.goalsDone) done / \(report.goalsTotal) total")
            labeled("Hours", String(format: "%.1f", report.totalHours))
        }
        let children = store.visibleGoals.filter { $0.strategyId == strategy.id }
        if children.isEmpty {
            Text("No tactical goals yet. Drag one onto this lane.")
                .font(.callout)
                .foregroundStyle(.secondary)
        } else {
            Text("Tactical goals")
                .font(.subheadline.weight(.semibold))
            ForEach(children) { goal in
                Button {
                    store.select(.goal(goal.id))
                } label: {
                    HStack {
                        Image(systemName: goal.completed ? "checkmark.circle" : "circle")
                        Text(goal.text)
                            .lineLimit(2)
                            .multilineTextAlignment(.leading)
                        Spacer()
                    }
                }
                .buttonStyle(.plain)
            }
        }
    }

    @ViewBuilder
    private func goalInspector(_ goal: Goal) -> some View {
        titleBlock(goal.text, subtitle: "Tactical · #\(goal.id) · \(goal.date)")
        labeled("Status", goal.completed ? "completed" : "open")
        labeled("Priority", priorityLabel(important: goal.important, urgent: goal.urgent))
        if let ticket = goal.ticketRef, !ticket.isEmpty {
            labeled("Ticket", ticket)
        }
        if !goal.entryIds.isEmpty {
            labeled("Entries", goal.entryIds.map(String.init).joined(separator: ", "))
        }

        Text("Aligns to")
            .font(.subheadline.weight(.semibold))
        Picker("Strategy", selection: strategyBinding(for: goal)) {
            Text("Unaligned").tag(Optional<Int64>.none)
            ForEach(store.visibleStrategies) { strategy in
                Text(strategy.title).tag(Optional(strategy.id))
            }
        }
        .labelsHidden()
        .pickerStyle(.menu)

        if goal.isUnaligned {
            Text("This goal does not inform a strategic goal.")
                .font(.callout)
                .foregroundStyle(.orange)
        }

        HStack {
            Button(goal.completed ? "Reopen" : "Mark done") {
                Task { await store.toggleCompleted(goal) }
            }
            if goal.strategyId != nil {
                Button("Unlink") {
                    Task { await store.link(goal.id, to: nil) }
                }
            }
        }
    }

    private var footer: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("hrs serve")
                .font(.caption)
                .foregroundStyle(.secondary)
            TextField("http://127.0.0.1:9746", text: $store.baseURLString)
                .textFieldStyle(.roundedBorder)
                .onSubmit {
                    Task { await store.refresh() }
                }
        }
        .padding(16)
    }

    private func titleBlock(_ title: String, subtitle: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(subtitle)
                .font(.caption)
                .foregroundStyle(.secondary)
            Text(title)
                .font(.title3.weight(.semibold))
                .textSelection(.enabled)
        }
    }

    private func labeled(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top) {
            Text(label)
                .foregroundStyle(.secondary)
                .frame(width: 80, alignment: .leading)
            Text(value)
                .textSelection(.enabled)
            Spacer()
        }
        .font(.callout)
    }

    private func priorityLabel(important: Bool, urgent: Bool) -> String {
        switch (important, urgent) {
        case (true, true): return "important, urgent"
        case (true, false): return "important"
        case (false, true): return "urgent"
        case (false, false): return "none"
        }
    }

    private func strategyBinding(for goal: Goal) -> Binding<Int64?> {
        Binding(
            get: { store.goals.first(where: { $0.id == goal.id })?.strategyId },
            set: { newValue in
                Task { await store.link(goal.id, to: newValue) }
            }
        )
    }
}
