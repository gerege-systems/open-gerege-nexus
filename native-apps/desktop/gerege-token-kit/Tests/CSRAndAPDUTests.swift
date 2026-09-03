import XCTest
import CryptoKit
@testable import GeregeTokenKit

/// APDU builder/response parsing болон CSR (PKCS#10) DER encoding-ийн цэвэр логикийг шалгана —
/// картны төхөөрөмжгүйгээр туршиж болох бүх хэсэг.
final class CSRAndAPDUTests: XCTestCase {

    // MARK: - APDUCommand builder

    func testBuildBufferHeaderOnly() throws {
        let cmd = try APDUCommand(cla: 0x00, ins: 0xA4, p1: 0x04, p2: 0x00)
        XCTAssertEqual(cmd.buildBuffer(), [0x00, 0xA4, 0x04, 0x00])
        XCTAssertEqual(cmd.lc, 0)
    }

    func testBuildBufferLeOnly() throws {
        let cmd = try APDUCommand(cla: 0x00, ins: 0x84, p1: 0x00, p2: 0x00, le: 0x08)
        XCTAssertEqual(cmd.buildBuffer(), [0x00, 0x84, 0x00, 0x00, 0x08])
        XCTAssertNil(cmd.payload)
    }

    func testBuildBufferDataPlusLe() throws {
        let cmd = try APDUCommand(cla: 0x00, ins: 0x2A, p1: 0x9E, p2: 0x9A,
                                  data: [0xAA, 0xBB], le: 0x10)
        let buf = cmd.buildBuffer()
        XCTAssertEqual(buf, [0x00, 0x2A, 0x9E, 0x9A, 0x02, 0xAA, 0xBB, 0x10])
        XCTAssertEqual(cmd.lc, 2)
    }

    func testDataTooLongThrows() {
        XCTAssertThrowsError(try APDUCommand(cla: 0, ins: 0, p1: 0, p2: 0,
                                             data: [UInt8](repeating: 0, count: 256)))
    }

    func testHexInitLeOnly() throws {
        let cmd = try APDUCommand(hex: "0084000008")
        XCTAssertEqual(cmd.le, 0x08)
        XCTAssertNil(cmd.payload)
        XCTAssertEqual(cmd.lc, 0)
    }

    func testHexInitHeaderOnly() throws {
        let cmd = try APDUCommand(hex: "00A40400")
        XCTAssertEqual(cmd.ins, 0xA4)
        XCTAssertNil(cmd.payload)
        XCTAssertEqual(cmd.le, 0)
    }

    func testHexInitDataPlusLe() throws {
        // 4-байт header + Lc=02 + 2 data + Le
        let cmd = try APDUCommand(hex: "00A4040002AABB10")
        XCTAssertEqual(cmd.payload ?? [], [0xAA, 0xBB])
        XCTAssertEqual(cmd.le, 0x10)
    }

    func testHexInitTooShortThrows() {
        XCTAssertThrowsError(try APDUCommand(hex: "0011"))
    }

    func testHexInitInconsistentLengthThrows() {
        // Lc=02 гэж хэлсэн ч зөвхөн 1 data байт → invalidAPDU.
        XCTAssertThrowsError(try APDUCommand(hex: "00A4040002AA"))
    }

    func testRedactedDescriptionHidesPayload() throws {
        let secret: [UInt8] = [0x31, 0x32, 0x33, 0x34]  // "1234" PIN
        let cmd = try APDUCommand(cla: 0x00, ins: 0x20, p1: 0x00, p2: 0x02, data: secret)
        let desc = cmd.redactedDescription()
        XCTAssertTrue(desc.contains("redacted"), "payload log-д ил гарах ёсгүй")
        XCTAssertFalse(desc.contains("31323334"), "PIN байт ил гарахгүй")
        XCTAssertTrue(desc.contains("INS=20"))
    }

    // MARK: - APDUResponse parsing

    func testResponseStatusOnlyNoData() {
        let resp = APDUResponse(fullData: [0x90, 0x00])
        XCTAssertTrue(resp.isSuccess)
        XCTAssertNil(resp.data)
        XCTAssertEqual(resp.status, 0x9000)
    }

    func testResponseTooShortIsError() {
        // 2 байтаас бага → 6F00 (internal error), data nil.
        let resp = APDUResponse(fullData: [0x01])
        XCTAssertFalse(resp.isSuccess)
        XCTAssertEqual(resp.hexStatus, "6F00")
        XCTAssertNil(resp.data)
    }

