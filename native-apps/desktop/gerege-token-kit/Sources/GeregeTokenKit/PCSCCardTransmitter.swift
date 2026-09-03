import Foundation
import CryptoTokenKit

/// TKSmartCard-ийг CardTransmitter protocol-д adapt хийнэ.
public final class TKSmartCardTransmitter: CardTransmitter {
    private let card: TKSmartCard

    public init(card: TKSmartCard) {
        self.card = card
    }

    public func transmitRaw(_ apdu: [UInt8]) async throws -> [UInt8] {
        let response = try await card.transmit(Data(apdu))
        return Array(response)
    }
}
