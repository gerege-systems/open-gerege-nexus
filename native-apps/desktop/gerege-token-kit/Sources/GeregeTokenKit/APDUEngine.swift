import Foundation
import CryptoTokenKit

// MARK: - APDU Command

/// ISO 7816-4 APDU команд бүтээгч.
/// iOS SmartCardKit.swift-тай ижил pattern.
/// Payload хэзээ ч log-д ил гарахгүй (PIN, key material байж болно).
public final class APDUCommand {
    public let cla: UInt8
    public let ins: UInt8
    public let p1: UInt8
    public let p2: UInt8
    public let lc: UInt8
    public let payload: [UInt8]?
    public let le: UInt8

    public init(cla: UInt8, ins: UInt8, p1: UInt8, p2: UInt8, data: [UInt8]? = nil, le: UInt8 = 0) throws {
        let length = data?.count ?? 0
        guard length <= 255 else { throw TokenError.invalidAPDU }
        self.cla = cla; self.ins = ins; self.p1 = p1; self.p2 = p2
        self.lc = UInt8(length); self.payload = data; self.le = le
    }

    public init(hex command: String) throws {
        let bytes = hexToBytes(command)
        guard bytes.count >= 4 else { throw TokenError.invalidAPDU }
        self.cla = bytes[0]; self.ins = bytes[1]; self.p1 = bytes[2]; self.p2 = bytes[3]

        var data: [UInt8]?
        var expectedLe: UInt8 = 0

        if bytes.count == 5 {
            expectedLe = bytes[4]
        } else if bytes.count > 5 {
            let dataLength = Int(bytes[4])
            let headerPlusData = dataLength + 5
            guard dataLength <= 255, headerPlusData <= bytes.count,
                  bytes.count - headerPlusData <= 1 else { throw TokenError.invalidAPDU }
            data = Array(bytes[5..<headerPlusData])
            if bytes.count == headerPlusData + 1 { expectedLe = bytes[headerPlusData] }
        }

        let length = data?.count ?? 0
        guard length <= 255 else { throw TokenError.invalidAPDU }
        self.lc = UInt8(length); self.payload = data; self.le = expectedLe
    }

    public func buildBuffer() -> [UInt8] {
        var buffer: [UInt8]
        if let payload = payload {
            buffer = [UInt8](repeating: 0, count: 5 + payload.count + (le != 0 ? 1 : 0))
            buffer[4] = lc
            buffer.replaceSubrange(5..<5 + payload.count, with: payload)
        } else if le != 0 {
            buffer = [UInt8](repeating: 0, count: 5)
        } else {
            buffer = [UInt8](repeating: 0, count: 4)
        }
        buffer[0] = cla; buffer[1] = ins; buffer[2] = p1; buffer[3] = p2
        if le != 0 { buffer[buffer.count - 1] = le }
        return buffer
    }

    public func redactedDescription() -> String {
        var result = String(format: "CLA=%02X INS=%02X P1=%02X P2=%02X", cla, ins, p1, p2)
        if payload != nil { result += String(format: " Lc=%02X Data=<redacted:\(lc)B>", lc) }
        result += String(format: " Le=%02X", le)
        return result
    }

    deinit {
        if var p = payload {
            p.withUnsafeMutableBufferPointer { ptr in
                if let base = ptr.baseAddress { base.update(repeating: 0, count: ptr.count) }
            }
        }
    }
}

// MARK: - APDU Response

public final class APDUResponse: CustomStringConvertible {
    public private(set) var data: [UInt8]?
    public let sw1: UInt8
    public let sw2: UInt8

    public init(fullData: [UInt8]) {
        let length = fullData.count
        guard length >= 2 else { sw1 = 0x6F; sw2 = 0x00; data = nil; return }
        if length > 2 { data = Array(fullData.prefix(length - 2)) } else { data = nil }
        sw1 = fullData[length - 2]
        sw2 = fullData[length - 1]
    }

    /// SM unwrap дараа шууд data + status-аар бүтээх.
    public init(data: [UInt8]?, sw1: UInt8, sw2: UInt8) {
        self.data = data
        self.sw1 = sw1
        self.sw2 = sw2
    }

    public var status: UInt16 { (UInt16(sw1) << 8) | UInt16(sw2) }
    public var isSuccess: Bool { sw1 == 0x90 && sw2 == 0x00 }
    public var hexStatus: String { String(format: "%02X%02X", sw1, sw2) }

    public var description: String {
        "APDUResponse(SW=\(hexStatus), data=<\(data?.count ?? 0)B>)"
    }

    public func wipe() {
        if var d = data {
            d.withUnsafeMutableBufferPointer { ptr in
                if let base = ptr.baseAddress { base.update(repeating: 0, count: ptr.count) }
            }
            data = nil
        }
    }

    deinit { wipe() }
}

// MARK: - Transceiver

/// TKSmartCard-руу APDU илгээж хариу авна.
public enum APDUTransceiver {

    public static func transmit(_ command: APDUCommand, to card: TKSmartCard) async throws -> APDUResponse {
        let data = Data(command.buildBuffer())
        let responseData = try await card.transmit(data)
        return APDUResponse(fullData: Array(responseData))
    }

    public static func transmit(hex: String, to card: TKSmartCard) async throws -> APDUResponse {
        let bytes = hexToBytes(hex)
        guard !bytes.isEmpty else { throw TokenError.invalidAPDU }
        let responseData = try await card.transmit(Data(bytes))
        return APDUResponse(fullData: Array(responseData))
    }
}

// MARK: - Hex helpers

public func hexToBytes(_ hex: String) -> [UInt8] {
    var cleaned = [Character]()
    cleaned.reserveCapacity(hex.count)
    for ch in hex where !ch.isWhitespace && ch != ":" { cleaned.append(ch) }
    guard cleaned.count % 2 == 0 else { return [] }

    var bytes = [UInt8]()
    bytes.reserveCapacity(cleaned.count / 2)
    var i = 0
    while i < cleaned.count {
        guard let high = cleaned[i].hexDigitValue,
              let low = cleaned[i + 1].hexDigitValue else { return [] }
        bytes.append(UInt8((high << 4) | low))
        i += 2
    }
    return bytes
}

public func bytesToHex(_ bytes: [UInt8]) -> String {
    bytes.map { String(format: "%02X", $0) }.joined()
}

public func constantTimeEquals(_ a: [UInt8], _ b: [UInt8]) -> Bool {
    guard a.count == b.count else { return false }
    var diff: UInt8 = 0
    for i in 0..<a.count { diff |= a[i] ^ b[i] }
    return diff == 0
}