    func testResponseFromUnwrappedFields() {
        let resp = APDUResponse(data: [0xDE, 0xAD], sw1: 0x90, sw2: 0x00)
        XCTAssertTrue(resp.isSuccess)
        XCTAssertEqual(resp.data ?? [], [0xDE, 0xAD])
        XCTAssertEqual(resp.description, "APDUResponse(SW=9000, data=<2B>)")
    }

    func testResponseWipeClearsData() {
        let resp = APDUResponse(fullData: [0xAA, 0xBB, 0x90, 0x00])
        XCTAssertNotNil(resp.data)
        resp.wipe()
        XCTAssertNil(resp.data)
    }

    // MARK: - Hex helpers (нэмэлт)

    func testHexToBytesWhitespaceAndOddLength() {
        XCTAssertEqual(hexToBytes("00 A4\t04\n00"), [0x00, 0xA4, 0x04, 0x00])
        XCTAssertEqual(hexToBytes("ABC"), [], "сондгой урт → хоосон")
        XCTAssertEqual(hexToBytes("XYZ1"), [], "hex бус тэмдэгт → хоосон")
    }

    func testHexRoundTrip() {
        let bytes: [UInt8] = [0x00, 0xA4, 0xFF, 0x10, 0x7F]
        XCTAssertEqual(hexToBytes(bytesToHex(bytes)), bytes)
    }

    func testConstantTimeEqualsEmpty() {
        XCTAssertTrue(constantTimeEquals([], []))
        XCTAssertFalse(constantTimeEquals([], [0x00]))
    }

    // MARK: - CSR DER primitives

    func testDERLengthEncoding() {
        XCTAssertEqual(CSR.derLength(5), [0x05])
        XCTAssertEqual(CSR.derLength(0x7F), [0x7F])
        XCTAssertEqual(CSR.derLength(0x80), [0x81, 0x80])
        XCTAssertEqual(CSR.derLength(0xFF), [0x81, 0xFF])
        XCTAssertEqual(CSR.derLength(0x0100), [0x82, 0x01, 0x00])
        XCTAssertEqual(CSR.derLength(0xFFFF), [0x82, 0xFF, 0xFF])
        XCTAssertEqual(CSR.derLength(0x010000), [0x83, 0x01, 0x00, 0x00])
    }

    func testDERIntegerTrimmingAndSignPadding() {
        XCTAssertEqual(CSR.derInteger([0x00, 0x00, 0x05]), [0x02, 0x01, 0x05], "leading zero-г устгана")
        XCTAssertEqual(CSR.derInteger([0x80]), [0x02, 0x02, 0x00, 0x80], "MSB set → 0x00 prepend")
        XCTAssertEqual(CSR.derInteger([0x00]), [0x02, 0x01, 0x00], "тэг → 0x00")
        XCTAssertEqual(CSR.derInteger([0xFF, 0x01]), [0x02, 0x03, 0x00, 0xFF, 0x01])
        XCTAssertEqual(CSR.derInteger([0x7F, 0x01]), [0x02, 0x02, 0x7F, 0x01], "MSB цэвэр → prepend үгүй")
    }

    func testDERSequenceOIDBitString() {
        XCTAssertEqual(CSR.derSequence([0x01, 0x02]), [0x30, 0x02, 0x01, 0x02])
        XCTAssertEqual(CSR.derOID([0x55, 0x04, 0x03]), [0x06, 0x03, 0x55, 0x04, 0x03])
        XCTAssertEqual(CSR.derBitString([0xAA]), [0x03, 0x02, 0x00, 0xAA], "BIT STRING нь 0x00 unused-bits prefix-тэй")
    }

    func testEncodeECDSASigDER() {
        var rs = [UInt8](repeating: 0, count: 64)
        rs[31] = 0x01   // r = 1
        rs[63] = 0x02   // s = 2
        XCTAssertEqual(CSR.encodeECDSASigDER(rs: rs),
                       [0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02])
    }

    func testEncodeECDSASigDERHighBitPadsSign() {
        var rs = [UInt8](repeating: 0, count: 64)
        rs[0] = 0x80    // r-ийн MSB set → 0x00 prepend (33 байт INTEGER)
        rs[63] = 0x01   // s = 1
        let der = CSR.encodeECDSASigDER(rs: rs)
        XCTAssertEqual(der[0], 0x30)
        XCTAssertEqual(der[2], 0x02)          // r INTEGER
        XCTAssertEqual(der[3], 0x21)          // урт 33 (sign padding)
        XCTAssertEqual(der[4], 0x00)          // sign байт
    }

