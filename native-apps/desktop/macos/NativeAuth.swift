import Foundation

enum LoginPhase: Equatable {
    case idle, starting, waiting(code: String, link: URL?), success, expired, refused, error(String)
}

struct NativeUserProfile: Equatable {
    let id: String
    let name: String
    let email: String
    let tenantID: String

    static let eidUser = NativeUserProfile(id: "", name: "eID хэрэглэгч", email: "", tenantID: "")
}

private struct PasswordLoginResponse: Decodable {
    struct User: Decodable {
        let id: String
        let name: String
        let email: String
        let tenantID: String
        enum CodingKeys: String, CodingKey { case id, name, email; case tenantID = "tenant_id" }
    }
    let user: User
}

struct EIDStart: Decodable {
    let sessionID: String
    let verificationCode: String
    let expiresAt: String?
    let deviceLinkURL: URL?
    enum CodingKeys: String, CodingKey {
        case sessionID = "session_id"
        case verificationCode = "verification_code"
        case expiresAt = "expires_at"
        case deviceLinkURL = "device_link_url"
    }
}

private struct EIDPoll: Decodable { let state: String }

@MainActor
final class NativeAuth: NSObject {
    private let apiBase: URL
    var onPhase: ((LoginPhase) -> Void)?
    private(set) var phase: LoginPhase = .idle { didSet { onPhase?(phase) } }
    private var ticket = 0
    private var task: Task<Void, Never>?
    private let session: URLSession
    private let requestOrigin: String
    private(set) var profile: NativeUserProfile?

    init(apiEndpoint: String) {
        let root = apiEndpoint.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        apiBase = URL(string: root.hasSuffix("/api/v1") ? root : root + "/api/v1")!
        var origin = URLComponents(url: apiBase, resolvingAgainstBaseURL: false)!
        origin.path = ""; origin.query = nil; origin.fragment = nil
        requestOrigin = origin.string!.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        let config = URLSessionConfiguration.default
        config.httpCookieStorage = .shared
        config.httpShouldSetCookies = true
        session = URLSession(configuration: config)
        super.init()
    }

    func cancel() {
        ticket += 1
        task?.cancel()
        task = nil
        phase = .idle
    }

    func password(email: String, password: String) {
        begin { [self] mine in
            let data = try await request("auth/login", body: ["email": email, "password": password])
            let response = try JSONDecoder().decode(PasswordLoginResponse.self, from: data)
            guard mine == ticket else { return }
            profile = NativeUserProfile(id: response.user.id, name: response.user.name,
                                        email: response.user.email, tenantID: response.user.tenantID)
            phase = .success
        }
    }

    func push(nationalID: String) {
        begin { [self] mine in
            let data = try await request("auth/eid/start-id", body: [
                "national_id": nationalID.trimmingCharacters(in: .whitespacesAndNewlines).uppercased(),
                "callbackUrl": ""
            ])
            let started = try JSONDecoder().decode(EIDStart.self, from: data)
            guard mine == ticket else { return }
            phase = .waiting(code: started.verificationCode, link: started.deviceLinkURL)
            try await poll(started, ticket: mine)
        }
    }

    func qr() {
        begin { [self] mine in
            let data = try await request("auth/eid/start", body: ["callbackUrl": ""])
            let started = try JSONDecoder().decode(EIDStart.self, from: data)
            guard mine == ticket else { return }
            phase = .waiting(code: started.verificationCode, link: started.deviceLinkURL)
            try await poll(started, ticket: mine)
        }
    }

    private func begin(_ operation: @escaping (Int) async throws -> Void) {
        cancel()
        ticket += 1
        let mine = ticket
        phase = .starting
        task = Task {
            do { try await operation(mine) }
            catch is CancellationError { }
            catch {
                guard mine == ticket else { return }
                phase = .error(error.localizedDescription)
            }
        }
    }

    private func poll(_ start: EIDStart, ticket mine: Int) async throws {
        let parsedDeadline = start.expiresAt.flatMap { ISO8601DateFormatter().date(from: $0) }
        let deadline = parsedDeadline ?? Date().addingTimeInterval(15 * 60)
        var failures = 0
        while mine == ticket && !Task.isCancelled {
            if Date() >= deadline { phase = .expired; return }
            do {
                let data = try await request("auth/eid/poll", body: ["session_id": start.sessionID])
                failures = 0
                let result = try JSONDecoder().decode(EIDPoll.self, from: data)
                guard mine == ticket else { return }
                switch result.state.uppercased() {
                case "COMPLETE": profile = .eidUser; phase = .success; return
                case "EXPIRED": phase = .expired; return
                case "REFUSED": phase = .refused; return
                default: break
                }
            } catch is CancellationError { throw CancellationError() }
            catch {
                failures += 1
                if failures > 3 { throw error }
            }
            try await Task.sleep(nanoseconds: 400_000_000)
        }
    }

    @discardableResult
    private func request(_ path: String, body: [String: String]) async throws -> Data {
        var request = URLRequest(url: apiBase.appendingPathComponent(path))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("mn", forHTTPHeaderField: "Accept-Language")
        request.setValue(requestOrigin, forHTTPHeaderField: "Origin")
        request.httpBody = try JSONSerialization.data(withJSONObject: body)
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            let message = (try? JSONSerialization.jsonObject(with: data) as? [String: Any])?["error"] as? String
            throw NSError(domain: "GeregeAuth", code: (response as? HTTPURLResponse)?.statusCode ?? -1,
                          userInfo: [NSLocalizedDescriptionKey: message ?? "Нэвтрэх хүсэлт амжилтгүй"])
        }
        return data
    }

    func sessionCookies() -> [HTTPCookie] {
        HTTPCookieStorage.shared.cookies?.filter { $0.name == "session_token" } ?? []
    }
}
