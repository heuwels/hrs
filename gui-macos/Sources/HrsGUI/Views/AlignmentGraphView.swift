import HrsGUICore
import SwiftUI

struct AlignmentGraphView: View {
    @Bindable var store: GraphStore

    var body: some View {
        let layout = store.layout
        ScrollView([.horizontal, .vertical]) {
            ZStack(alignment: .topLeading) {
                unalignedBackdrop(layout)
                laneHighlights(layout)
                EdgeLayer(
                    layout: layout,
                    selected: store.selected,
                    draggingGoalID: store.draggingGoalID,
                    dragLocation: store.dragLocation
                )
                ForEach(layout.lanes) { lane in
                    StrategyCard(
                        strategy: lane.strategy,
                        report: store.reports[lane.strategy.id],
                        openCount: lane.goals.filter { !$0.goal.completed }.count,
                        selected: store.selected == .strategy(lane.strategy.id),
                        dropHighlighted: dropHighlight(layout) == .strategy(lane.strategy.id)
                    )
                    .frame(width: lane.strategyRect.width, height: lane.strategyRect.height, alignment: .topLeading)
                    .contentShape(Rectangle())
                    .offset(x: lane.strategyRect.minX, y: lane.strategyRect.minY)
                    .opacity(store.isDimmed(strategyID: lane.strategy.id) ? 0.35 : 1)
                    .onTapGesture { store.select(.strategy(lane.strategy.id)) }
                    ForEach(lane.goals) { frame in
                        goalCard(frame, layout: layout)
                    }
                }
                ForEach(layout.unaligned) { frame in
                    goalCard(frame, layout: layout)
                }
            }
            .frame(width: layout.size.width, height: layout.size.height, alignment: .topLeading)
            .coordinateSpace(name: "graph")
        }
        .background(Color(nsColor: .textBackgroundColor).opacity(0.4))
    }

    @ViewBuilder
    private func unalignedBackdrop(_ layout: GraphLayout) -> some View {
        let active = dropHighlight(layout) == .unaligned
        RoundedRectangle(cornerRadius: 16, style: .continuous)
            .fill(Color.orange.opacity(active ? 0.16 : 0.07))
            .overlay {
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .strokeBorder(Color.orange.opacity(active ? 0.7 : 0.25), style: StrokeStyle(lineWidth: 1, dash: [5, 4]))
            }
            .frame(width: layout.unalignedColumn.width, height: max(layout.unalignedColumn.height - 16, 80))
            .offset(x: layout.unalignedColumn.minX, y: 8)
    }

    @ViewBuilder
    private func laneHighlights(_ layout: GraphLayout) -> some View {
        let highlight = dropHighlight(layout)
        ForEach(layout.lanes) { lane in
            let on = highlight == .strategy(lane.strategy.id)
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .fill(Color.accentColor.opacity(on ? 0.12 : 0.03))
                .frame(width: lane.rect.width, height: lane.rect.height)
                .offset(x: lane.rect.minX, y: lane.rect.minY)
        }
    }

    private func goalCard(_ frame: GraphLayout.GoalFrame, layout: GraphLayout) -> some View {
        GoalCard(
            goal: frame.goal,
            selected: store.selected == .goal(frame.goal.id),
            dimmed: store.isDimmed(goal: frame.goal)
        )
        .frame(width: frame.rect.width, height: frame.rect.height, alignment: .topLeading)
        .contentShape(Rectangle())
        .offset(x: frame.rect.minX, y: frame.rect.minY)
        .opacity(store.draggingGoalID == frame.goal.id ? 0.55 : 1)
        .onTapGesture { store.select(.goal(frame.goal.id)) }
        .gesture(dragGesture(goalID: frame.goal.id))
        .contextMenu {
            Button(frame.goal.completed ? "Reopen" : "Mark done") {
                Task { await store.toggleCompleted(frame.goal) }
            }
            if frame.goal.strategyId != nil {
                Button("Unlink") {
                    Task { await store.link(frame.goal.id, to: nil) }
                }
            }
            Menu("Align to") {
                Button("Unaligned") {
                    Task { await store.link(frame.goal.id, to: nil) }
                }
                Divider()
                ForEach(store.visibleStrategies) { strategy in
                    Button(strategy.title) {
                        Task { await store.link(frame.goal.id, to: strategy.id) }
                    }
                }
            }
        }
    }

    private func dragGesture(goalID: Int64) -> some Gesture {
        DragGesture(minimumDistance: 6, coordinateSpace: .named("graph"))
            .onChanged { value in
                store.select(.goal(goalID))
                store.draggingGoalID = goalID
                store.dragLocation = value.location
            }
            .onEnded { value in
                Task { await store.drop(at: value.location) }
            }
    }

    private func dropHighlight(_ layout: GraphLayout) -> DropTarget? {
        guard store.draggingGoalID != nil, let point = store.dragLocation else { return nil }
        return layout.dropTarget(at: point)
    }
}

private struct EdgeLayer: View {
    var layout: GraphLayout
    var selected: NodeID?
    var draggingGoalID: Int64?
    var dragLocation: CGPoint?

    var body: some View {
        Canvas { context, _ in
            for edge in layout.edges {
                let focused = isFocused(edge)
                var path = Path()
                path.move(to: edge.from)
                let dx = max(36, abs(edge.to.x - edge.from.x) * 0.45)
                path.addCurve(
                    to: edge.to,
                    control1: CGPoint(x: edge.from.x + dx, y: edge.from.y),
                    control2: CGPoint(x: edge.to.x - dx, y: edge.to.y)
                )
                context.stroke(
                    path,
                    with: .color(.accentColor.opacity(focused ? 0.9 : 0.28)),
                    style: StrokeStyle(lineWidth: focused ? 2.2 : 1.4, lineCap: .round)
                )
                let port = Path(ellipseIn: CGRect(x: edge.from.x - 3, y: edge.from.y - 3, width: 6, height: 6))
                context.fill(port, with: .color(.accentColor.opacity(focused ? 1 : 0.45)))
            }
            if let gid = draggingGoalID, let loc = dragLocation, let frame = layout.goalFrame(id: gid) {
                let from = CGPoint(x: frame.rect.minX, y: frame.rect.midY)
                var path = Path()
                path.move(to: from)
                let dx = max(36, abs(loc.x - from.x) * 0.45)
                path.addCurve(
                    to: loc,
                    control1: CGPoint(x: from.x - dx, y: from.y),
                    control2: CGPoint(x: loc.x + dx, y: loc.y)
                )
                context.stroke(
                    path,
                    with: .color(.orange),
                    style: StrokeStyle(lineWidth: 2, lineCap: .round, dash: [6, 4])
                )
            }
        }
        .frame(width: layout.size.width, height: layout.size.height)
        .allowsHitTesting(false)
    }

    private func isFocused(_ edge: GraphLayout.Edge) -> Bool {
        switch selected {
        case .strategy(let id):
            return edge.strategyID == id
        case .goal(let id):
            return edge.goalID == id
        case nil:
            return true
        }
    }
}