    func testPEMWrapFormatAnd64ColumnLines() {
        let der = [UInt8](repeating: 0x41, count: 100)  // → base64 ~136 тэмдэгт
        let pem = CSR.pemWrap(der, label: "CERTIFICATE REQUEST")
        XCTAssertTrue(pem.hasPrefix("-----BEGIN CERTIFICATE REQUEST-----\n"))
        XCTAssertTrue(pem.hasSuffix("-----END CERTIFICATE REQUEST-----\n"))
        let body = pem.split(separator: "\n").filter { !$0.hasPrefix("-----") }
        for line in body.dropLast() {
            XCTAssertEqual(line.count, 64, "сүүлчээс бусад мөр 64 тэмдэгт байх ёстой")
        }
    }

    func testRDNEncoding() {
        let rdn = CSR.rdn(oid: CSR.oidCN, value: "AB", asString: .utf8)
        // SET(0x31){ SEQ(0x30){ OID(2.5.4.3), UTF8String("AB") } }
        XCTAssertEqual(rdn[0], 0x31, "RDN нь SET")
        XCTAssertTrue(rdn.contains(0x0C), "UTF8String tag 0x0C агуулна")
        // "AB" = 0x41 0x42
        XCTAssertTrue(rdn.contains(0x41) && rdn.contains(0x42))
    }

    // MARK: - CSR full build

    func testBuildP256RejectsInvalidPublicKey() async {
        do {
            _ = try await CSR.buildP256(subject: .init(commonName: "x"),
                                        publicKeyPoint: [0x04, 0x01, 0x02],  // 65 байт биш
                                        signer: { _ in [UInt8](repeating: 0, count: 64) })
            XCTFail("богино public key-д алдаа шидэх ёстой")
        } catch let e as CSRError {
            if case .invalidPublicKey = e {} else { XCTFail("invalidPublicKey хүлээсэн, \(e) ирлээ") }
        } catch { XCTFail("CSRError хүлээсэн, \(error) ирлээ") }
    }

    func testBuildP256ProducesValidStructure() async throws {
        // Жинхэнэ P-256 түлхүүр → uncompressed цэг; signer нь SHA-256(CRI)-г ECDSA-аар зурна.
        let priv = P256.Signing.PrivateKey()
        let pubPoint = [UInt8](priv.publicKey.x963Representation)  // 04 || X || Y (65 байт)
        XCTAssertEqual(pubPoint.count, 65)

        var capturedHash: [UInt8] = []
        let (der, pem) = try await CSR.buildP256(
            subject: .init(commonName: "Test User", organization: "Gerege", country: "MN"),
            publicKeyPoint: pubPoint,
            signer: { hash in
                capturedHash = hash
                XCTAssertEqual(hash.count, 32, "signer нь 32-байт SHA-256 digest авах ёстой")
                // r||s (64 байт) буцаах — buildP256 үүнийг DER болгоно.
                var sig = [UInt8](repeating: 0, count: 64)
                sig[31] = 0x09; sig[63] = 0x11
                return sig
            })

        XCTAssertEqual(der.first, 0x30, "CSR нь SEQUENCE-ээр эхэлнэ")
        XCTAssertEqual(capturedHash.count, 32)
        // Public key цэг DER дотор бүтнээрээ орсон байх ёстой.
        XCTAssertTrue(der.contains(subarray: pubPoint), "SPKI дотор pubkey цэг байх ёстой")
        XCTAssertTrue(pem.contains("BEGIN CERTIFICATE REQUEST"))
        XCTAssertTrue(pem.contains("END CERTIFICATE REQUEST"))
    }

    func testBuildP256PassesThroughDERSignature() async throws {
        // signer 64 байт биш (DER гэж үзнэ) буцаавал шууд ашиглана.
        let priv = P256.Signing.PrivateKey()
        let pubPoint = [UInt8](priv.publicKey.x963Representation)
        let derSig: [UInt8] = [0x30, 0x06, 0x02, 0x01, 0x05, 0x02, 0x01, 0x06]  // 8 байт (≠64)
        let (der, _) = try await CSR.buildP256(subject: .init(commonName: "x"),
                                               publicKeyPoint: pubPoint,
                                               signer: { _ in derSig })
        XCTAssertTrue(der.contains(subarray: derSig), "DER гарын үсэг шууд дамжина")
    }
}

private extension Array where Element == UInt8 {
    /// `sub`-г бүтнээр агуулж байгаа эсэх (subsequence хайлт).
    func contains(subarray sub: [UInt8]) -> Bool {
        guard !sub.isEmpty, count >= sub.count else { return false }
        for start in 0...(count - sub.count) where Array(self[start..<start + sub.count]) == sub {
            return true
        }
        return false
    }
}

// MARK: - ESPK гэрчилгээ сугалах (bare-metal тест — карт шаардахгүй)

final class ESPKCertificateExtractionTests: XCTestCase {

