import Foundation
import Testing
@testable import GeregeShellKit

actor StubTransport: AuthTransport {
    var responses: [Result<AuthResponse, Error>]
    private(set) var paths: [String] = []
    init(_ responses: [Result<AuthResponse, Error>]) { self.responses = responses }
    func post(path: String, body: [String: String]) async throws -> AuthResponse {
        paths.append(path)
        return try responses.removeFirst().get()
    }
}

@MainActor
struct AuthControllerTests {
    @Test func passwordSuccessPublishesCookie() async throws {
        let cookie = SessionCookie(name: "session_token", value: "abc", domain: "localhost", path: "/", secure: false, httpOnly: true)
        let transport = StubTransport([.success(AuthResponse(data: Data("{}".utf8), cookies: [cookie]))])
        let controller = AuthController(transport: transport)
        controller.password(email: "admin@example.com", password: "secret")
        try await wait { controller.phase == .success }
        #expect(controller.sessionCookies == [cookie])
        #expect(await transport.paths == ["auth/login"])
    }

    @Test func refusedPollStopsAttempt() async throws {
        let start = #"{"session_id":"s1","verification_code":"4722"}"#
        let transport = StubTransport([
            .success(AuthResponse(data: Data(start.utf8))),
            .success(AuthResponse(data: Data(#"{"state":"REFUSED"}"#.utf8))),
        ])
        let controller = AuthController(transport: transport)
        controller.push(nationalID: "aa00112233")
        try await wait { controller.phase == .refused }
        #expect(await transport.paths == ["auth/eid/start-id", "auth/eid/poll"])
    }

    private func wait(_ condition: @escaping @MainActor () -> Bool) async throws {
        for _ in 0..<100 {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        Issue.record("Timed out waiting for auth state")
    }
}
