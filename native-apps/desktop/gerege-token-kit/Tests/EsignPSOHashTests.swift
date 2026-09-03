import XCTest
import CryptoKit
@testable import GeregeTokenKit

final class EsignPSOHashTests: XCTestCase {

    // Дотоод SHA-256 нь CryptoKit-тэй яг таарах ёстой (завсрын төлөв зөв гэдгийн баталгаа).
    func testSha256CoreMatchesCryptoKit() {
        for len in [1, 55, 56, 63, 64, 65, 127, 128, 2816, 2857, 4000] {
            let m = (0..<len).map { UInt8(($0 * 31 + 7) & 0xFF) }
            XCTAssertEqual(EsignPSOHash.sha256(m), Array(SHA256.hash(data: Data(m))),
                           "sha256 зөрлөө len=\(len)")
        }
    }

    // Трэйсийн PSO HASH бүтэц: 81 04 <блок> 90 20 <мид> 80 <r> <tail>.
    // Windows нэвтрэлт: 2857 байт P → 44 блок (0x2C) + 41 байт сүүл.
    func testTLVStructureAndBlockCount() {
        let msg = (0..<2857).map { UInt8(($0 * 131 + 17) & 0xFF) }
        let tlv = EsignPSOHash.hashTLV(for: msg)
        XCTAssertEqual(Array(tlv[0..<6]), [0x81, 0x04, 0x00, 0x00, 0x00, 0x2C]) // 44 блок
        XCTAssertEqual(Array(tlv[6..<8]), [0x90, 0x20])
        XCTAssertEqual(tlv[40], 0x80)
        XCTAssertEqual(Int(tlv[41]), 41)          // сүүл = 2857 - 44*64
        XCTAssertEqual(tlv.count, 6 + 34 + 2 + 41)
    }

    // Бүх байт TLV-д хамрагдана: блок·64 + сүүл = урт (картны finalize энэ инвариант дээр
    // тулгуурлаж SHA256(P)-г гаргана; compression нь CryptoKit-тэй ижил тул мид-төлөв жинхэнэ).
    func testTLVCoversEveryByte() {
        for len in [65, 200, 2816, 2857, 4001] {
            let tlv = EsignPSOHash.hashTLV(for: (0..<len).map { UInt8(($0 * 17 + 3) & 0xFF) })
            let blocks = Int(tlv[2]) << 24 | Int(tlv[3]) << 16 | Int(tlv[4]) << 8 | Int(tlv[5])
            XCTAssertEqual(blocks * 64 + Int(tlv[41]), len, "хамрах алдаа len=\(len)")
        }
    }

    // Регресс: `90` DO нь SHA-256-ийн ГАНЦ БЛОКИЙН дараах жинхэнэ state (big-endian words).
    // Байтын дараалал урвуулбал (өмнөх алдаа) карт өөр digest зурна. "A"×64-ийн эталон
    // midstate-ийг тогтмолоор пинлэнэ.
    func testMidstateByteOrderIsCanonical() {
        let msg = [UInt8](repeating: 0x41, count: 64) + Array("hello".utf8) // 1 блок + 5B сүүл
        let tlv = EsignPSOHash.hashTLV(for: msg)
        let mid = Array(tlv[8..<40]).map { String(format: "%02X", $0) }.joined()
        XCTAssertEqual(mid, "6CB7244102F65790145141E105D2C3D387A1D91E9CEDA13A4A3D25DD87BEEA05")
    }

    // Урт 64-д хуваагдах ирмэг: сүүл заавал 64 байт, блок нэгээр багасна.
    func testExactBlockMultipleKeepsFullTail() {
        let msg = (0..<128).map { UInt8($0) }        // яг 2 блок
        let tlv = EsignPSOHash.hashTLV(for: msg)
        XCTAssertEqual(Array(tlv[0..<6]), [0x81, 0x04, 0x00, 0x00, 0x00, 0x01]) // 1 блок
        XCTAssertEqual(Int(tlv[41]), 64)             // сүүл = бүтэн блок
    }

}
