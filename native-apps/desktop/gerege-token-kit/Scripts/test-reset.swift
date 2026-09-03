#!/usr/bin/env swift

// BioPass2003 токен руу PCSC-ээр шууд APDU илгээх тест.
// TKSmartCardSlotManager entitlement хэрэггүй.

import Foundation

// PCSC framework
typealias SCARDCONTEXT = UInt
typealias SCARDHANDLE = UInt
typealias DWORD = UInt32

@_silgen_name("SCardEstablishContext")
func SCardEstablishContext(_ dwScope: DWORD, _ pvReserved1: UnsafeRawPointer?, _ pvReserved2: UnsafeRawPointer?, _ phContext: UnsafeMutablePointer<SCARDCONTEXT>) -> Int32

@_silgen_name("SCardListReaders")
func SCardListReaders(_ hContext: SCARDCONTEXT, _ mszGroups: UnsafePointer<CChar>?, _ mszReaders: UnsafeMutablePointer<CChar>?, _ pcchReaders: UnsafeMutablePointer<DWORD>) -> Int32

@_silgen_name("SCardConnect")
func SCardConnect(_ hContext: SCARDCONTEXT, _ szReader: UnsafePointer<CChar>, _ dwShareMode: DWORD, _ dwPreferredProtocols: DWORD, _ phCard: UnsafeMutablePointer<SCARDHANDLE>, _ pdwActiveProtocol: UnsafeMutablePointer<DWORD>) -> Int32

@_silgen_name("SCardTransmit")
func SCardTransmit(_ hCard: SCARDHANDLE, _ pioSendPci: UnsafeRawPointer, _ pbSendBuffer: UnsafePointer<UInt8>, _ cbSendLength: DWORD, _ pioRecvPci: UnsafeMutableRawPointer?, _ pbRecvBuffer: UnsafeMutablePointer<UInt8>, _ pcbRecvLength: UnsafeMutablePointer<DWORD>) -> Int32

@_silgen_name("SCardDisconnect")
func SCardDisconnect(_ hCard: SCARDHANDLE, _ dwDisposition: DWORD) -> Int32

@_silgen_name("SCardReleaseContext")
func SCardReleaseContext(_ hContext: SCARDCONTEXT) -> Int32

// PCSC constants
let SCARD_SCOPE_SYSTEM: DWORD = 2
let SCARD_SHARE_SHARED: DWORD = 2
let SCARD_PROTOCOL_T0: DWORD = 1
let SCARD_PROTOCOL_T1: DWORD = 2
let SCARD_LEAVE_CARD: DWORD = 0

// T=1 protocol PCI
struct SCARD_IO_REQUEST {
    var dwProtocol: DWORD
    var cbPciLength: DWORD
}
var g_pioSendPciT1 = SCARD_IO_REQUEST(dwProtocol: SCARD_PROTOCOL_T1, cbPciLength: 8)

func hexStr(_ bytes: [UInt8]) -> String {
    bytes.map { String(format: "%02X", $0) }.joined(separator: " ")
}

func transmit(_ card: SCARDHANDLE, apdu: [UInt8]) -> (data: [UInt8], sw1: UInt8, sw2: UInt8)? {
    var recvBuffer = [UInt8](repeating: 0, count: 258)
    var recvLength: DWORD = DWORD(recvBuffer.count)

    let result = apdu.withUnsafeBufferPointer { sendBuf in
        withUnsafeMutablePointer(to: &g_pioSendPciT1) { pci in
            SCardTransmit(card, pci, sendBuf.baseAddress!, DWORD(apdu.count), nil, &recvBuffer, &recvLength)
        }
    }

    guard result == 0, recvLength >= 2 else {
        print("   SCardTransmit алдаа: \(result)")
        return nil
    }

    let len = Int(recvLength)
    let sw1 = recvBuffer[len - 2]
    let sw2 = recvBuffer[len - 1]
    let data = len > 2 ? Array(recvBuffer[0..<len-2]) : []
    return (data, sw1, sw2)
}

// ======== MAIN ========

