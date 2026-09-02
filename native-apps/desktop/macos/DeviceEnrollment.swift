import Foundation
import Security

struct MacEnrolledDevice: Decodable { let deviceID, deviceToken: String; enum CodingKeys: String, CodingKey { case deviceID = "device_id", deviceToken = "device_token" } }
struct MacDeviceIdentity: Decodable { let id, name, status: String }

final class MacDeviceEnrollmentClient {
    private func url(_ endpoint: String, _ path: String) throws -> URL {
        var root = endpoint.trimmingCharacters(in: CharacterSet(charactersIn: "/")); if !root.hasSuffix("/api/v1") { root += "/api/v1" }
        guard let result = URL(string: "\(root)/\(path)") else { throw NSError(domain: "GeregeDevice", code: 1, userInfo: [NSLocalizedDescriptionKey: "API endpoint буруу байна"]) }; return result
    }
    func enroll(endpoint: String, code: String, name: String, site: String) async throws -> MacEnrolledDevice {
        var request = URLRequest(url: try url(endpoint, "devices/enroll")); request.httpMethod = "POST"; request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: ["code": code, "name": name, "site": site, "platform": "macos", "form_factor": "desktop", "app_version": "dev", "os_version": ProcessInfo.processInfo.operatingSystemVersionString])
        return try await send(request, MacEnrolledDevice.self)
    }
    func identity(endpoint: String, token: String) async throws -> MacDeviceIdentity { var request = URLRequest(url: try url(endpoint, "devices/me")); request.setValue("Device \(token)", forHTTPHeaderField: "Authorization"); return try await send(request, MacDeviceIdentity.self) }
    private func send<T: Decodable>(_ request: URLRequest, _ type: T.Type) async throws -> T { let (data, response) = try await URLSession.shared.data(for: request); guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else { throw NSError(domain: "GeregeDevice", code: 2, userInfo: [NSLocalizedDescriptionKey: String(data: data, encoding: .utf8) ?? "Enrollment API алдаа"]) }; return try JSONDecoder().decode(type, from: data) }
}

enum MacDeviceTokenStore {
    private static let query: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "mn.gerege.nexus.device-token", kSecAttrAccount as String: "device"]
    static func save(_ token: String) throws { SecItemDelete(query as CFDictionary); var item = query; item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly; item[kSecValueData as String] = Data(token.utf8); let result = SecItemAdd(item as CFDictionary, nil); guard result == errSecSuccess else { throw NSError(domain: NSOSStatusErrorDomain, code: Int(result)) } }
    static func load() -> String? { var item = query; item[kSecReturnData as String] = true; item[kSecMatchLimit as String] = kSecMatchLimitOne; var result: CFTypeRef?; guard SecItemCopyMatching(item as CFDictionary, &result) == errSecSuccess, let data = result as? Data else { return nil }; return String(data: data, encoding: .utf8) }
    static func clear() { SecItemDelete(query as CFDictionary) }
}
