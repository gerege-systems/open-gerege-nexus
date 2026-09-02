import Foundation

public struct AuthResponse: Sendable {
    public let data: Data
    public let cookies: [SessionCookie]
    public init(data: Data, cookies: [SessionCookie] = []) { self.data = data; self.cookies = cookies }
}

public protocol AuthTransport: Sendable {
    func post(path: String, body: [String: String]) async throws -> AuthResponse
}

public struct AuthHTTPError: LocalizedError, Sendable {
    public let status: Int
    public let message: String
    public var errorDescription: String? { message }
}

public final class URLSessionAuthTransport: AuthTransport, @unchecked Sendable {
    public let apiBase: URL
    private let session: URLSession

    public init(apiBase: URL = URL(string: GeregeDeviceLine.origin + "/api/v1/")!, session: URLSession = .shared) {
        self.apiBase = apiBase; self.session = session
    }

    public func post(path: String, body: [String: String]) async throws -> AuthResponse {
        let url = apiBase.appending(path: path)
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("mn", forHTTPHeaderField: "Accept-Language")
        var origin = URLComponents(url: apiBase, resolvingAgainstBaseURL: false)!
        origin.path = ""; origin.query = nil; origin.fragment = nil
        request.setValue(origin.string?.trimmingCharacters(in: CharacterSet(charactersIn: "/")), forHTTPHeaderField: "Origin")
        request.httpBody = try JSONSerialization.data(withJSONObject: body)
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw AuthHTTPError(status: -1, message: "Серверийн хариу танигдсангүй")
        }
        guard (200..<300).contains(http.statusCode) else {
            let message = (try? JSONSerialization.jsonObject(with: data) as? [String: Any])?["error"] as? String
            throw AuthHTTPError(status: http.statusCode, message: message ?? "HTTP \(http.statusCode)")
        }
        let fields = http.allHeaderFields.reduce(into: [String: String]()) { result, item in
            guard let key = item.key as? String, let value = item.value as? String else { return }
            result[key] = value
        }
        let cookies = HTTPCookie.cookies(withResponseHeaderFields: fields, for: url).map {
            SessionCookie(name: $0.name, value: $0.value, domain: $0.domain, path: $0.path,
                          secure: $0.isSecure, httpOnly: $0.isHTTPOnly)
        }
        return AuthResponse(data: data, cookies: cookies)
    }
}
