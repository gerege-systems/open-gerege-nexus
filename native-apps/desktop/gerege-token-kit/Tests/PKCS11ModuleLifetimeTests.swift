import XCTest
@testable import GeregeTokenKit

/// PKCS#11 модуль процессийн туршид ачаалагдсан хэвээр байх ёстой.
///
/// Middleware нь `C_Finalize` дотроосоо ажлаа хойшлуулдаг тул түүнийг буулгавал
/// хойшилсон блок буугаагүй хаяг уншиж `EXC_BAD_ACCESS` өгнө — Токен уншигч
/// дэлгэц нээх бүрд яг тэр болж байв (`TokenScanView.scan()` → `finalize()`).
final class PKCS11ModuleLifetimeTests: XCTestCase {

    /// `dlclose`-ийн ДАРАА модулийн тэмдгийг дуудахад амьд байх ёстой.
    ///
    /// Энэ бол унасан кодын яг тэр зам: `RTLD_NODELETE`-гүй бол энэ мөр SIGSEGV
    /// өгч, тестийн процессийг бүхэлд нь унагана — тэр нь зөв дохио.
    func testSymbolStaysCallableAfterClose() throws {
        guard let path = PKCS11Module.installedLibraryPaths.first else {
            throw XCTSkip("Энэ машин дээр PKCS#11 модуль суугаагүй")
        }
        XCTAssertNotEqual(PKCS11Module.dlopenMode & RTLD_NODELETE, 0,
                          "Модулийг RTLD_NODELETE-гүй ачаалж байна — буулгавал хойшилсон ажил унана")

        let handle = try XCTUnwrap(dlopen(path, PKCS11Module.dlopenMode))
        let symbol = try XCTUnwrap(dlsym(handle, "C_Finalize"))
        dlclose(handle)

        typealias FnFinalize = @convention(c) (UnsafeMutableRawPointer?) -> UInt
        let rv = unsafeBitCast(symbol, to: FnFinalize.self)(nil)
        // 0x190 = CKR_CRYPTOKI_NOT_INITIALIZED — амьд сангаас ирсэн ЖИНХЭНЭ хариу.
        XCTAssertEqual(rv, 0x190)
    }
}
