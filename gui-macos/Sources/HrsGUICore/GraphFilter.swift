import Foundation

public enum GraphFilter {
    public static func visibleGoals(_ goals: [Goal], includeCompleted: Bool) -> [Goal] {
        let filtered = includeCompleted ? goals : goals.filter { !$0.completed }
        return filtered.sorted(by: Eisenhower.compareGoals)
    }

    /// Active strategies always show (so they remain drop targets even with no
    /// goals). Completed/archived strategies show when `includeCompleted` is
    /// true, or when a visible goal still points at them so those goals do not
    /// look unaligned.
    public static func visibleStrategies(
        _ strategies: [Strategy],
        goals: [Goal],
        includeCompleted: Bool
    ) -> [Strategy] {
        let referenced = Set(goals.compactMap(\.strategyId))
        let filtered = strategies.filter { strategy in
            if referenced.contains(strategy.id) { return true }
            if strategy.status == "active" { return true }
            return includeCompleted
        }
        return filtered.sorted(by: Eisenhower.compareStrategies)
    }
}
