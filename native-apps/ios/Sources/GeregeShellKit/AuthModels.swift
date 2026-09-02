import Foundation

public enum LoginPhase: Equatable, Sendable {
    case idle
    case starting
    case waiting(verificationCode: String, deviceLinkURL: URL?)
    case success
    case expired
    case refused
    case error(String)
}

public struct SessionCookie: Equatable, Sendable {
    public let name: String
    public let value: String
    public let domain: String
    public let path: String
    public let secure: Bool
    public let httpOnly: Bool

    public init(name: String, value: String, domain: String, path: String, secure: Bool, httpOnly: Bool) {
        self.name = name; self.value = value; self.domain = domain; self.path = path
        self.secure = secure; self.httpOnly = httpOnly
    }
}

struct EIDStart: Decodable, Sendable {
    let sessionID: String
    let deviceLinkURL: URL?
    let verificationCode: String
    let expiresAt: Date?

    enum CodingKeys: String, CodingKey {
        case sessionID = "session_id"
        case deviceLinkURL = "device_link_url"
        case verificationCode = "verification_code"
        case expiresAt = "expires_at"
    }
}

struct EIDPoll: Decodable, Sendable { let state: String }
