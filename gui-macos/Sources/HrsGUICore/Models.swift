import Foundation

public struct Strategy: Codable, Identifiable, Hashable, Sendable {
    public var id: Int64
    public var title: String
    public var description: String
    public var status: String
    public var createdAt: String
    public var completedAt: String?
    public var important: Bool
    public var urgent: Bool
    public var ticketRef: String?

    public init(
        id: Int64,
        title: String,
        description: String = "",
        status: String = "active",
        createdAt: String = "",
        completedAt: String? = nil,
        important: Bool = false,
        urgent: Bool = false,
        ticketRef: String? = nil
    ) {
        self.id = id
        self.title = title
        self.description = description
        self.status = status
        self.createdAt = createdAt
        self.completedAt = completedAt
        self.important = important
        self.urgent = urgent
        self.ticketRef = ticketRef
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(Int64.self, forKey: .id)
        title = try c.decode(String.self, forKey: .title)
        description = try c.decodeIfPresent(String.self, forKey: .description) ?? ""
        status = try c.decodeIfPresent(String.self, forKey: .status) ?? "active"
        createdAt = try c.decodeIfPresent(String.self, forKey: .createdAt) ?? ""
        completedAt = try c.decodeIfPresent(String.self, forKey: .completedAt)
        important = try c.decodeIfPresent(Bool.self, forKey: .important) ?? false
        urgent = try c.decodeIfPresent(Bool.self, forKey: .urgent) ?? false
        ticketRef = try c.decodeIfPresent(String.self, forKey: .ticketRef)
    }

    enum CodingKeys: String, CodingKey {
        case id, title, description, status, important, urgent
        case createdAt = "created_at"
        case completedAt = "completed_at"
        case ticketRef = "ticket_ref"
    }
}

public struct StrategyReport: Codable, Identifiable, Hashable, Sendable {
    public var id: Int64
    public var title: String
    public var description: String
    public var status: String
    public var createdAt: String
    public var completedAt: String?
    public var important: Bool
    public var urgent: Bool
    public var ticketRef: String?
    public var goalsDone: Int
    public var goalsTotal: Int
    public var totalHours: Double

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(Int64.self, forKey: .id)
        title = try c.decode(String.self, forKey: .title)
        description = try c.decodeIfPresent(String.self, forKey: .description) ?? ""
        status = try c.decodeIfPresent(String.self, forKey: .status) ?? "active"
        createdAt = try c.decodeIfPresent(String.self, forKey: .createdAt) ?? ""
        completedAt = try c.decodeIfPresent(String.self, forKey: .completedAt)
        important = try c.decodeIfPresent(Bool.self, forKey: .important) ?? false
        urgent = try c.decodeIfPresent(Bool.self, forKey: .urgent) ?? false
        ticketRef = try c.decodeIfPresent(String.self, forKey: .ticketRef)
        goalsDone = try c.decodeIfPresent(Int.self, forKey: .goalsDone) ?? 0
        goalsTotal = try c.decodeIfPresent(Int.self, forKey: .goalsTotal) ?? 0
        totalHours = try c.decodeIfPresent(Double.self, forKey: .totalHours) ?? 0
    }

    enum CodingKeys: String, CodingKey {
        case id, title, description, status, important, urgent
        case createdAt = "created_at"
        case completedAt = "completed_at"
        case ticketRef = "ticket_ref"
        case goalsDone = "goals_done"
        case goalsTotal = "goals_total"
        case totalHours = "total_hours"
    }
}

public struct Goal: Codable, Identifiable, Hashable, Sendable {
    public var id: Int64
    public var date: String
    public var text: String
    public var completed: Bool
    public var entryIds: [Int64]
    public var strategyId: Int64?
    public var important: Bool
    public var urgent: Bool
    public var ticketRef: String?

    public init(
        id: Int64,
        date: String,
        text: String,
        completed: Bool = false,
        entryIds: [Int64] = [],
        strategyId: Int64? = nil,
        important: Bool = false,
        urgent: Bool = false,
        ticketRef: String? = nil
    ) {
        self.id = id
        self.date = date
        self.text = text
        self.completed = completed
        self.entryIds = entryIds
        self.strategyId = strategyId
        self.important = important
        self.urgent = urgent
        self.ticketRef = ticketRef
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(Int64.self, forKey: .id)
        date = try c.decode(String.self, forKey: .date)
        text = try c.decode(String.self, forKey: .text)
        completed = try c.decodeIfPresent(Bool.self, forKey: .completed) ?? false
        entryIds = try c.decodeIfPresent([Int64].self, forKey: .entryIds) ?? []
        strategyId = try c.decodeIfPresent(Int64.self, forKey: .strategyId)
        important = try c.decodeIfPresent(Bool.self, forKey: .important) ?? false
        urgent = try c.decodeIfPresent(Bool.self, forKey: .urgent) ?? false
        ticketRef = try c.decodeIfPresent(String.self, forKey: .ticketRef)
    }

    public var isUnaligned: Bool { strategyId == nil }

    enum CodingKeys: String, CodingKey {
        case id, date, text, completed, important, urgent
        case entryIds = "entry_ids"
        case strategyId = "strategy_id"
        case ticketRef = "ticket_ref"
    }
}

public enum NodeID: Hashable, Sendable {
    case strategy(Int64)
    case goal(Int64)
}

public enum Eisenhower {
    /// Important first, then urgent, then newest date, then id.
    public static func compareGoals(_ a: Goal, _ b: Goal) -> Bool {
        if a.important != b.important { return a.important && !b.important }
        if a.urgent != b.urgent { return a.urgent && !b.urgent }
        if a.date != b.date { return a.date > b.date }
        return a.id < b.id
    }

    public static func compareStrategies(_ a: Strategy, _ b: Strategy) -> Bool {
        if a.important != b.important { return a.important && !b.important }
        if a.urgent != b.urgent { return a.urgent && !b.urgent }
        return a.id < b.id
    }
}
