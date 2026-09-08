import Foundation

public struct HrsClient: Sendable {
    public var baseURL: URL
    public var session: URLSession

    public init(baseURL: URL, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    public static let defaultBaseURL = URL(string: "http://127.0.0.1:9746")!

    public func health() async throws -> Bool {
        let data: Health = try await get("health")
        return data.status == "ok"
    }

    public func strategies() async throws -> [Strategy] {
        try await get("strategies")
    }

    public func strategyReport(_ id: Int64) async throws -> StrategyReport {
        try await get("strategies/\(id)")
    }

    public func activeGoals() async throws -> [Goal] {
        try await get("goals/active")
    }

    public func allGoals() async throws -> [Goal] {
        try await get("goals?all=1")
    }

    public func setStrategy(goalID: Int64, strategyID: Int64?) async throws -> Goal {
        let body = Self.encodeStrategyPatch(strategyID: strategyID)
        return try await put("goals/\(goalID)", body: body)
    }

    public func setStrategyStatus(_ id: Int64, status: String) async throws {
        _ = try await putRaw("strategies/\(id)", body: Self.encodeStatusPatch(status: status))
    }

    public func completeGoal(_ id: Int64) async throws {
        _ = try await putRaw("goals/\(id)/done", body: Data("{}".utf8))
    }

    public func undoGoal(_ id: Int64) async throws {
        _ = try await putRaw("goals/\(id)/undo", body: Data("{}".utf8))
    }

    public static func encodeStrategyPatch(strategyID: Int64?) -> Data {
        if let strategyID {
            let payload = ["strategy_id": strategyID]
            return try! JSONEncoder().encode(payload)
        }
        return Data(#"{"strategy_id":null}"#.utf8)
    }

    public static func encodeStatusPatch(status: String) -> Data {
        try! JSONEncoder().encode(["status": status])
    }

    public static func endpoint(base: URL, path: String) throws -> URL {
        guard var components = URLComponents(url: base, resolvingAgainstBaseURL: false) else {
            throw HrsClientError.badURL(path)
        }
        var remainder = path
        var query: String?
        if let idx = remainder.firstIndex(of: "?") {
            query = String(remainder[remainder.index(after: idx)...])
            remainder = String(remainder[..<idx])
        }
        if remainder.hasPrefix("/") {
            remainder.removeFirst()
        }
        var basePath = components.path
        if basePath.hasSuffix("/") {
            basePath.removeLast()
        }
        components.path = basePath + "/" + remainder
        if let query {
            components.percentEncodedQuery = query
        }
        guard let url = components.url else { throw HrsClientError.badURL(path) }
        return url
    }

    private struct Health: Decodable { var status: String }

    private func get<T: Decodable>(_ path: String) async throws -> T {
        let request = URLRequest(url: try Self.endpoint(base: baseURL, path: path))
        return try await send(request)
    }

    private func put<T: Decodable>(_ path: String, body: Data) async throws -> T {
        let data = try await putRaw(path, body: body)
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw HrsClientError.decode(error)
        }
    }

    private func putRaw(_ path: String, body: Data) async throws -> Data {
        var request = URLRequest(url: try Self.endpoint(base: baseURL, path: path))
        request.httpMethod = "PUT"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = body
        return try await sendRaw(request)
    }

    private func send<T: Decodable>(_ request: URLRequest) async throws -> T {
        let data = try await sendRaw(request)
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw HrsClientError.decode(error)
        }
    }

    private func sendRaw(_ request: URLRequest) async throws -> Data {
        let (data, response) = try await session.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        if !(200...299).contains(status) {
            let message = String(data: data, encoding: .utf8) ?? "HTTP \(status)"
            throw HrsClientError.http(status, message)
        }
        return data
    }
}

public enum HrsClientError: LocalizedError {
    case badURL(String)
    case http(Int, String)
    case decode(Error)

    public var errorDescription: String? {
        switch self {
        case .badURL(let path):
            return "Could not build URL for \(path)"
        case .http(let code, let body):
            return "hrs serve returned \(code): \(body)"
        case .decode(let error):
            return "Could not decode hrs response: \(error.localizedDescription)"
        }
    }
}
