import Combine
import Foundation

@MainActor
public final class AuthController: ObservableObject {
    @Published public private(set) var phase: LoginPhase = .idle
    public private(set) var sessionCookies: [SessionCookie] = []

    private let transport: any AuthTransport
    private var attempt: Task<Void, Never>?
    private var ticket: UInt64 = 0

    public init(transport: any AuthTransport = URLSessionAuthTransport()) { self.transport = transport }

    public func cancel() {
        ticket &+= 1; attempt?.cancel(); attempt = nil; phase = .idle
    }

    public func password(email: String, password: String) {
        begin { [transport] mine in
            let response = try await transport.post(path: "auth/login", body: ["email": email, "password": password])
            await self.finish(response, ticket: mine)
        }
    }

    public func push(nationalID: String, callbackURL: String = "") {
        begin { [transport] mine in
            let response = try await transport.post(path: "auth/eid/start-id", body: [
                "national_id": nationalID.trimmingCharacters(in: .whitespacesAndNewlines).uppercased(),
                "callbackUrl": callbackURL,
            ])
            let start = try Self.decode(EIDStart.self, from: response.data)
            await self.watch(start, ticket: mine)
        }
    }

    public func appToApp(callbackURL: String) {
        begin { [transport] mine in
            let response = try await transport.post(path: "auth/eid/start", body: ["callbackUrl": callbackURL])
            let start = try Self.decode(EIDStart.self, from: response.data)
            await self.watch(start, ticket: mine)
        }
    }

    private func begin(_ operation: @escaping @Sendable (UInt64) async throws -> Void) {
        cancel(); ticket &+= 1
        let mine = ticket; phase = .starting
        attempt = Task {
            do { try await operation(mine) }
            catch is CancellationError { }
            catch {
                guard ticket == mine else { return }
                phase = .error(error.localizedDescription)
            }
        }
    }

    private func watch(_ start: EIDStart, ticket mine: UInt64) async {
        phase = .waiting(verificationCode: start.verificationCode, deviceLinkURL: start.deviceLinkURL)
        let deadline = start.expiresAt ?? Date().addingTimeInterval(15 * 60)
        var failures = 0
        while ticket == mine && !Task.isCancelled {
            if Date() >= deadline { phase = .expired; return }
            do {
                let response = try await transport.post(path: "auth/eid/poll", body: ["session_id": start.sessionID])
                failures = 0
                let poll = try Self.decode(EIDPoll.self, from: response.data)
                guard ticket == mine else { return }
                switch poll.state.uppercased() {
                case "COMPLETE": finish(response, ticket: mine); return
                case "EXPIRED": phase = .expired; return
                case "REFUSED": phase = .refused; return
                default: break
                }
            } catch is CancellationError { return }
            catch {
                failures += 1
                if failures > 3 { phase = .error(error.localizedDescription); return }
            }
            try? await Task.sleep(for: .milliseconds(400))
        }
    }

    private func finish(_ response: AuthResponse, ticket mine: UInt64) {
        guard ticket == mine else { return }
        sessionCookies = response.cookies.filter { $0.name == "session_token" }
        phase = .success
    }

    nonisolated private static func decode<T: Decodable>(_ type: T.Type, from data: Data) throws -> T {
        let decoder = JSONDecoder(); decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(type, from: data)
    }
}
