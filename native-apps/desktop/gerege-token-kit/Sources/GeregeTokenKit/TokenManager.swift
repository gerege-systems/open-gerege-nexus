import Foundation
import CryptoTokenKit
import os

/// USB токен илрүүлэх, session удирдах.
/// iOS SmartCardKit.swift-ийн withSession() pattern-тай ижил.
public final class TokenManager {

    public static let shared = TokenManager()

    private let logger = Logger(subsystem: "mn.gerege.token-kit", category: "TokenManager")

    private init() {}

    // MARK: - Reader detection

    /// Холбогдсон smart card reader-ийн нэрсийг буцаана.
    public func getReaderNames() -> [String] {
        TKSmartCardSlotManager.default?.slotNames ?? []
    }

    /// Токен залгагдсан reader байгаа эсэх.
    public func hasToken() -> Bool {
        !getReaderNames().isEmpty
    }

    // MARK: - Session

    /// Токентэй session нээж, operation гүйцэтгэнэ.
    /// Session автоматаар хаагдана.
    public func withSession<T>(
        readerIndex: Int = 0,
        _ operation: (TKSmartCard) async throws -> T
    ) async throws -> T {
        let manager = TKSmartCardSlotManager.default
        guard let slotNames = manager?.slotNames,
              readerIndex >= 0, readerIndex < slotNames.count else {
            throw TokenError.readerNotConnected
        }

        guard let slot = await manager?.getSlot(withName: slotNames[readerIndex]) else {
            throw TokenError.cardNotFound
        }

        guard let card = slot.makeSmartCard() else {
            throw TokenError.cardNotFound
        }

        try await card.beginSession()
        defer { card.endSession() }

        logger.info("Session нээгдлээ: \(slotNames[readerIndex])")

        return try await operation(card)
    }

    // MARK: - ATR

    /// Токений ATR (Answer To Reset) авна.
    public func getATR(readerIndex: Int = 0) async throws -> [UInt8] {
        try await withSession(readerIndex: readerIndex) { card in
            guard let atr = card.slot.atr else { throw TokenError.cardNotFound }
            return Array(atr.bytes)
        }
    }
}
