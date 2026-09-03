#!/usr/bin/env xcrun swiftc -framework PCSC -framework Security -o /tmp/reset-token

// BioPass2003 lock болсон токеныг macOS дээр reset хийх tool.
// Compile: xcrun swiftc -framework PCSC -framework Security Scripts/reset-token.swift -o /tmp/reset-token
// Run: /tmp/reset-token

import Foundation
import CommonCrypto

// MARK: - PCSC bindings

typealias SCARDCONTEXT = UInt
typealias SCARDHANDLE = UInt
typealias DWORD = UInt32

@_silgen_name("SCardEstablishContext")
func SCardEstablishContext(_ s: DWORD, _ r1: UnsafeRawPointer?, _ r2: UnsafeRawPointer?, _ ctx: UnsafeMutablePointer<SCARDCONTEXT>) -> Int32
@_silgen_name("SCardListReaders")
func SCardListReaders(_ ctx: SCARDCONTEXT, _ g: UnsafePointer<CChar>?, _ r: UnsafeMutablePointer<CChar>?, _ l: UnsafeMutablePointer<DWORD>) -> Int32
@_silgen_name("SCardConnect")
func SCardConnect(_ ctx: SCARDCONTEXT, _ r: UnsafePointer<CChar>, _ sm: DWORD, _ p: DWORD, _ h: UnsafeMutablePointer<SCARDHANDLE>, _ ap: UnsafeMutablePointer<DWORD>) -> Int32
@_silgen_name("SCardDisconnect")
func SCardDisconnect(_ h: SCARDHANDLE, _ d: DWORD) -> Int32
@_silgen_name("SCardReleaseContext")
func SCardReleaseContext(_ ctx: SCARDCONTEXT) -> Int32
@_silgen_name("SCardTransmit")
func SCardTransmit(_ h: SCARDHANDLE, _ sp: UnsafeRawPointer, _ sb: UnsafePointer<UInt8>, _ sl: DWORD, _ rp: UnsafeMutableRawPointer?, _ rb: UnsafeMutablePointer<UInt8>, _ rl: UnsafeMutablePointer<DWORD>) -> Int32
@_silgen_name("SCardReconnect")
func SCardReconnect(_ h: SCARDHANDLE, _ sm: DWORD, _ p: DWORD, _ init: DWORD, _ ap: UnsafeMutablePointer<DWORD>) -> Int32

struct PCI { var proto: DWORD; var len: DWORD }
var pciT1 = PCI(proto: 2, len: 8)

func hex(_ b: [UInt8]) -> String { b.map { String(format: "%02X", $0) }.joined(separator: " ") }

func send(_ card: SCARDHANDLE, _ apdu: [UInt8]) -> (data: [UInt8], sw1: UInt8, sw2: UInt8)? {
    var recv = [UInt8](repeating: 0, count: 258)
    var recvLen: DWORD = 258
    let r = apdu.withUnsafeBufferPointer { buf in
        withUnsafeMutablePointer(to: &pciT1) { p in
            SCardTransmit(card, p, buf.baseAddress!, DWORD(apdu.count), nil, &recv, &recvLen)
        }
    }
    guard r == 0, recvLen >= 2 else { return nil }
    let n = Int(recvLen)
    return (n > 2 ? Array(recv[0..<n-2]) : [], recv[n-2], recv[n-1])
}

func ccCrypt(op: Int, alg: Int, opts: Int, key: [UInt8], iv: [UInt8]?, data: [UInt8]) -> [UInt8]? {
    let outSize = data.count + 24
    var out = [UInt8](repeating: 0, count: outSize)
    var outLen = 0
    let s = CCCrypt(CCOperation(op), CCAlgorithm(alg), CCOptions(opts),
                    key, key.count, iv, data, data.count, &out, outSize, &outLen)
    return s == 0 ? Array(out.prefix(outLen)) : nil
}

func des3ECB(_ key: [UInt8], _ data: [UInt8]) -> [UInt8]? {
    let k24 = key + Array(key.prefix(8))
    return ccCrypt(op: kCCEncrypt, alg: kCCAlgorithm3DES, opts: kCCOptionECBMode, key: k24, iv: nil, data: data)
}

