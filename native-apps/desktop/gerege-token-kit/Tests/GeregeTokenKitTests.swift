import XCTest
@testable import GeregeTokenKit

final class GeregeTokenKitTests: XCTestCase {

    func testHexToBytes() {
        XCTAssertEqual(hexToBytes("00A40400"), [0x00, 0xA4, 0x04, 0x00])
        XCTAssertEqual(hexToBytes("FF"), [0xFF])
        XCTAssertEqual(hexToBytes(""), [])
        XCTAssertEqual(hexToBytes("0G"), []) // invalid hex
    }

    func testHexToBytesWithColons() {
        XCTAssertEqual(hexToBytes("00:A4:04:00"), [0x00, 0xA4, 0x04, 0x00])
    }

    func testBytesToHex() {
        XCTAssertEqual(bytesToHex([0x00, 0xA4, 0x04, 0x00]), "00A40400")
        XCTAssertEqual(bytesToHex([0xFF]), "FF")
    }

    func testConstantTimeEquals() {
        XCTAssertTrue(constantTimeEquals([0x01, 0x02], [0x01, 0x02]))
        XCTAssertFalse(constantTimeEquals([0x01, 0x02], [0x01, 0x03]))
        XCTAssertFalse(constantTimeEquals([0x01], [0x01, 0x02]))
    }

    func testAPDUCommand() throws {
        let cmd = try APDUCommand(cla: 0x00, ins: 0xA4, p1: 0x04, p2: 0x00,
                                  data: BioPassDriver.appletAID)
        XCTAssertEqual(cmd.cla, 0x00)
        XCTAssertEqual(cmd.ins, 0xA4)
        XCTAssertEqual(cmd.lc, 14) // AID is 14 bytes
        let buffer = cmd.buildBuffer()
        XCTAssertEqual(buffer[0], 0x00)
        XCTAssertEqual(buffer[1], 0xA4)
        XCTAssertEqual(buffer[4], 14) // Lc
    }

    func testAPDUCommandFromHex() throws {
        let cmd = try APDUCommand(hex: "00A404000E656E746572736166652D66697073")
        XCTAssertEqual(cmd.ins, 0xA4)
        XCTAssertEqual(cmd.lc, 14)
        XCTAssertEqual(cmd.payload, BioPassDriver.appletAID)
    }

    func testAPDUResponse() {
        // Success with data
        let resp = APDUResponse(fullData: [0x01, 0x02, 0x03, 0x90, 0x00])
        XCTAssertTrue(resp.isSuccess)
        XCTAssertEqual(resp.data, [0x01, 0x02, 0x03])
        XCTAssertEqual(resp.hexStatus, "9000")

        // Error
        let errResp = APDUResponse(fullData: [0x69, 0x85])
        XCTAssertFalse(errResp.isSuccess)
        XCTAssertEqual(errResp.status, 0x6985)
        XCTAssertNil(errResp.data)
    }

    func testAPDUResponseWipe() {
        let resp = APDUResponse(fullData: [0x01, 0x02, 0x90, 0x00])
        XCTAssertNotNil(resp.data)
        resp.wipe()
        XCTAssertNil(resp.data)
    }

    func testTransportKey() {
        XCTAssertEqual(BioPassDriver.defaultTransportKey.count, 16)
        XCTAssertEqual(BioPassDriver.defaultTransportKey[0], 0x01)
        XCTAssertEqual(BioPassDriver.defaultTransportKey[15], 0x10)
    }

    func testAppletAID() {
        XCTAssertEqual(BioPassDriver.appletAID.count, 14)
        // "entersafe-fips" in ASCII
        XCTAssertEqual(String(bytes: BioPassDriver.appletAID, encoding: .ascii), "entersafe-fips")
    }

    func testTokenInfoModel() {
        let info = TokenInfo(
            atr: [0x3B, 0x9F, 0x00, 0x03],
            label: "Test",
            isFIPS: false,
            appletSelected: true
        )
        XCTAssertEqual(info.atrHex, "3B9F0003")
        XCTAssertEqual(info.label, "Test")
    }
}