    /// Бодит картан дээр EF-ийн эхэнд хувьсах урттай толгой байдаг (24C0 → 87 байт,
    /// 24E0 → 58) тул DER-ийг эхлэлээс нь БИШ, `30 82` тэмдгээс нь олох ёстой.
    func testExtractsDERAfterVariableHeader() {
        let body: [UInt8] = [0x30, 0x82, 0x00, 0x64, 0x30] + [UInt8](repeating: 0xAB, count: 99)
        for headerLen in [0, 1, 58, 87] {
            let raw = [UInt8](repeating: 0x00, count: headerLen) + body + [UInt8](repeating: 0xFF, count: 32)
            let der = BioPassDriver.extractDER(raw)
            XCTAssertEqual(der, body, "толгой \(headerLen) байт дээр DER буруу сугаллаа")
        }
    }

    /// Зарласан уртаас илүү уншихгүй — дүүргэлт (0xFF) DER-т орж болохгүй.
    func testTrimsTrailingPadding() {
        let body: [UInt8] = [0x30, 0x82, 0x00, 0x50, 0x30] + [UInt8](repeating: 0x11, count: 79)
        let raw = body + [UInt8](repeating: 0xFF, count: 400)
        XCTAssertEqual(BioPassDriver.extractDER(raw)?.count, 84)
    }

    /// EF-ийн толгойд санамсаргүй тохиосон `30 82` дээр зогсохгүй — X.509 нь SEQUENCE
    /// дотор SEQUENCE (tbsCertificate) агуулдаг тул түүгээр ялгана. (Бодит жишээ:
    /// EF 24E0-ийн толгой доторх хуурамч тэмдэг 828 байтын хог өгдөг байв.)
    func testSkipsDecoySequenceInHeader() {
        let decoy: [UInt8] = [0x30, 0x82, 0x00, 0x64, 0x02] + [UInt8](repeating: 0x00, count: 99)
        let real: [UInt8] = [0x30, 0x82, 0x00, 0x64, 0x30] + [UInt8](repeating: 0xAB, count: 99)
        XCTAssertEqual(BioPassDriver.extractDER(decoy + real), real)
    }

    /// Гэрчилгээгүй EF (метадата, хоосон, хэт богино) → nil.
    func testRejectsNonCertificateContent() {
        XCTAssertNil(BioPassDriver.extractDER([]))
        XCTAssertNil(BioPassDriver.extractDER([UInt8](repeating: 0x00, count: 64)))
        // 30 82 бий ч зарласан урт нь EF-ээс хэтэрсэн — таслах боломжгүй.
        XCTAssertNil(BioPassDriver.extractDER([0x30, 0x82, 0x10, 0x00, 0x01, 0x02]))
        // Хэт богино DER (64 байтаас бага) — гэрчилгээ байх боломжгүй.
        XCTAssertNil(BioPassDriver.extractDER([0x30, 0x82, 0x00, 0x04, 0x30, 2, 3, 4]))
    }
}

// MARK: - PIN external-auth түлхүүр (бодит Windows PC/SC трэйсээс задлан баталгаажуулав)

final class ExternalAuthKeyTests: XCTestCase {

    /// Бодит нэвтрэлтийн трэйс: session key сэргээгээд, картын өгсөн challenge-ийг PIN-ий
    /// түлхүүрээр шифрлэхэд картын cryptogram-той ЯГ таарна. Энэ нь externalAuthKey-ийн
    /// гаргалтыг (SHA1(pin) ‖ BE32(len), ДҮҮРГЭЛТГҮЙ) хатуу тогтооно.
    func testExternalAuthKeyMatchesRealTrace() throws {
        let challenge = Data([0x34, 0xA6, 0x7F, 0xF4, 0xD7, 0xF2, 0xCE, 0xE2])
        let expected  = Data([0xE6, 0x3D, 0x99, 0x05, 0x73, 0xDA, 0xF7, 0x18])
        let key = BioPassDriver.externalAuthKey(pin: "91258091")
        XCTAssertEqual(key.count, 24, "3-key 3DES түлхүүр 24 байт байх ёстой")

        let cryptogram = try SecureMessaging.des3EncryptCBCPublic(
            key: key, iv: [UInt8](repeating: 0, count: 8), data: [UInt8](challenge))
        XCTAssertEqual(Data(cryptogram.prefix(8)), expected,
                       "PIN түлхүүрийн 3DES cryptogram картынхтай таарсангүй")
    }

    /// Түлхүүрийн бүтэц: SHA1(pin) = 20 байт + урт BE32.
    func testKeyStructure() {
        let key = BioPassDriver.externalAuthKey(pin: "1234")
        XCTAssertEqual(Array(key.suffix(4)), [0x00, 0x00, 0x00, 0x04])   // len=4, big-endian
    }
}
