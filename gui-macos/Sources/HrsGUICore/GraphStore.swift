import CoreGraphics
import Foundation
import Observation

@Observable
@MainActor
public final class GraphStore {
    public var strategies: [Strategy] = []
    public var goals: [Goal] = []
    public var reports: [Int64: StrategyReport] = [:]
    public var connected: Bool = false
    public var loading: Bool = false
    public var errorMessage: String?
    public var showCompletedStrategies: Bool = false
    public var showCompletedGoals: Bool = false
    public var query: String = ""
    public var selected: NodeID?
    public var draggingGoalID: Int64?
    public var dragLocation: CGPoint?
    public var baseURLString: String {
        didSet {
            UserDefaults.standard.set(baseURLString, forKey: Self.baseURLKey)
            client.baseURL = resolvedBaseURL
        }
    }

    public var client: HrsClient

    public static let baseURLKey = "hrs.baseURL"

    public init(client: HrsClient = HrsClient(baseURL: HrsClient.defaultBaseURL)) {
        if let env = ProcessInfo.processInfo.environment["HRS_URL"],
           let url = URL(string: env), url.scheme != nil {
            self.baseURLString = env
            self.client = HrsClient(baseURL: url, session: client.session)
            return
        }
        if let stored = UserDefaults.standard.string(forKey: Self.baseURLKey), !stored.isEmpty {
            self.baseURLString = stored
            self.client = HrsClient(baseURL: URL(string: stored) ?? client.baseURL, session: client.session)
        } else {
            self.baseURLString = client.baseURL.absoluteString
            self.client = client
        }
    }

    public var resolvedBaseURL: URL {
        URL(string: baseURLString) ?? HrsClient.defaultBaseURL
    }

    public var visibleGoals: [Goal] {
        GraphFilter.visibleGoals(goals, includeCompleted: showCompletedGoals)
    }

    public var visibleStrategies: [Strategy] {
        GraphFilter.visibleStrategies(
            strategies,
            goals: visibleGoals,
            includeCompleted: showCompletedStrategies
        )
    }

    public var layout: GraphLayout {
        GraphLayout.compute(strategies: visibleStrategies, goals: visibleGoals)
    }

    public var openCount: Int { goals.filter { !$0.completed }.count }
    public var unalignedOpen: [Goal] { goals.filter { !$0.completed && $0.strategyId == nil } }
    public var unalignedCount: Int { unalignedOpen.count }

    public var selectedStrategy: Strategy? {
        guard case .strategy(let id) = selected else { return nil }
        return strategies.first { $0.id == id }
    }

    public var selectedGoal: Goal? {
        guard case .goal(let id) = selected else { return nil }
        return goals.first { $0.id == id }
    }

    public func matchesSearch(_ text: String) -> Bool {
        let q = query.trimmingCharacters(in: .whitespacesAndNewlines)
        if q.isEmpty { return true }
        return text.localizedCaseInsensitiveContains(q)
    }

    public func isDimmed(strategyID: Int64) -> Bool {
        let q = query.trimmingCharacters(in: .whitespacesAndNewlines)
        if q.isEmpty { return false }
        if let strategy = strategies.first(where: { $0.id == strategyID }),
           matchesSearch(strategy.title) || matchesSearch(strategy.description) {
            return false
        }
        return !visibleGoals.contains { goal in
            goal.strategyId == strategyID && matchesSearch(goal.text)
        }
    }

    public func isDimmed(goal: Goal) -> Bool {
        let q = query.trimmingCharacters(in: .whitespacesAndNewlines)
        if q.isEmpty { return false }
        if matchesSearch(goal.text) { return false }
        if let sid = goal.strategyId,
           let strategy = strategies.first(where: { $0.id == sid }),
           matchesSearch(strategy.title) || matchesSearch(strategy.description) {
            return false
        }
        return true
    }

    public func refresh() async {
        loading = true
        errorMessage = nil
        client.baseURL = resolvedBaseURL
        do {
            connected = try await client.health()
            async let strategyList = client.strategies()
            async let goalList: [Goal] = showCompletedGoals ? client.allGoals() : client.activeGoals()
            strategies = try await strategyList
            goals = try await goalList
            await loadReports()
            pruneSelection()
        } catch {
            connected = false
            errorMessage = error.localizedDescription
        }
        loading = false
    }

    public func link(_ goalID: Int64, to strategyID: Int64?) async {
        guard let index = goals.firstIndex(where: { $0.id == goalID }) else { return }
        let previous = goals[index].strategyId
        if previous == strategyID { return }
        goals[index].strategyId = strategyID
        do {
            let updated = try await client.setStrategy(goalID: goalID, strategyID: strategyID)
            goals[index] = updated
            if strategyID == nil, updated.strategyId != nil {
                errorMessage = "hrs serve is too old to unlink. Rebuild and restart `hrs serve`."
                await refresh()
            }
        } catch {
            goals[index].strategyId = previous
            errorMessage = error.localizedDescription
        }
    }

    public func drop(at point: CGPoint) async {
        guard let goalID = draggingGoalID else { return }
        let target = layout.dropTarget(at: point)
        draggingGoalID = nil
        dragLocation = nil
        switch target {
        case .strategy(let id):
            await link(goalID, to: id)
        case .unaligned:
            await link(goalID, to: nil)
        case nil:
            break
        }
    }

    public func toggleCompleted(_ goal: Goal) async {
        do {
            if goal.completed {
                try await client.undoGoal(goal.id)
            } else {
                try await client.completeGoal(goal.id)
            }
            await refresh()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    public func setStrategyStatus(_ id: Int64, status: String) async {
        guard let index = strategies.firstIndex(where: { $0.id == id }) else { return }
        let previous = strategies[index].status
        if previous == status { return }
        strategies[index].status = status
        do {
            try await client.setStrategyStatus(id, status: status)
            await refresh()
            if case .strategy(let selectedID) = selected,
               selectedID == id,
               !visibleStrategies.contains(where: { $0.id == id }) {
                selected = nil
            }
        } catch {
            strategies[index].status = previous
            errorMessage = error.localizedDescription
        }
    }

    public func select(_ node: NodeID?) {
        selected = node
    }

    private func loadReports() async {
        var next: [Int64: StrategyReport] = [:]
        await withTaskGroup(of: (Int64, StrategyReport?).self) { group in
            for strategy in strategies {
                group.addTask { [client] in
                    let report = try? await client.strategyReport(strategy.id)
                    return (strategy.id, report)
                }
            }
            for await (id, report) in group {
                if let report {
                    next[id] = report
                }
            }
        }
        reports = next
    }

    private func pruneSelection() {
        switch selected {
        case .strategy(let id):
            if !strategies.contains(where: { $0.id == id }) { selected = nil }
        case .goal(let id):
            if !goals.contains(where: { $0.id == id }) { selected = nil }
        case nil:
            break
        }
    }
}
