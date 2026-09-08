import CoreGraphics
import Foundation
import HrsGUICore
import Testing

struct GraphLayoutTests {
    private let iso = Strategy(id: 5, title: "ISO 27001 Certification", important: true, urgent: true)
    private let kh = Strategy(id: 6, title: "KH CLI")
    private let archived = Strategy(id: 8, title: "Old", status: "completed")

    @Test func lanesGroupLinkedGoalsAndCollectOrphans() {
        let goals = [
            Goal(id: 1, date: "2026-09-08", text: "Write evidence", strategyId: 5),
            Goal(id: 2, date: "2026-09-08", text: "PAT form", strategyId: 6),
            Goal(id: 3, date: "2026-09-08", text: "Casio setup"),
        ]
        let layout = GraphLayout.compute(strategies: [iso, kh], goals: goals)

        #expect(layout.lanes.count == 2)
        #expect(layout.lanes[0].strategy.id == 5)
        #expect(layout.lanes[0].goals.map(\.id) == [1])
        #expect(layout.lanes[1].goals.map(\.id) == [2])
        #expect(layout.unaligned.map(\.id) == [3])
        #expect(layout.edges.count == 2)
    }

    @Test func unknownStrategyIdCountsAsUnaligned() {
        let goals = [Goal(id: 9, date: "2026-09-08", text: "Dangling", strategyId: 99)]
        let layout = GraphLayout.compute(strategies: [iso], goals: goals)
        #expect(layout.unaligned.map(\.id) == [9])
        #expect(layout.edges.isEmpty)
    }

    @Test func edgesAreHorizontalFromStrategyToGoal() throws {
        let goals = [Goal(id: 1, date: "2026-09-08", text: "Write evidence", strategyId: 5)]
        let layout = GraphLayout.compute(strategies: [iso], goals: goals)
        let edge = try #require(layout.edges.first)
        let frame = try #require(layout.lanes.first?.goals.first)
        #expect(edge.from.x == layout.lanes[0].strategyRect.maxX)
        #expect(edge.to.x == frame.rect.minX)
        #expect(edge.from.y == frame.rect.midY)
        #expect(edge.to.y == frame.rect.midY)
    }

    @Test func dropOnLaneLinksAndDropOnUnalignedUnlinks() {
        let goals = [
            Goal(id: 1, date: "2026-09-08", text: "Write evidence", strategyId: 5),
            Goal(id: 3, date: "2026-09-08", text: "Casio setup"),
        ]
        let layout = GraphLayout.compute(strategies: [iso, kh], goals: goals)
        let isoLane = layout.lanes[0].rect
        let khLane = layout.lanes[1].rect

        #expect(layout.dropTarget(at: CGPoint(x: isoLane.midX, y: isoLane.midY)) == .strategy(5))
        #expect(layout.dropTarget(at: CGPoint(x: khLane.midX, y: khLane.midY)) == .strategy(6))
        #expect(layout.dropTarget(at: CGPoint(x: layout.unalignedColumn.midX, y: 40)) == .unaligned)
        #expect(layout.dropTarget(at: CGPoint(x: 0, y: 0)) == nil)
    }

    @Test func emptyStrategiesStillReserveAnUnalignedColumn() {
        let layout = GraphLayout.compute(strategies: [], goals: [
            Goal(id: 3, date: "2026-09-08", text: "Casio setup"),
        ])
        #expect(layout.lanes.isEmpty)
        #expect(layout.unaligned.count == 1)
        #expect(layout.size.height >= GraphMetrics().minHeight)
        #expect(layout.dropTarget(at: CGPoint(x: layout.unalignedColumn.midX, y: 20)) == .unaligned)
    }

    @Test func importantStrategiesSortFirst() {
        let layout = GraphLayout.compute(strategies: [kh, iso], goals: [])
        #expect(layout.lanes.map(\.strategy.id) == [5, 6])
    }
}

struct GraphFilterTests {
    @Test func hidingCompletedKeepsReferencedStrategies() {
        let active = Strategy(id: 5, title: "ISO", status: "active")
        let done = Strategy(id: 8, title: "Old", status: "completed")
        let unusedDone = Strategy(id: 9, title: "Spare", status: "completed")
        let goals = [
            Goal(id: 1, date: "2026-09-08", text: "Still going", strategyId: 8),
            Goal(id: 2, date: "2026-09-01", text: "Finished", completed: true, strategyId: 5),
        ]
        let visibleGoals = GraphFilter.visibleGoals(goals, includeCompleted: false)
        #expect(visibleGoals.map(\.id) == [1])

        let visible = GraphFilter.visibleStrategies(
            [active, done, unusedDone],
            goals: visibleGoals,
            includeCompleted: false
        )
        #expect(Set(visible.map(\.id)) == [5, 8])
    }

    @Test func completedGoalsStayHiddenUntilAsked() {
        let goals = [
            Goal(id: 1, date: "2026-09-08", text: "Open"),
            Goal(id: 2, date: "2026-09-07", text: "Done", completed: true),
        ]
        #expect(GraphFilter.visibleGoals(goals, includeCompleted: false).map(\.id) == [1])
        #expect(GraphFilter.visibleGoals(goals, includeCompleted: true).map(\.id) == [1, 2])
    }
}

struct HrsClientTests {
    @Test func endpointJoinsPathAndQuery() throws {
        let base = URL(string: "http://127.0.0.1:9746")!
        #expect(try HrsClient.endpoint(base: base, path: "goals/active").absoluteString == "http://127.0.0.1:9746/goals/active")
        #expect(try HrsClient.endpoint(base: base, path: "goals?all=1").absoluteString == "http://127.0.0.1:9746/goals?all=1")
        #expect(try HrsClient.endpoint(base: base, path: "/strategies/5").absoluteString == "http://127.0.0.1:9746/strategies/5")
    }

    @Test func strategyPatchEncodesNullAndNumber() throws {
        struct Patch: Decodable {
            var strategyId: Int64
            enum CodingKeys: String, CodingKey { case strategyId = "strategy_id" }
        }
        let linked = HrsClient.encodeStrategyPatch(strategyID: 26)
        let patch = try JSONDecoder().decode(Patch.self, from: linked)
        #expect(patch.strategyId == 26)
        let unlinked = HrsClient.encodeStrategyPatch(strategyID: nil)
        #expect(String(data: unlinked, encoding: .utf8) == #"{"strategy_id":null}"#)
    }

    @Test func statusPatchEncodesCompleted() throws {
        struct Patch: Decodable { var status: String }
        let data = HrsClient.encodeStatusPatch(status: "completed")
        let patch = try JSONDecoder().decode(Patch.self, from: data)
        #expect(patch.status == "completed")
    }

    @Test func goalDecoderTreatsMissingStrategyAsUnaligned() throws {
        let json = Data(#"{"id":3,"date":"2026-09-08","text":"Casio setup","completed":false}"#.utf8)
        let goal = try JSONDecoder().decode(Goal.self, from: json)
        #expect(goal.isUnaligned)
        #expect(goal.entryIds.isEmpty)
    }
}
