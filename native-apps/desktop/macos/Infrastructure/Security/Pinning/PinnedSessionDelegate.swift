import Foundation
import Security
import os

/// SPKI-SHA256 certificate pinning for the desktop backend sessions.
///
/// Ported from the iOS client (`PinnedSessionDelegate`). Pins the **Let's
/// Encrypt E7 + E8 intermediates** rather than the leaf, so a ~90-day leaf
/// renewal does not lock the client out. Because the intermediate is the pin
/// target, the same set protects every Let's Encrypt-served host the app
/// talks to (`api.eidmongol.mn`, `ca.gerege.mn`).
///
/// DEBUG builds bypass pin verification (still requires a valid system trust)
/// so the dev loop works against staging / a local server.
final class PinnedSessionDelegate: NSObject, URLSessionDelegate {

    private let pinnedHashes: Set<String>
    private let logger = Logger(subsystem: "mn.gerege.nexus.desktop", category: "Pinning")

    /// - Parameter extraLeafPins: optional additional leaf SPKI-SHA256 (base64)
    ///   pins. The LE E7/E8 intermediate pins are always included.
    init(extraLeafPins: [String] = []) {
        var pins = Set(extraLeafPins.filter { !$0.isEmpty })
        for pin in SecurityPins.intermediatePins {
            pins.insert(pin)
        }
        self.pinnedHashes = pins
    }

    func urlSession(
        _ session: URLSession,
        didReceive challenge: URLAuthenticationChallenge,
        completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void
    ) {
        guard challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodServerTrust,
              let trust = challenge.protectionSpace.serverTrust else {
            completionHandler(.performDefaultHandling, nil)
            return
        }

        #if DEBUG
        // Dev loop: accept system trust, skip pin verification.
        completionHandler(.useCredential, URLCredential(trust: trust))
        #else
        // v3: pinning is opt-in (`security.tlsPinning`) — the v3 deployment's
        // CA chain may not match the baked-in LE pins; system trust still applies.
        guard UserDefaults.standard.bool(forKey: "security.tlsPinning") else {
            if SecTrustEvaluateWithError(trust, nil) {
                completionHandler(.useCredential, URLCredential(trust: trust))
            } else {
                completionHandler(.cancelAuthenticationChallenge, nil)
            }
            return
        }
        guard SecTrustEvaluateWithError(trust, nil),
              let chain = SecTrustCopyCertificateChain(trust) as? [SecCertificate] else {
            logger.error("Pinning: system trust evaluation failed")
            completionHandler(.cancelAuthenticationChallenge, nil)
            return
        }
        // Walk the full chain — match the leaf or any issuer pin.
        for cert in chain {
            if let pin = SPKIHash.sha256Base64(of: cert), pinnedHashes.contains(pin) {
                completionHandler(.useCredential, URLCredential(trust: trust))
                return
            }
        }
        logger.error("Pinning: no pinned key in chain — refusing connection (possible MITM)")
        completionHandler(.cancelAuthenticationChallenge, nil)
        #endif
    }
}

/// Pinned SPKI-SHA256 values. Kept in sync with the iOS client's pin set.
/// XOR-obfuscated (SEC-3) so they don't appear verbatim in `strings <binary>`;
/// assembled at runtime.
enum SecurityPins {
    static var intermediatePins: [String] {
        Obfuscated.pins.map { $0.reveal() }
    }
}
