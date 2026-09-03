import Foundation
import Darwin
import Security
import os

/// Runtime integrity / anti-tamper checks (SEC-2 / SEC-3).
///
/// All enforcement is **Release-only** (`#if !DEBUG`) so the Xcode debug loop,
/// crash reporters and Apple's own tooling keep working in development. These
/// checks are a *deterrent* that raises the cost of reverse engineering — they
/// are not a substitute for server-side enforcement (HMAC, session/token
/// validation), which remains the source of truth.
enum SecurityGuard {

    private static let logger = Logger(subsystem: "mn.gerege.nexus.desktop", category: "SecurityGuard")

    // MARK: - Debugger detection (sysctl P_TRACED)

    /// True when this process is currently traced by a debugger.
    static func isDebuggerAttached() -> Bool {
        var info = kinfo_proc()
        var size = MemoryLayout<kinfo_proc>.stride
        var mib: [Int32] = [CTL_KERN, KERN_PROC, KERN_PROC_PID, getpid()]
        let rc = mib.withUnsafeMutableBufferPointer { buf in
            sysctl(buf.baseAddress, u_int(buf.count), &info, &size, nil, 0)
        }
        guard rc == 0 else { return false }
        return (info.kp_proc.p_flag & P_TRACED) != 0
    }

    /// Ask the kernel to refuse any future debugger attach (PT_DENY_ATTACH).
    /// Resolved via dlsym to avoid Darwin-overlay symbol-visibility issues.
    static func denyDebuggerAttach() {
        typealias PtraceFn = @convention(c) (CInt, CInt, CInt, CInt) -> CInt
        guard let sym = dlsym(UnsafeMutableRawPointer(bitPattern: -2), "ptrace") else { return }
        let ptraceFn = unsafeBitCast(sym, to: PtraceFn.self)
        _ = ptraceFn(SecurityStrings.ptDenyAttach, 0, 0, 0)
    }

    // MARK: - Code-signature self-validation

    /// Validate our own on-disk signature against a designated requirement
    /// (Apple anchor + Gerege team OU). Fails if the binary was modified or
    /// re-signed by another team.
    static func hasValidSignature() -> Bool {
        var codeRef: SecCode?
        guard SecCodeCopySelf(SecCSFlags(), &codeRef) == errSecSuccess,
              let code = codeRef else { return false }

        var staticRef: SecStaticCode?
        guard SecCodeCopyStaticCode(code, SecCSFlags(), &staticRef) == errSecSuccess,
              let staticCode = staticRef else { return false }

        var requirement: SecRequirement?
        guard SecRequirementCreateWithString(
                SecurityStrings.signingRequirement as CFString,
                SecCSFlags(), &requirement) == errSecSuccess,
              let req = requirement else { return false }

        return SecStaticCodeCheckValidity(staticCode, SecCSFlags(), req) == errSecSuccess
    }

    // MARK: - Injection detection

    /// True when a foreign library was injected (DYLD env var, or a known
    /// instrumentation framework in the loaded image list). Note: Hardened
    /// Runtime already blocks unsigned dylib injection — this is belt-and-braces.
    static func isInjected() -> Bool {
        if ProcessInfo.processInfo.environment["DYLD_INSERT_LIBRARIES"] != nil { return true }
        let count = _dyld_image_count()
        for index in 0..<count {
            guard let raw = _dyld_get_image_name(index) else { continue }
            let path = String(cString: raw).lowercased()
            for needle in SecurityStrings.suspiciousImages where path.contains(needle) {
                return true
            }
        }
        return false
    }

    // MARK: - Environment integrity (SEC-3)

    /// Running as root is never expected for a desktop GUI app.
    static func isRunningAsRoot() -> Bool { getuid() == 0 }

    /// Best-effort System Integrity Protection check. `csr_get_active_config`
    /// is an SPI resolved via dlsym; absent → returns false (treat as enabled).
    static func isSIPDisabled() -> Bool {
        typealias CsrFn = @convention(c) (UnsafeMutablePointer<UInt32>) -> CInt
        guard let sym = dlsym(UnsafeMutableRawPointer(bitPattern: -2), "csr_get_active_config") else {
            return false
        }
        let csr = unsafeBitCast(sym, to: CsrFn.self)
        var flags: UInt32 = 0
        guard csr(&flags) == 0 else { return false }
        // Any non-zero active config means at least one SIP protection is off.
        return flags != 0
    }

    // MARK: - Aggregate enforcement

    /// True when all hard integrity checks pass (always true in DEBUG).
    static func passesIntegrityChecks() -> Bool {
        #if DEBUG
        return true
        #else
        if isDebuggerAttached() { logger.error("integrity: debugger attached"); return false }
        if isInjected() { logger.error("integrity: foreign library injected"); return false }
        if isRunningAsRoot() { logger.error("integrity: running as root"); return false }
        if !hasValidSignature() { logger.error("integrity: signature invalid"); return false }
        return true
        #endif
    }

    /// Run the launch-time guard: deny attach, then verify integrity. On
    /// failure run `onFail` (e.g. wipe the session) and terminate. No-op in DEBUG.
    static func enforce(onFail: () -> Void = {}) {
        #if !DEBUG
        denyDebuggerAttach()
        if !passesIntegrityChecks() {
            onFail()
            exit(EXIT_FAILURE)
        }
        // SIP-disabled is a softer signal — log only, don't block (devs/labs).
        if isSIPDisabled() { logger.notice("integrity: SIP appears disabled") }
        #endif
    }
}

/// String constants used by `SecurityGuard`. SEC-3 obfuscates these so they
/// don't appear verbatim in `strings <binary>`.
enum SecurityStrings {
    static let ptDenyAttach: CInt = 31 // PT_DENY_ATTACH

    /// Designated requirement: signed by Apple anchor AND the Gerege team OU.
    static var signingRequirement: String {
        Obfuscated.requirement.reveal()
    }

    /// Lowercased substrings of known instrumentation libraries.
    static var suspiciousImages: [String] {
        Obfuscated.suspicious.map { $0.reveal() }
    }
}
