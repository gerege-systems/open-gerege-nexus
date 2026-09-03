import SwiftUI
import CryptoKit
import GeregeTokenKit

/// USB токены гарын үсгийн тест — `TokensView` (токен detail) дотор AppCard-д
/// шигдсэн. Файл сонгох → SHA-256 → GeregeTokenKit SM (APDU) эсвэл libcastle
/// PKCS#11-ээр ECDSA. Энэ нь хатуу токены хөгжүүлэлт/тест зориулалттай —
/// иргэний "өөрийн нэрийн өмнөөс PDF гарын үсэг" нь `SignView` (web /sign parity).
struct TokenSignTestView: View {
    @EnvironmentObject private var tokenManager: TokenManager
    @ObservedObject private var loc = LocalizationService.shared

    @State private var documentHash = ""
    @State private var documentName = ""
    @State private var signatureResult: String?
    @State private var isLoading = false
    @State private var errorMessage: String?

    @State private var signMode: SignMode = .apdu
    @State private var pin = ""
    @State private var keyLabel = ""

    enum SignMode: String, CaseIterable, Identifiable {
        case apdu = "APDU (SM)"
        case pkcs11 = "PKCS#11 (FEITIAN)"
        var id: String { rawValue }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            modePicker
            documentSection
            actionRow

            if let error = errorMessage {
                InlineBanner(text: error, variant: .error)
            }
            if let result = signatureResult {
                resultBlock(result)
            }
        }
    }

    // MARK: - Mode

    private var modePicker: some View {
        VStack(alignment: .leading, spacing: 10) {
            Picker("", selection: $signMode) {
                ForEach(SignMode.allCases) { mode in
                    Text(mode.rawValue).tag(mode)
                }
            }
            .pickerStyle(.segmented)
            .labelsHidden()

            if signMode == .pkcs11 {
                VStack(alignment: .leading, spacing: 8) {
                    field("PIN", binding: $pin, isSecure: true, placeholder: "User PIN")
                    field(loc.t("Sign_KeyLabel"), binding: $keyLabel,
                          placeholder: "(хоосон бол эхний түлхүүр)")
                }
                Text("FEITIAN Castle драйвер (libcastle.1.0.0.dylib)-аар дамжина.")
                    .labelStyle()
            } else {
                Text("GeregeTokenKit SM mutual auth + APDU sign (PIN токены өөрийнх).")
                    .labelStyle()
            }
        }
    }

    private func field(_ label: String, binding: Binding<String>, isSecure: Bool = false, placeholder: String) -> some View {
        HStack(alignment: .center, spacing: 10) {
            Text("\(label):")
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
                .frame(width: 80, alignment: .leading)
            Group {
                if isSecure {
                    SecureField(placeholder, text: binding)
                } else {
                    TextField(placeholder, text: binding)
                }
            }
            .textFieldStyle(.roundedBorder)
            .frame(maxWidth: 280)
        }
    }

    // MARK: - Document

    private var documentSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            field(loc.t("Sign_Field_DocName"), binding: $documentName, placeholder: "Гэрээ.pdf")
            HStack(alignment: .center, spacing: 10) {
                Text("SHA-256:")
                    .font(.eidLabel)
                    .foregroundStyle(Color.eidMuted)
                    .frame(width: 80, alignment: .leading)
                TextField("Файлын SHA-256 hash", text: $documentHash)
                    .textFieldStyle(.roundedBorder)
                    .font(.eidMonoSmall)
            }

            HStack(spacing: 10) {
                Button(loc.t("Sign_PickFile")) { selectFile() }
                    .buttonStyle(.secondary)

                if !documentHash.isEmpty {
                    HStack(spacing: 4) {
                        Image(systemName: "checkmark.circle.fill")
                            .foregroundStyle(Color.eidSuccess)
                        Text(loc.t("Sign_HashReady"))
                            .font(.eidLabel)
                            .foregroundStyle(Color.eidSuccess)
                    }
                }
            }
        }
    }

    // MARK: - Action

    private var actionRow: some View {
        HStack {
            Button {
                Task { await performSign() }
            } label: {
                HStack(spacing: 6) {
                    if isLoading { ProgressView().controlSize(.small) }
                    Image(systemName: "signature")
                    Text(loc.t("Sign_Action"))
                }
            }
            .buttonStyle(.primary)
            .disabled(!canSign)
            .keyboardShortcut(.return, modifiers: .command)

            Spacer()
        }
    }

    // MARK: - Result

    private func resultBlock(_ result: String) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 6) {
                Image(systemName: "checkmark.seal.fill")
                    .foregroundStyle(Color.eidSuccess)
                Text(loc.t("Sign_Success"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.eidSuccess)
            }
            Text(result)
                .font(.eidMonoSmall)
                .textSelection(.enabled)
                .lineLimit(8)
                .padding(10)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Color.backgroundSecondary)
                .clipShape(RoundedRectangle(cornerRadius: Radius.md, style: .continuous))
            Button(loc.t("Sign_Copy")) {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(result, forType: .string)
            }
            .buttonStyle(.secondary)
        }
    }

    // MARK: - Logic

    private var canSign: Bool {
        guard !isLoading, !documentHash.isEmpty, tokenManager.isTokenPresent
        else { return false }
        if signMode == .pkcs11, pin.isEmpty { return false }
        return true
    }

    private func selectFile() {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.message = "Гарын үсэг зурах файл сонгоно уу"

        guard panel.runModal() == .OK, let url = panel.url else { return }

        do {
            let data = try Data(contentsOf: url)
            let hash = SHA256.hash(data: data)
            documentHash = hash.compactMap { String(format: "%02x", $0) }.joined()
            if documentName.isEmpty {
                documentName = url.lastPathComponent
            }
        } catch {
            errorMessage = "Файл уншиx алдаа: \(error.localizedDescription)"
        }
    }

    private func performSign() async {
        // SEC-3: guard the crypto path — refuse to sign on a tampered runtime.
        SecurityGuard.enforce()
        isLoading = true
        errorMessage = nil
        signatureResult = nil
        defer { isLoading = false }

        do {
            let hashBytes = hexToBytes(documentHash)
            guard hashBytes.count == 32 else {
                errorMessage = "SHA-256 hash 32 байт (64 hex тэмдэгт) байх ёстой."
                return
            }

            let signature: [UInt8]
            switch signMode {
            case .apdu:
                signature = try await tokenManager.signECDSA(hash: hashBytes)
            case .pkcs11:
                let label = keyLabel.isEmpty ? nil : keyLabel
                let pinCopy = pin
                signature = try await Task.detached(priority: .userInitiated) {
                    let module = try PKCS11Module.open()
                    return try module.signECDSA(pin: pinCopy, keyLabel: label, hash: hashBytes)
                }.value
            }
            signatureResult = bytesToHex(signature)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
