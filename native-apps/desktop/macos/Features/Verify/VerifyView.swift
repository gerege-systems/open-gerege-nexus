import SwiftUI
import UniformTypeIdentifiers

/// Windows `VerifyPage` port — PEM/DER X.509 цертификатыг чирж тавих, эсвэл
/// текстээр буулгах. CertificateServiceImpl-ээр parse.
struct VerifyView: View {
    @State private var pasteText: String = ""
    @State private var selectedFilePath: String = ""
    @State private var certificate: CertificateInfo?
    @State private var errorMessage: String = ""
    @State private var isDropTargeted = false
    @ObservedObject private var loc = LocalizationService.shared

    private let service = CertificateServiceImpl()

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                heroHeader
                dropZone
                pasteSection

                if !errorMessage.isEmpty {
                    InlineBanner(text: errorMessage, variant: .error)
                }

                if let cert = certificate {
                    resultCard(cert)
                }
            }
            .padding(.horizontal, Space.pageHoriz)
            .padding(.top, Space.pageTop)
            .padding(.bottom, Space.pageBottom)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .background(Color.eidSurface)
    }

    // MARK: - Hero header

    private var heroHeader: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(loc.t("Verify_Title"))
                .heroTitleStyle()
            Text(loc.t("Verify_Subtitle"))
                .subtleSubtitleStyle()
        }
    }

    // MARK: - Drop zone

    private var dropZone: some View {
        VStack(spacing: 12) {
            Image(systemName: "doc.text.magnifyingglass")
                .font(.system(size: 32))
                .foregroundStyle(Color.eidAccent)
            Text(loc.t("Verify_DropZone_Hint"))
                .font(.eidBody)
                .foregroundStyle(Color.eidMuted)
                .multilineTextAlignment(.center)
            HStack(spacing: 10) {
                Button(loc.t("Verify_PickFile")) {
                    pickFile()
                }
                .buttonStyle(.primary)

                Button(loc.t("Verify_Clear")) {
                    clearAll()
                }
                .buttonStyle(.secondary)
            }
            if !selectedFilePath.isEmpty {
                Text(selectedFilePath)
                    .font(.eidLabel)
                    .foregroundStyle(Color.eidMuted)
                    .multilineTextAlignment(.center)
                    .lineLimit(2)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(28)
        .background(
            Color.eidAccentSubtle.opacity(isDropTargeted ? 0.65 : 0.35)
        )
        .overlay(
            RoundedRectangle(cornerRadius: Radius.lg, style: .continuous)
                .strokeBorder(
                    isDropTargeted ? Color.eidAccent : Color.eidCardStroke,
                    style: StrokeStyle(lineWidth: 2, dash: [6, 4])
                )
        )
        .clipShape(RoundedRectangle(cornerRadius: Radius.lg, style: .continuous))
        .onDrop(of: [.fileURL], isTargeted: $isDropTargeted) { providers in
            handleDrop(providers: providers)
        }
    }

    // MARK: - Paste section

    private var pasteSection: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 10) {
                Text(loc.t("Verify_Paste_Hint"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                TextEditor(text: $pasteText)
                    .font(.eidMonoSmall)
                    .frame(minHeight: 160, maxHeight: 220)
                    .padding(8)
                    .background(Color.backgroundSecondary)
                    .clipShape(RoundedRectangle(cornerRadius: Radius.md, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: Radius.md, style: .continuous)
                            .strokeBorder(Color.eidCardStroke, lineWidth: 1)
                    )
                HStack {
                    Spacer()
                    Button(loc.t("Verify_Parse")) {
                        parsePaste()
                    }
                    .buttonStyle(.primary)
                    .disabled(pasteText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
        }
    }

    // MARK: - Result card

    private func resultCard(_ cert: CertificateInfo) -> some View {
        let now = Date()
        let variant: StatusPill.Variant = {
            if cert.isExpired(at: now)    { return .bad }
            if cert.isNotYetValid(at: now) { return .warn }
            return .ok
        }()
        let label: String = {
            if cert.isExpired(at: now)    { return loc.t("Verify_Validity_Expired") }
            if cert.isNotYetValid(at: now) { return loc.t("Verify_Validity_NotYetValid") }
            return loc.t("Verify_Validity_Valid")
        }()

        return AppCard {
            VStack(alignment: .leading, spacing: 20) {
                HStack(spacing: 10) {
                    StatusDot(color: variantColor(variant), size: 12)
                    Text(label)
                        .font(.eidPageTitle)
                        .foregroundStyle(Color.textPrimary)
                    Spacer()
                    StatusPill(label, variant: variant)
                }

                verifySection(loc.t("Verify_Section_Identity")) {
                    verifyRow(loc.t("Verify_Field_Subject"), cert.subject)
                    verifyRow(loc.t("Verify_Field_Issuer"), cert.issuer)
                    verifyRow(loc.t("Verify_Field_Serial"), cert.serialNumberHex)
                }

                verifySection(loc.t("Verify_Section_Validity")) {
                    verifyRow(loc.t("Verify_Field_NotBefore"), formatDate(cert.notBefore))
                    verifyRow(loc.t("Verify_Field_NotAfter"), formatDate(cert.notAfter))
                }

                verifySection(loc.t("Verify_Section_Crypto")) {
                    verifyRow(loc.t("Verify_Field_SignatureAlgorithm"), cert.signatureAlgorithm)
                    verifyRow(loc.t("Verify_Field_PublicKey"), "\(cert.publicKeyAlgorithm) / \(cert.publicKeyBits) bit")
                }

                verifySection(loc.t("Verify_Section_Fingerprints")) {
                    verifyRow(loc.t("Verify_Field_Sha256"), cert.thumbprintSha256Hex)
                    verifyRow(loc.t("Verify_Field_Sha1"),   cert.thumbprintSha1Hex)
                    verifyRow(loc.t("Verify_Field_SpkiSha256"), cert.spkiSha256Base64)
                }

                if !cert.keyUsages.isEmpty {
                    verifySection(loc.t("Verify_Section_KeyUsage")) {
                        Text(cert.keyUsages.joined(separator: ", "))
                            .font(.eidBody)
                            .foregroundStyle(Color.textPrimary)
                    }
                }

                if !cert.subjectAlternativeNames.isEmpty {
                    verifySection(loc.t("Verify_Section_San")) {
                        ForEach(cert.subjectAlternativeNames, id: \.self) { name in
                            Text(name)
                                .font(.eidMonoSmall)
                                .foregroundStyle(Color.textPrimary)
                                .textSelection(.enabled)
                        }
                    }
                }
            }
        }
    }

    @ViewBuilder
    private func verifySection<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title)
                .font(.eidSectionTitle)
                .foregroundStyle(Color.textPrimary)
            content()
        }
    }

    private func verifyRow(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Text("\(label):")
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
                .frame(width: 150, alignment: .leading)
            Text(value)
                .font(.eidMonoSmall)
                .foregroundStyle(Color.textPrimary)
                .textSelection(.enabled)
                .lineLimit(nil)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
    }

    private func variantColor(_ variant: StatusPill.Variant) -> Color {
        switch variant {
        case .ok: return Color.eidSuccess
        case .warn: return Color.eidWarning
        case .bad: return Color.eidDanger
        case .accent: return Color.eidAccent
        case .neutral: return Color.eidMuted
        }
    }

    // MARK: - Actions

    private func clearAll() {
        certificate = nil
        pasteText = ""
        selectedFilePath = ""
        errorMessage = ""
    }

    private func pickFile() {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.message = "X.509 гэрчилгээ сонгоно уу"
        if let pem = UTType("public.pem"), pem.identifier != "" {
            panel.allowedContentTypes = [pem, .x509Certificate, .data]
        } else {
            panel.allowedContentTypes = [.x509Certificate, .data]
        }
        guard panel.runModal() == .OK, let url = panel.url else { return }
        loadFile(at: url)
    }

    private func handleDrop(providers: [NSItemProvider]) -> Bool {
        guard let provider = providers.first else { return false }
        provider.loadItem(forTypeIdentifier: UTType.fileURL.identifier, options: nil) { item, _ in
            var url: URL?
            if let urlValue = item as? URL {
                url = urlValue
            } else if let data = item as? Data {
                url = URL(dataRepresentation: data, relativeTo: nil)
            }
            if let url {
                DispatchQueue.main.async { loadFile(at: url) }
            }
        }
        return true
    }

    private func loadFile(at url: URL) {
        selectedFilePath = url.path
        errorMessage = ""
        switch service.parseFile(at: url) {
        case .success(let cert):
            certificate = cert
            pasteText = cert.pemSource
        case .failure(let err):
            certificate = nil
            errorMessage = err.message
        }
    }

    private func parsePaste() {
        errorMessage = ""
        switch service.parsePem(pasteText) {
        case .success(let cert):
            certificate = cert
        case .failure(let err):
            certificate = nil
            errorMessage = err.message
        }
    }

    private func formatDate(_ d: Date) -> String {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd HH:mm:ss z"
        f.timeZone = TimeZone(identifier: "Asia/Ulaanbaatar")
        return f.string(from: d)
    }
}