print("=== BioPass2003 Token Reset Test ===\n")

// 1. Establish context
var context: SCARDCONTEXT = 0
var rv = SCardEstablishContext(SCARD_SCOPE_SYSTEM, nil, nil, &context)
guard rv == 0 else { print("❌ SCardEstablishContext: \(rv)"); exit(1) }

// 2. List readers
var readersBufLen: DWORD = 0
SCardListReaders(context, nil, nil, &readersBufLen)
var readersBuf = [CChar](repeating: 0, count: Int(readersBufLen))
rv = SCardListReaders(context, nil, &readersBuf, &readersBufLen)
guard rv == 0 else { print("❌ Reader олдсонгүй"); exit(1) }
let readerName = String(cString: readersBuf)
print("✅ Reader: \(readerName)")

// 3. Connect
var card: SCARDHANDLE = 0
var activeProtocol: DWORD = 0
rv = SCardConnect(context, readerName, SCARD_SHARE_SHARED,
                  SCARD_PROTOCOL_T0 | SCARD_PROTOCOL_T1, &card, &activeProtocol)
guard rv == 0 else { print("❌ SCardConnect: \(rv)"); exit(1) }
print("✅ Холбогдлоо (protocol: T=\(activeProtocol == SCARD_PROTOCOL_T1 ? "1" : "0"))")

// 4. SELECT applet
let selectAPDU: [UInt8] = [
    0x00, 0xA4, 0x04, 0x00, 0x0E,
    0x65, 0x6E, 0x74, 0x65, 0x72, 0x73, 0x61, 0x66, 0x65, 0x2D, 0x66, 0x69, 0x70, 0x73
]
if let resp = transmit(card, apdu: selectAPDU) {
    print("✅ SELECT applet: SW=\(String(format: "%02X%02X", resp.sw1, resp.sw2))")
    if resp.sw1 == 0x90 {
        print("   Applet амжилттай!")
    } else {
        print("   ⚠️ Applet сонгогдсонгүй")
    }
} else {
    print("❌ SELECT transmit алдаа")
}

// 5. GET CHALLENGE
let chalAPDU: [UInt8] = [0x00, 0x84, 0x00, 0x00, 0x08]
if let resp = transmit(card, apdu: chalAPDU) {
    print("✅ GET CHALLENGE: SW=\(String(format: "%02X%02X", resp.sw1, resp.sw2))")
    if resp.sw1 == 0x90 {
        print("   Challenge: \(hexStr(resp.data))")
        print("   ✅ Токен хариу өгч байна — reset хийх боломжтой!")
    } else {
        print("   ⚠️ Challenge авч чадсангүй — токен lock байж магадгүй")
    }
}

// 6. Try VERIFY with default PIN (check if factory PIN still works)
// Default ePass2003 user PIN is usually "12345678" or "1234567890123456"
let verifyAPDU: [UInt8] = [
    0x00, 0x20, 0x00, 0x01, 0x10,  // VERIFY, P2=01 (SO PIN), Lc=16
    0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38,  // "12345678"
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00   // padding
]
if let resp = transmit(card, apdu: verifyAPDU) {
    print("\n📌 VERIFY SO-PIN (12345678): SW=\(String(format: "%02X%02X", resp.sw1, resp.sw2))")
    if resp.sw1 == 0x90 {
        print("   ✅ SO-PIN зөв! Token удирдах боломжтой!")
    } else if resp.sw1 == 0x63 {
        let retries = resp.sw2 & 0x0F
        print("   ❌ PIN буруу. \(retries) оролдлого үлдсэн.")
    } else if resp.sw1 == 0x69 && resp.sw2 == 0x83 {
        print("   🔒 PIN блоклогдсон!")
    } else if resp.sw1 == 0x69 && resp.sw2 == 0x85 {
        print("   ⚠️ Conditions not satisfied — mutual auth хэрэгтэй")
    }
}

// 7. Disconnect
SCardDisconnect(card, SCARD_LEAVE_CARD)
SCardReleaseContext(context)

print("\n=== Тест дууслаа ===")