func des3CBC(_ key: [UInt8], _ iv: [UInt8], _ data: [UInt8]) -> [UInt8]? {
    let k24 = key + Array(key.prefix(8))
    return ccCrypt(op: kCCEncrypt, alg: kCCAlgorithm3DES, opts: 0, key: k24, iv: iv, data: data)
}

func desCBC(_ key: [UInt8], _ iv: [UInt8], _ data: [UInt8]) -> [UInt8]? {
    ccCrypt(op: kCCEncrypt, alg: kCCAlgorithmDES, opts: 0, key: key, iv: iv, data: data)
}

func desDecCBC(_ key: [UInt8], _ iv: [UInt8], _ data: [UInt8]) -> [UInt8]? {
    ccCrypt(op: kCCDecrypt, alg: kCCAlgorithmDES, opts: 0, key: key, iv: iv, data: data)
}

func retailMAC(_ key: [UInt8], _ iv: [UInt8], _ data: [UInt8]) -> [UInt8]? {
    let k1 = Array(key.prefix(8)), k2 = Array(key.suffix(8))
    guard let enc = desCBC(k1, iv, data) else { return nil }
    let last8 = Array(enc.suffix(8))
    let z8 = [UInt8](repeating: 0, count: 8)
    guard let dec = desDecCBC(k2, z8, last8) else { return nil }
    guard let mac = desCBC(k1, z8, Array(dec.prefix(8))) else { return nil }
    return Array(mac.prefix(8))
}

// MARK: - Main

print("=== BioPass2003 Token Reset Tool ===\n")

let transportKey: [UInt8] = [0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,
                              0x09,0x0A,0x0B,0x0C,0x0D,0x0E,0x0F,0x10]

// Connect
var ctx: SCARDCONTEXT = 0
guard SCardEstablishContext(2, nil, nil, &ctx) == 0 else { print("❌ PCSC context"); exit(1) }

var rLen: DWORD = 0
_ = SCardListReaders(ctx, nil, nil, &rLen)
var rBuf = [CChar](repeating: 0, count: Int(rLen))
guard SCardListReaders(ctx, nil, &rBuf, &rLen) == 0 else { print("❌ Reader олдсонгүй"); exit(1) }
let reader = String(cString: rBuf)
print("✅ Reader: \(reader)")

var card: SCARDHANDLE = 0
var proto: DWORD = 0
guard SCardConnect(ctx, reader, 2, 3, &card, &proto) == 0 else { print("❌ Connect"); exit(1) }
print("✅ Холбогдлоо")

// Step 1: GET DATA 0x86 (detect FIPS mode)
print("\n--- Step 1: GET DATA (FIPS detect) ---")
if let r = send(card, [0x80, 0xCA, 0x00, 0x86, 0x00]) {
    print("GET DATA: SW=\(String(format: "%02X%02X", r.sw1, r.sw2)) data=\(r.data.count)B")
    if r.sw1 == 0x90 {
        print("✅ Card харилцаж байна!")
        let isFIPS = r.data.count > 34 && r.data[32] == 0x87 && r.data[33] == 0x01 && r.data[34] == 0x01
        let isAES = r.data.count > 2 && r.data[2] == 0x01
        print("   FIPS: \(isFIPS), KeyType: \(isAES ? "AES" : "3DES")")
    } else {
        print("⚠️ GET DATA failed — токен lock төлөвт байна")
        print("   Reconnect оролдож байна...")

        // Reconnect with reset
        _ = SCardReconnect(card, 2, 3, 1, &proto) // SCARD_RESET_CARD = 1
        print("   Reconnect хийгдлэ")

        if let r2 = send(card, [0x80, 0xCA, 0x00, 0x86, 0x00]) {
            print("   Retry GET DATA: SW=\(String(format: "%02X%02X", r2.sw1, r2.sw2))")
            if r2.sw1 != 0x90 {
                print("\n❌ Токен lock болсон — mutual auth-гүйгээр хандах боломжгүй.")
                print("   FEITIAN ePassManagerAdm_2003.exe (Windows) ашиглан reset хийнэ.")
                _ = SCardDisconnect(card, 0)
                _ = SCardReleaseContext(ctx)
                exit(1)
            }
        }
    }
}

