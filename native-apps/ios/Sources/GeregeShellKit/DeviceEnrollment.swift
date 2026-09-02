import Foundation
import Security

public struct EnrolledDevice: Decodable, Sendable {
    public let deviceID: String
    public let tenantID: String
    public let deviceToken: String
    enum CodingKeys: String, CodingKey { case deviceID = "device_id", tenantID = "tenant_id", deviceToken = "device_token" }
}

public struct DeviceIdentity: Decodable, Sendable {
    public let id: String
    public let tenantID: String
    public let name: String
    public let platform: String
    public let formFactor: String
    public let status: String
    enum CodingKeys: String, CodingKey { case id, name, platform, status; case tenantID = "tenant_id", formFactor = "form_factor" }
}

public final class DeviceEnrollmentClient: @unchecked Sendable {
    private let session: URLSession
    public init(session: URLSession = .shared) { self.session = session }

    public func enroll(apiEndpoint: String, code: String, name: String, platform: String, formFactor: String, site: String, appVersion: String, osVersion: String) async throws -> EnrolledDevice {
        var request = URLRequest(url: try endpoint(apiEndpoint, path: "devices/enroll"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: ["code": code, "name": name, "platform": platform, "form_factor": formFactor, "site": site, "app_version": appVersion, "os_version": osVersion])
        return try await send(request, as: EnrolledDevice.self)
    }

    public func identity(apiEndpoint: String, token: String) async throws -> DeviceIdentity {
        var request = URLRequest(url: try endpoint(apiEndpoint, path: "devices/me"))
        request.setValue("Device \(token)", forHTTPHeaderField: "Authorization")
        return try await send(request, as: DeviceIdentity.self)
    }

    private func endpoint(_ base: String, path: String) throws -> URL {
        guard var components = URLComponents(string: base) else { throw AuthHTTPError(status: -1, message: "API endpoint буруу байна") }
        let prefix = components.path.hasSuffix("/api/v1") || components.path.hasSuffix("/api/v1/") ? components.path : components.path.trimmingCharacters(in: CharacterSet(charactersIn: "/")) + "/api/v1"
        components.path = "/" + prefix.trimmingCharacters(in: CharacterSet(charactersIn: "/")) + "/" + path
        guard let url = components.url else { throw AuthHTTPError(status: -1, message: "API endpoint буруу байна") }
        return url
    }

    private func send<T: Decodable>(_ request: URLRequest, as type: T.Type) async throws -> T {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw AuthHTTPError(status: -1, message: "Серверийн хариу танигдсангүй") }
        guard (200..<300).contains(http.statusCode) else {
            let message = (try? JSONSerialization.jsonObject(with: data) as? [String: Any])?["error"] as? String
            throw AuthHTTPError(status: http.statusCode, message: message ?? "HTTP \(http.statusCode)")
        }
        return try JSONDecoder().decode(type, from: data)
    }
}

public struct DeviceTokenStore: Sendable {
    private let service = "mn.gerege.nexus.device-token"
    private let account = "device"
    public init() {}

    public func save(_ token: String) throws {
        try? clear()
        let status = SecItemAdd([kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: account,
                                 kSecAttrAccessible: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
                                 kSecValueData: Data(token.utf8)] as CFDictionary, nil)
        guard status == errSecSuccess else { throw AuthHTTPError(status: Int(status), message: "Keychain-д token хадгалж чадсангүй") }
    }

    public func load() throws -> String? {
        var item: CFTypeRef?
        let status = SecItemCopyMatching([kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: account,
                                          kSecReturnData: true, kSecMatchLimit: kSecMatchLimitOne] as CFDictionary, &item)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess, let data = item as? Data else { throw AuthHTTPError(status: Int(status), message: "Keychain token уншиж чадсангүй") }
        return String(data: data, encoding: .utf8)
    }

    public func clear() throws {
        let status = SecItemDelete([kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: account] as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else { throw AuthHTTPError(status: Int(status), message: "Keychain token устгаж чадсангүй") }
    }
}
