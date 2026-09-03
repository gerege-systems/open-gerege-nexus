import Foundation
import CryptoTokenKit
import GeregeTokenKit
import os

/// USB токен илрүүлэх, GeregeTokenKit ашиглан APDU-аар харилцах.
/// UI layer-д зориулсан ObservableObject wrapper.
@MainActor
final class TokenManager: ObservableObject {

    @Published var isTokenPresent = false
    @Published var readerNames: [String] = []
    @Published var tokenInfo: GeregeTokenKit.TokenInfo?
    @Published var errorMessage: String?

    private var watcher: TKTokenWatcher?
    private let kit = GeregeTokenKit.TokenManager.shared
    private let driver = BioPassDriver()
    private let logger = Logger(subsystem: "mn.gerege.nexus.desktop", category: "TokenManager")

    init() {
        startWatching()
    }

    // MARK: - Token Detection

    func startWatching() {
        watcher = TKTokenWatcher()

        refreshTokenList()

        watcher?.setInsertionHandler { [weak self] tokenID in
            Task { @MainActor in
                self?.logger.info("Токен залгагдлаа: \(tokenID)")
                self?.refreshTokenList()
            }
        }
    }

    func refreshTokenList() {
        readerNames = kit.getReaderNames()
        isTokenPresent = kit.hasToken()

        if isTokenPresent {
            loadTokenInfo()
        } else {
            tokenInfo = nil
        }
    }

    // MARK: - Token Info

    private func loadTokenInfo() {
        Task {
            do {
                let info = try await kit.withSession { [driver] card in
                    try await driver.getTokenInfo(card: card)
                }
                self.tokenInfo = info
                logger.info("Токен: \(info.label), FIPS=\(info.isFIPS)")
            } catch {
                logger.error("Token info алдаа: \(error.localizedDescription)")
                self.errorMessage = error.localizedDescription
            }
        }
    }

    // MARK: - VERIFY PIN

    func verifyPIN(_ pin: String) async throws {
        try await kit.withSession { [driver] card in
            _ = try await driver.selectApplet(card: card)
            try await driver.verifyPIN(pin, card: card)
        }
    }

    // MARK: - SIGN

    func signECDSA(hash: [UInt8]) async throws -> [UInt8] {
        try await kit.withSession { [driver] card in
            _ = try await driver.selectApplet(card: card)
            return try await driver.signECDSA(hash: hash, card: card)
        }
    }

    func signRSA(data: [UInt8]) async throws -> [UInt8] {
        try await kit.withSession { [driver] card in
            _ = try await driver.selectApplet(card: card)
            return try await driver.signRSA(data: data, card: card)
        }
    }

    // MARK: - GET CHALLENGE (for testing connectivity)

    func testConnection() async -> Bool {
        do {
            let challenge = try await kit.withSession { [driver] card in
                _ = try await driver.selectApplet(card: card)
                return try await driver.getChallenge(card: card)
            }
            logger.info("Token connection OK, challenge: \(bytesToHex(challenge))")
            return true
        } catch {
            logger.error("Token connection failed: \(error.localizedDescription)")
            errorMessage = error.localizedDescription
            return false
        }
    }
}
