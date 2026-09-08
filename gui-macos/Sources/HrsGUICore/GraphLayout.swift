import CoreGraphics
import Foundation

public struct GraphMetrics: Equatable, Sendable {
    public var padding: CGFloat
    public var strategySize: CGSize
    public var goalSize: CGSize
    public var laneGap: CGFloat
    public var goalGap: CGFloat
    public var edgeGap: CGFloat
    public var columnGap: CGFloat
    public var minHeight: CGFloat

    public init(
        padding: CGFloat = 20,
        strategySize: CGSize = CGSize(width: 300, height: 108),
        goalSize: CGSize = CGSize(width: 260, height: 76),
        laneGap: CGFloat = 16,
        goalGap: CGFloat = 8,
        edgeGap: CGFloat = 88,
        columnGap: CGFloat = 36,
        minHeight: CGFloat = 640
    ) {
        self.padding = padding
        self.strategySize = strategySize
        self.goalSize = goalSize
        self.laneGap = laneGap
        self.goalGap = goalGap
        self.edgeGap = edgeGap
        self.columnGap = columnGap
        self.minHeight = minHeight
    }
}

public enum DropTarget: Equatable, Sendable {
    case strategy(Int64)
    case unaligned
}

public struct GraphLayout: Equatable, Sendable {
    public struct GoalFrame: Equatable, Sendable, Identifiable {
        public var id: Int64 { goal.id }
        public var goal: Goal
        public var rect: CGRect
    }

    public struct Lane: Equatable, Sendable, Identifiable {
        public var id: Int64 { strategy.id }
        public var strategy: Strategy
        public var strategyRect: CGRect
        public var goals: [GoalFrame]
        /// Strategy card plus its tactical column, including the edge gap.
        /// Dropping anywhere in this rect links the dragged goal to this strategy.
        public var rect: CGRect
    }

    public struct Edge: Equatable, Sendable {
        public var goalID: Int64
        public var strategyID: Int64
        public var from: CGPoint
        public var to: CGPoint
    }

    public var size: CGSize
    public var lanes: [Lane]
    public var unaligned: [GoalFrame]
    public var unalignedColumn: CGRect
    public var edges: [Edge]
    public var metrics: GraphMetrics

    public var strategyColumnX: CGFloat { metrics.padding }
    public var tacticalColumnX: CGFloat { metrics.padding + metrics.strategySize.width + metrics.edgeGap }
    public var unalignedColumnX: CGFloat {
        tacticalColumnX + metrics.goalSize.width + metrics.columnGap
    }

    public static func compute(
        strategies: [Strategy],
        goals: [Goal],
        metrics: GraphMetrics = GraphMetrics()
    ) -> GraphLayout {
        let orderedStrategies = strategies.sorted(by: Eisenhower.compareStrategies)
        let strategyIDs = Set(orderedStrategies.map(\.id))
        let grouped = Dictionary(grouping: goals) { goal -> Int64? in
            guard let sid = goal.strategyId, strategyIDs.contains(sid) else { return nil }
            return sid
        }
        let orphans = (grouped[nil] ?? []).sorted(by: Eisenhower.compareGoals)

        var lanes: [Lane] = []
        var edges: [Edge] = []
        var y = metrics.padding
        let xStrategy = metrics.padding
        let xGoal = xStrategy + metrics.strategySize.width + metrics.edgeGap
        let xUnaligned = xGoal + metrics.goalSize.width + metrics.columnGap

        for strategy in orderedStrategies {
            let children = (grouped[strategy.id] ?? []).sorted(by: Eisenhower.compareGoals)
            let stackHeight: CGFloat
            if children.isEmpty {
                stackHeight = metrics.strategySize.height
            } else {
                stackHeight = CGFloat(children.count) * metrics.goalSize.height
                    + CGFloat(children.count - 1) * metrics.goalGap
            }
            let laneHeight = max(metrics.strategySize.height, stackHeight)
            let strategyRect = CGRect(
                x: xStrategy,
                y: y,
                width: metrics.strategySize.width,
                height: laneHeight
            )
            var frames: [GoalFrame] = []
            for (index, goal) in children.enumerated() {
                let gy = y + CGFloat(index) * (metrics.goalSize.height + metrics.goalGap)
                let rect = CGRect(
                    x: xGoal,
                    y: gy,
                    width: metrics.goalSize.width,
                    height: metrics.goalSize.height
                )
                frames.append(GoalFrame(goal: goal, rect: rect))
                edges.append(
                    Edge(
                        goalID: goal.id,
                        strategyID: strategy.id,
                        from: CGPoint(x: strategyRect.maxX, y: rect.midY),
                        to: CGPoint(x: rect.minX, y: rect.midY)
                    )
                )
            }
            let laneRect = CGRect(
                x: xStrategy,
                y: y,
                width: (xGoal + metrics.goalSize.width) - xStrategy,
                height: laneHeight
            )
            lanes.append(
                Lane(
                    strategy: strategy,
                    strategyRect: strategyRect,
                    goals: frames,
                    rect: laneRect
                )
            )
            y += laneHeight + metrics.laneGap
        }

        var unalignedFrames: [GoalFrame] = []
        var uy = metrics.padding
        for goal in orphans {
            let rect = CGRect(
                x: xUnaligned,
                y: uy,
                width: metrics.goalSize.width,
                height: metrics.goalSize.height
            )
            unalignedFrames.append(GoalFrame(goal: goal, rect: rect))
            uy += metrics.goalSize.height + metrics.goalGap
        }

        let lanesBottom = lanes.isEmpty ? metrics.padding : (y - metrics.laneGap)
        let unalignedBottom = unalignedFrames.isEmpty ? metrics.padding : (uy - metrics.goalGap)
        let height = max(lanesBottom, unalignedBottom, metrics.minHeight) + metrics.padding
        let width = xUnaligned + metrics.goalSize.width + metrics.padding
        let unalignedColumn = CGRect(
            x: xUnaligned - metrics.columnGap / 2,
            y: 0,
            width: metrics.goalSize.width + metrics.columnGap,
            height: height
        )

        return GraphLayout(
            size: CGSize(width: width, height: height),
            lanes: lanes,
            unaligned: unalignedFrames,
            unalignedColumn: unalignedColumn,
            edges: edges,
            metrics: metrics
        )
    }

    public func dropTarget(at point: CGPoint) -> DropTarget? {
        if unalignedColumn.contains(point) {
            return .unaligned
        }
        for lane in lanes where lane.rect.contains(point) {
            return .strategy(lane.strategy.id)
        }
        return nil
    }

    public func goalFrame(id: Int64) -> GoalFrame? {
        for lane in lanes {
            if let frame = lane.goals.first(where: { $0.id == id }) {
                return frame
            }
        }
        return unaligned.first(where: { $0.id == id })
    }
}