// Step 2: Host random
print("\n--- Step 2: Mutual Auth ---")
var hostRandom = [UInt8](repeating: 0, count: 8)
_ = SecRandomCopyBytes(kSecRandomDefault, 8, &hostRandom)
print("Host random: \(hex(hostRandom))")

// Step 3: INITIALIZE UPDATE (CLA=80 INS=50)
var initAPDU: [UInt8] = [0x80, 0x50, 0x00, 0x00, 0x08] + hostRandom + [0x1C] // Le=28 (non-FIPS)
if let r = send(card, initAPDU) {
    print("INIT UPDATE: SW=\(String(format: "%02X%02X", r.sw1, r.sw2)) data=\(r.data.count)B")

    if r.sw1 == 0x90 && r.data.count >= 28 {
        let result = r.data
        let cardRandom = Array(result[12..<20])
        print("Card random: \(hex(cardRandom))")

        // Derive session keys
        var deriv = [UInt8](repeating: 0, count: 16)
        deriv.replaceSubrange(0..<4, with: result[16..<20])
        deriv.replaceSubrange(4..<8, with: hostRandom[0..<4])
        deriv.replaceSubrange(8..<12, with: result[12..<16])
        deriv.replaceSubrange(12..<16, with: hostRandom[4..<8])

        guard let skEnc = des3ECB(transportKey, deriv),
              let skMac = des3ECB(transportKey, deriv) else {
            print("❌ Session key derive failed"); exit(1)
        }
        print("✅ Session keys derived")

        // Host cryptogram
        var cryptInput = [UInt8](repeating: 0, count: 24)
        cryptInput.replaceSubrange(0..<8, with: hostRandom)
        cryptInput.replaceSubrange(8..<16, with: cardRandom)
        cryptInput[16] = 0x80
        let iv0 = [UInt8](repeating: 0, count: 8)

        guard let cryptogram = des3CBC(skEnc, iv0, cryptInput) else {
            print("❌ Cryptogram failed"); exit(1)
        }

        // Verify card cryptogram
        let cardCrypt = Array(result[20..<28])
        let expectedCrypt = Array(cryptogram[16..<24])
        if cardCrypt != expectedCrypt {
            print("❌ Card cryptogram буруу — transport key буруу байна")
            _ = SCardDisconnect(card, 0); _ = SCardReleaseContext(ctx); exit(1)
        }
        print("✅ Card cryptogram verified!")

        let hostCrypt = Array(cryptogram[16..<24])

        // Build EXTERNAL AUTHENTICATE
        var macInput = [UInt8](repeating: 0, count: 16)
        macInput[0] = 0x84; macInput[1] = 0x82; macInput[2] = 0x03; macInput[3] = 0x00; macInput[4] = 0x10
        macInput.replaceSubrange(5..<13, with: hostCrypt)
        macInput[13] = 0x80

        guard let mac = retailMAC(skMac, iv0, macInput) else {
            print("❌ MAC failed"); exit(1)
        }

        var extAuth: [UInt8] = [0x84, 0x82, 0x03, 0x00, 0x10]
        extAuth.append(contentsOf: hostCrypt)
        extAuth.append(contentsOf: mac)

        if let r2 = send(card, extAuth) {
            print("EXT AUTH: SW=\(String(format: "%02X%02X", r2.sw1, r2.sw2))")
            if r2.sw1 == 0x90 {
                print("\n🎉 MUTUAL AUTH АМЖИЛТТАЙ!")
                print("   Session key established — SM mode идэвхжлээ")
                print("   Токеныг удирдах боломжтой боллоо!")

                // TODO: SM-ээр wrapped APDU илгээж file structure үүсгэх
                print("\n   Дараагийн алхам: SM APDU-аар PKCS#15 file structure үүсгэнэ")
            } else {
                print("❌ External auth failed")
            }
        }
    } else {
        print("❌ INIT UPDATE failed — mutual auth боломжгүй")
        print("   Windows ePassManagerAdm_2003.exe ашиглана уу.")
    }
} else {
    print("❌ INIT UPDATE transmit failed")
}

_ = SCardDisconnect(card, 0)
_ = SCardReleaseContext(ctx)
print("\n=== Дууслаа ===")
