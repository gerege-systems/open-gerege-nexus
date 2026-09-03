#!/usr/bin/env swift

// GeregeTokenKit-ийн token connection тест.
// Ажиллуулах: cd gerege-token-kit && swift Scripts/test-token.swift

import Foundation
import CryptoTokenKit

print("=== GeregeTokenKit Token Test ===\n")

// 1. Reader шалгах
let manager = TKSmartCardSlotManager.default
guard let slotNames = manager?.slotNames, !slotNames.isEmpty else {
    print("❌ Уншигч олдсонгүй. USB токен залгагдсан эсэхийг шалгана уу.")
    exit(1)
}

print("✅ Reader олдлоо: \(slotNames)")

// 2. Card connect
let semaphore = DispatchSemaphore(value: 0)

Task {
    guard let slot = await manager?.getSlot(withName: slotNames[0]) else {
        print("❌ Slot авахад алдаа")
        semaphore.signal()
        return
    }

    guard let card = slot.makeSmartCard() else {
        print("❌ SmartCard үүсгэхэд алдаа")
        semaphore.signal()
        return
    }

    do {
        try await card.beginSession()
        print("✅ Session нээгдлээ")

        // ATR
        if let atr = card.slot.atr {
            let atrHex = atr.bytes.map { String(format: "%02X", $0) }.joined(separator: ":")
            print("✅ ATR: \(atrHex)")
        }

        // SELECT applet: 00 A4 04 00 0E 656E746572736166652D66697073
        let selectAPDU: [UInt8] = [
            0x00, 0xA4, 0x04, 0x00, 0x0E,
            0x65, 0x6E, 0x74, 0x65, 0x72, 0x73, 0x61, 0x66, 0x65, 0x2D, 0x66, 0x69, 0x70, 0x73
        ]
        let selectResp = try await card.transmit(Data(selectAPDU))
        let selectBytes = Array(selectResp)
        let sw1 = selectBytes[selectBytes.count - 2]
        let sw2 = selectBytes[selectBytes.count - 1]
        print("✅ SELECT applet: SW=\(String(format: "%02X%02X", sw1, sw2))")

        if sw1 == 0x90 && sw2 == 0x00 {
            print("   Applet амжилттай сонгогдлоо!")

            // GET CHALLENGE: 00 84 00 00 08
            let challengeAPDU: [UInt8] = [0x00, 0x84, 0x00, 0x00, 0x08]
            let chalResp = try await card.transmit(Data(challengeAPDU))
            let chalBytes = Array(chalResp)
            let csw1 = chalBytes[chalBytes.count - 2]
            let csw2 = chalBytes[chalBytes.count - 1]
            print("✅ GET CHALLENGE: SW=\(String(format: "%02X%02X", csw1, csw2))")

            if csw1 == 0x90 && csw2 == 0x00 && chalBytes.count > 2 {
                let challenge = Array(chalBytes.prefix(chalBytes.count - 2))
                let chalHex = challenge.map { String(format: "%02X", $0) }.joined()
                print("   Challenge: \(chalHex)")
                print("\n🔑 Mutual auth хийхэд бэлэн! (3DES encrypt хэрэгтэй)")
            }
        } else if sw1 == 0x69 && sw2 == 0x85 {
            print("   ⚠️ SW=6985 — Conditions not satisfied (token locked?)")
        }

        card.endSession()
        print("\n✅ Session хаагдлаа")

    } catch {
        print("❌ Алдаа: \(error)")
    }

    semaphore.signal()
}

semaphore.wait()
print("\n=== Тест дууслаа ===")
