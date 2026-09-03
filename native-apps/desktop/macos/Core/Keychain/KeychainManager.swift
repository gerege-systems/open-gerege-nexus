import Foundation
import Security

enum KeychainKey: String {
    case nationalID   = "mn.gerege.nexus.desktop.national.id"
    case apiKey       = "mn.gerege.nexus.desktop.api.key"
    /// Bearer session token from /web/auth/session/:id/token. Stored at
    /// kSecAttrAccessibleWhenUnlockedThisDeviceOnly so the token does not
    /// follow the user to a different machine via Keychain sync.
    case sessionToken = "mn.gerege.nexus.desktop.session.token"
    /// v3: нэвтэрсэн иргэний identity snapshot (JSON) — documentNumber, нэр,
    /// civil ID, cert level. v3-д bearer token байхгүй тул үүнийг сэргээнэ.
    case identity     = "mn.gerege.nexus.desktop.identity"
}

/// macOS Keychain-д мэдээлэл хадгалах.
/// iOS KeychainManager-тай ижил pattern.
final class KeychainManager {
    static let shared = KeychainManager()

    private let service = "mn.gerege.nexus.desktop"

    private init() {}

    // MARK: - Save

    func save(_ data: Data, key: KeychainKey) throws {
        try? delete(key)

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key.rawValue,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        ]

        let status = SecItemAdd(query as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw KeychainError.saveFailed(status)
        }
    }

    func saveString(_ value: String, key: KeychainKey) throws {
        guard let data = value.data(using: .utf8) else {
            throw KeychainError.invalidData
        }
        try save(data, key: key)
    }

    // MARK: - Load

    func load(_ key: KeychainKey) throws -> Data {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key.rawValue,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)

        guard status == errSecSuccess, let data = item as? Data else {
            throw KeychainError.loadFailed(status)
        }

        return data
    }

    func loadString(_ key: KeychainKey) throws -> String {
        let data = try load(key)
        guard let string = String(data: data, encoding: .utf8) else {
            throw KeychainError.invalidData
        }
        return string
    }

    // MARK: - Delete

    func delete(_ key: KeychainKey) throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key.rawValue
        ]

        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw KeychainError.deleteFailed(status)
        }
    }

    func exists(_ key: KeychainKey) -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key.rawValue,
            kSecReturnData as String: false
        ]
        return SecItemCopyMatching(query as CFDictionary, nil) == errSecSuccess
    }
}

enum KeychainError: LocalizedError {
    case saveFailed(OSStatus)
    case loadFailed(OSStatus)
    case deleteFailed(OSStatus)
    case invalidData

    var errorDescription: String? {
        switch self {
        case .saveFailed(let s):   return "Keychain хадгалах алдаа: \(s)"
        case .loadFailed(let s):   return "Keychain уншиx алдаа: \(s)"
        case .deleteFailed(let s): return "Keychain устгах алдаа: \(s)"
        case .invalidData:         return "Буруу өгөгдөл."
        }
    }
}
