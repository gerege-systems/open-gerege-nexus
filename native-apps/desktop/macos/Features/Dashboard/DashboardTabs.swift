import SwiftUI
import GeregeTokenKit

// MARK: - USB Token Tab (Windows TokensPage + TokenDetailPage port)
//
// `TokensView` mirrors the Windows two-page structure:
//   • TokensListView  — Windows `TokensPage`: detected-token list + scan.
//   • TokenDetailView — Windows `TokenDetailPage`: per-token management
//     (info, provision wizard, CSR flow, sign test).
//
// macOS detects a single FEITIAN token via `TokenManager` (TKTokenWatcher),
// so the list shows that one token (or an empty state) and "View" opens the
// detail page — the same list→detail flow `OrgView` uses internally.

/// `TokensView` — entry from sidebar `tokens` route. Routes between the
/// token list and the per-token detail page.
struct TokensView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var tokenManager: TokenManager

    @State private var mode: Mode = .list

    enum Mode { case list, detail }

    var body: some View {
        switch mode {
        case .list:
            TokensListView(onOpenDetail: { mode = .detail })
        case .detail:
            TokenDetailView(onBack: { mode = .list })
        }
    }
}

// MARK: - Tokens list (Windows TokensPage)

/// Detected-token list with a scan/refresh action. Selecting a token opens
/// `TokenDetailView`.
struct TokensListView: View {
    @EnvironmentObject private var tokenManager: TokenManager
    @ObservedObject private var loc = LocalizationService.shared
    let onOpenDetail: () -> Void

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                heroHeader
                tokensCard
            }
            .padding(.horizontal, Space.pageHoriz)
            .padding(.top, Space.pageTop)
            .padding(.bottom, Space.pageBottom)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .background(Color.eidSurface)
    }

    // MARK: Hero header

    private var heroHeader: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(loc.t("Tokens_Title"))
                .heroTitleStyle()
            Text(loc.t("Tokens_Subtitle"))
                .subtleSubtitleStyle()
        }
    }

    // MARK: Tokens card

    private var tokensCard: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                Text(loc.t("Tokens_Connected_Header"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Spacer()
                Button {
                    tokenManager.refreshTokenList()
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "arrow.clockwise")
                        Text(loc.t("Common_Refresh"))
                    }
                }
                .buttonStyle(.secondary)
            }

            AppCard {
                if tokenManager.isTokenPresent {
                    tokenRow
                } else {
                    emptyState
                }
            }
        }
    }

    private var emptyState: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(loc.t("Tokens_Empty_Title"))
                .font(.eidBody)
                .foregroundStyle(Color.textPrimary)
            Text(loc.t("Tokens_Empty_Hint"))
                .labelStyle()
        }
    }

    private var tokenRow: some View {
        HStack(spacing: 14) {
            ZStack {
                RoundedRectangle(cornerRadius: 10)
                    .fill(Color.eidAccentSubtle)
                    .frame(width: 44, height: 44)
                Image(systemName: "key.horizontal.fill")
                    .font(.system(size: 18))
                    .foregroundStyle(Color.eidAccent)
            }
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 8) {
                    Text(tokenManager.tokenInfo?.label ?? loc.t("Tokens_Default_Name"))
                        .font(.eidSectionTitle)
                        .foregroundStyle(Color.textPrimary)
                    AccentBadge(text: tokenManager.tokenInfo?.isFIPS == true ? "FIPS" : "APDU")
                }
                Text(tokenManager.tokenInfo?.atrHex ?? "")
                    .font(.eidMonoSmall)
                    .foregroundStyle(Color.eidMuted)
                    .textSelection(.enabled)
            }
            Spacer()
            Button {
                onOpenDetail()
            } label: {
                HStack(spacing: 6) {
                    Text(loc.t("Common_Details"))
                    Image(systemName: "chevron.right")
                }
            }
            .buttonStyle(.primary)
        }
    }
}

// MARK: - Token detail (Windows TokenDetailPage)

/// Per-token management page: info, provision wizard, CSR flow, sign test.
struct TokenDetailView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var tokenManager: TokenManager
    @ObservedObject private var loc = LocalizationService.shared

    let onBack: () -> Void

    @State private var provisionStep: ProvisionStep = .idle
    @State private var pin = ""
    @State private var pinConfirm = ""
    @State private var puk = ""
    @State private var isLoading = false
    @State private var statusMessage = ""
    @State private var statusIsError = false
    @State private var progressStage = ""

    // CSR / provision flow state
    @State private var csrPIN = ""
    @State private var csrNationalID = ""
    @State private var csrLabel = ""
    @State private var csrResultPEM: String?

    // Change user PIN
    @State private var curUserPIN = ""
    @State private var newUserPIN = ""
    @State private var newUserPINConfirm = ""
    @State private var changingUserPIN = false
    @State private var userPINStatus = ""
    @State private var userPINIsError = false

    // Unlock user PIN (SO required)
    @State private var unlockSOPIN = ""
    @State private var unlockNewPIN = ""
    @State private var unlockNewPINConfirm = ""
    @State private var unlockingPIN = false
    @State private var unlockStatus = ""
    @State private var unlockIsError = false

    // Change SO (admin) PIN
    @State private var curSOPIN = ""
    @State private var newSOPIN = ""
    @State private var newSOPINConfirm = ""
    @State private var changingSOPIN = false
    @State private var soPINStatus = ""
    @State private var soPINIsError = false

    // Objects + key generation (PKCS#11)
    @State private var objectsPIN = ""
    @State private var objects: [GeregeTokenKit.PKCS11Module.TokenObjectInfo] = []
    @State private var objectsLoading = false
    @State private var objectsStatus = ""
    @State private var objectsIsError = false
    @State private var genKeyLabel = "gerege-sign"
    @State private var genKeyLoading = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                backRow

                detailHero

                if let info = tokenManager.tokenInfo {
                    tokenInfoCard(info)
                }

                if !statusMessage.isEmpty {
                    InlineBanner(
                        text: statusMessage,
                        variant: statusIsError ? .error : .success
                    )
                }

                if tokenManager.isTokenPresent {
                    provisionCard
                    objectsCard
                    generateKeyCard
                    csrCard
                    signCard
                    changeUserPINCard
                    unlockPINCard
                    changeSOPINCard
                } else {
                    InlineBanner(
                        text: loc.t("Tokens_Disconnected_Notice"),
                        variant: .info
                    )
                }
            }
            .padding(.horizontal, Space.pageHoriz)
            .padding(.top, Space.pageTop)
            .padding(.bottom, Space.pageBottom)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .background(Color.eidSurface)
    }

    // MARK: - Back row

    private var backRow: some View {
        Button {
            onBack()
        } label: {
            HStack(spacing: 6) {
                Image(systemName: "chevron.left")
                Text(loc.t("Nav_Tokens"))
            }
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(Color.eidAccent)
        }
        .buttonStyle(.plain)
    }

    // MARK: - Detail hero (status + label)

    private var detailHero: some View {
        AppCard {
            HStack(spacing: 14) {
                ZStack {
                    RoundedRectangle(cornerRadius: 10)
                        .fill((tokenManager.isTokenPresent ? Color.eidSuccess : Color.eidMuted).opacity(0.12))
                        .frame(width: 48, height: 48)
                    Image(systemName: tokenManager.isTokenPresent
                          ? "checkmark.shield.fill"
                          : "xmark.shield")
                        .font(.system(size: 20))
                        .foregroundStyle(tokenManager.isTokenPresent ? Color.eidSuccess : Color.eidMuted)
                }
                VStack(alignment: .leading, spacing: 4) {
                    HStack(spacing: 8) {
                        Text(tokenManager.isTokenPresent
                             ? (tokenManager.tokenInfo?.label ?? loc.t("Tokens_Connected"))
                             : loc.t("Tokens_Disconnected"))
                            .font(.eidPageTitle)
                            .foregroundStyle(Color.textPrimary)
                        if tokenManager.isTokenPresent {
                            AccentBadge(text: tokenManager.tokenInfo?.isFIPS == true ? "FIPS" : "APDU")
                        }
                    }
                    Text(tokenManager.isTokenPresent
                         ? loc.t("Tokens_Ready")
                         : loc.t("Tokens_PlugIn"))
                        .labelStyle()
                }
                Spacer()
                Button(loc.t("Common_Refresh")) {
                    tokenManager.refreshTokenList()
                }
                .buttonStyle(.secondary)
            }
        }
    }

    // MARK: - Token info card

    private func tokenInfoCard(_ info: GeregeTokenKit.TokenInfo) -> some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Tokens_Info"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                infoRow(loc.t("Tokens_Info_Name"), info.label)
                infoRow(loc.t("Tokens_Info_Fips"), info.isFIPS ? loc.t("Tokens_Yes") : loc.t("Tokens_No"))
                infoRow(loc.t("Tokens_Info_Atr"), info.atrHex)
            }
        }
    }

    private func infoRow(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Text(label)
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
                .frame(width: 96, alignment: .leading)
            Text(value)
                .font(.eidMonoSmall)
                .foregroundStyle(Color.textPrimary)
                .textSelection(.enabled)
            Spacer(minLength: 0)
        }
    }

    // MARK: - CSR card

    private var csrCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 14) {
                Text(loc.t("Tokens_Section_Csr"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text("Токен дээрх EC P-256 pubkey-ээр PKCS#10 CSR үүсгэж CA руу илгээнэ.")
                    .labelStyle()

                VStack(alignment: .leading, spacing: 10) {
                    csrField("Регистрийн дугаар (CN)", binding: $csrNationalID, placeholder: "МА74101813")
                    csrSecureField("Token PIN", binding: $csrPIN)
                    csrField("Key label", binding: $csrLabel, placeholder: "(хоосон бол эхний түлхүүр)")
                }

                HStack(spacing: 12) {
                    Button {
                        Task { await runCSRFlow() }
                    } label: {
                        HStack(spacing: 6) {
                            if isLoading { ProgressView().controlSize(.small) }
                            Image(systemName: "arrow.up.doc")
                            Text("CSR үүсгэж илгээх")
                        }
                    }
                    .buttonStyle(.primary)
                    .disabled(isLoading || csrPIN.isEmpty || csrNationalID.isEmpty)

                    if let pem = csrResultPEM {
                        Button("Хуулах CSR") {
                            NSPasteboard.general.clearContents()
                            NSPasteboard.general.setString(pem, forType: .string)
                        }
                        .buttonStyle(.secondary)
                    }
                }

                if let pem = csrResultPEM {
                    Text(pem)
                        .font(.eidMonoSmall)
                        .textSelection(.enabled)
                        .lineLimit(10)
                        .padding(10)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(Color.backgroundSecondary)
                        .clipShape(RoundedRectangle(cornerRadius: Radius.md, style: .continuous))
                }
            }
        }
    }

    private func csrField(_ label: String, binding: Binding<String>, placeholder: String) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label)
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
            TextField(placeholder, text: binding)
                .textFieldStyle(.roundedBorder)
                .frame(maxWidth: 280)
        }
    }

    private func csrSecureField(_ label: String, binding: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label)
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
            SecureField("PIN", text: binding)
                .textFieldStyle(.roundedBorder)
                .frame(maxWidth: 280)
        }
    }

    // MARK: - Provision card

    private var provisionCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 14) {
                Text(loc.t("Tokens_Section_Provision"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                switch provisionStep {
                case .idle:           provisionIdleView
                case .setPIN:         provisionPINView
                case .setPINPKCS11:   provisionPINPKCS11View
                case .generating:     provisionGeneratingView
                case .done:           provisionDoneView
                }
            }
        }
    }

    private var provisionIdleView: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Шинэ токен тохируулах — 2 зам: APDU (SM) эсвэл PKCS#11 (FEITIAN).")
                .labelStyle()

            HStack(spacing: 10) {
                Button {
                    provisionStep = .setPIN
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "lock.shield")
                        Text("APDU тохируулах")
                    }
                }
                .buttonStyle(.primary)

                Button {
                    provisionStep = .setPINPKCS11
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "cpu")
                        Text("PKCS#11 тохируулах")
                    }
                }
                .buttonStyle(.secondary)
            }

            HStack(spacing: 10) {
                Button {
                    Task { await testConnection() }
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "antenna.radiowaves.left.and.right")
                        Text("Холболт тест")
                    }
                }
                .buttonStyle(.secondary)

                Button {
                    Task { await testPKCS11() }
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "cpu")
                        Text("PKCS#11 тест")
                    }
                }
                .buttonStyle(.secondary)
                .disabled(isLoading)

                Button {
                    Task { await eraseOnly() }
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "trash")
                        Text("Factory erase")
                    }
                }
                .buttonStyle(.secondary)
                .disabled(isLoading)
            }
        }
    }

    private var provisionPINView: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("PIN болон PUK код тогтооно уу")
                .font(.eidSectionTitle)
                .foregroundStyle(Color.textPrimary)

            pinField("User PIN (6-16 тэмдэгт)", binding: $pin)
            pinField("PIN давтах", binding: $pinConfirm)
            pinField("SO PIN / PUK (8-16 тэмдэгт)", binding: $puk)

            HStack(spacing: 12) {
                Button {
                    Task { await startProvision() }
                } label: {
                    HStack(spacing: 6) {
                        if isLoading { ProgressView().controlSize(.small) }
                        Text("Тохируулах")
                    }
                }
                .buttonStyle(.primary)
                .disabled(!canProvision || isLoading)

                Button("Цуцлах") {
                    provisionStep = .idle
                    clearFields()
                }
                .buttonStyle(.secondary)
                .disabled(isLoading)
            }
        }
    }

    private var provisionPINPKCS11View: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "exclamationmark.triangle.fill")
                    .foregroundStyle(Color.eidDanger)
                Text("PKCS#11 provision — factory reset")
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.eidDanger)
            }
            Text("Токен бүх түлхүүр, сертификат устана. libcastle.dylib-ээр C_InitToken → C_InitPIN → C_GenerateKeyPair гүйцэтгэнэ.")
                .labelStyle()

            pinField("SO PIN (8-16 тэмдэгт)", binding: $puk)
            pinField("User PIN (6-16 тэмдэгт)", binding: $pin)
            pinField("User PIN давтах", binding: $pinConfirm)

            HStack(spacing: 12) {
                Button {
                    Task { await startProvisionPKCS11() }
                } label: {
                    HStack(spacing: 6) {
                        if isLoading { ProgressView().controlSize(.small) }
                        Text("Factory reset + EC keypair")
                    }
                }
                .buttonStyle(.primary)
                .disabled(!canProvision || isLoading)

                Button("Цуцлах") {
                    provisionStep = .idle
                    clearFields()
                }
                .buttonStyle(.secondary)
                .disabled(isLoading)
            }
        }
    }

    private var provisionGeneratingView: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                ProgressView().controlSize(.small)
                Text(progressStage.isEmpty ? "Боловсруулж байна..." : progressStage)
                    .font(.eidBody)
                    .foregroundStyle(Color.textPrimary)
            }
            Text("Токен дээр шууд гүйцэтгэнэ — хэсэг хугацаа шаардана.")
                .labelStyle()
        }
    }

    private var provisionDoneView: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 8) {
                Image(systemName: "checkmark.seal.fill")
                    .foregroundStyle(Color.eidSuccess)
                Text("Токен амжилттай тохируулагдлаа!")
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.eidSuccess)
            }
            Text("PIN тогтоогдсон. Түлхүүр хос үүсгэгдсэн. Одоо гарын үсэг зурахад бэлэн.")
                .labelStyle()
            Button("Шинэ тохиргоо") {
                provisionStep = .idle
                clearFields()
            }
            .buttonStyle(.secondary)
        }
    }

    private func pinField(_ label: String, binding: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label)
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
            SecureField("", text: binding)
                .textFieldStyle(.roundedBorder)
                .frame(maxWidth: 280)
        }
    }

    // MARK: - Sign card (inline sign test)

    private var signCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Tokens_Section_SignTest"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text("Файл сонгож SHA-256 hash тооцоод токеноор ECDSA гарын үсэг зурна.")
                    .labelStyle()
                TokenSignTestView()
                    .padding(.top, 4)
            }
        }
    }

    // MARK: - Change user PIN card (Windows TokenDetailPage "Change PIN")

    private var changeUserPINCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Tokens_Section_ChangePin"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text(loc.t("Tokens_ChangePin_Hint"))
                    .labelStyle()

                pinField(loc.t("Tokens_ChangePin_Current"), binding: $curUserPIN)
                pinField(loc.t("Tokens_ChangePin_New"), binding: $newUserPIN)
                pinField(loc.t("Tokens_ChangePin_Confirm"), binding: $newUserPINConfirm)

                Button {
                    Task { await changeUserPIN() }
                } label: {
                    HStack(spacing: 6) {
                        if changingUserPIN { ProgressView().controlSize(.small) }
                        Image(systemName: "lock.rotation")
                        Text(loc.t("Tokens_ChangePin_Action"))
                    }
                }
                .buttonStyle(.primary)
                .disabled(changingUserPIN || !canChangeUserPIN)

                if !userPINStatus.isEmpty {
                    InlineBanner(text: userPINStatus, variant: userPINIsError ? .error : .success)
                }
            }
        }
    }

    // MARK: - Unlock user PIN card (Windows TokenDetailPage "Unlock User PIN")

    private var unlockPINCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Tokens_Section_UnlockPin"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text(loc.t("Tokens_UnlockPin_Hint"))
                    .labelStyle()

                pinField(loc.t("Tokens_Unlock_SoPin"), binding: $unlockSOPIN)
                pinField(loc.t("Tokens_Unlock_New"), binding: $unlockNewPIN)
                pinField(loc.t("Tokens_Unlock_Confirm"), binding: $unlockNewPINConfirm)

                Button {
                    Task { await unlockPIN() }
                } label: {
                    HStack(spacing: 6) {
                        if unlockingPIN { ProgressView().controlSize(.small) }
                        Image(systemName: "lock.open")
                        Text(loc.t("Tokens_UnlockPin_Action"))
                    }
                }
                .buttonStyle(.primary)
                .disabled(unlockingPIN || !canUnlockPIN)

                if !unlockStatus.isEmpty {
                    InlineBanner(text: unlockStatus, variant: unlockIsError ? .error : .success)
                }
            }
        }
    }

    // MARK: - Change SO PIN card (Windows TokenDetailPage "Change SO PIN")

    private var changeSOPINCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Tokens_Section_ChangeSoPin"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text(loc.t("Tokens_ChangeSoPin_Hint"))
                    .labelStyle()

                pinField(loc.t("Tokens_SoPin_Current"), binding: $curSOPIN)
                pinField(loc.t("Tokens_SoPin_New"), binding: $newSOPIN)
                pinField(loc.t("Tokens_SoPin_Confirm"), binding: $newSOPINConfirm)

                Button {
                    Task { await changeSOPIN() }
                } label: {
                    HStack(spacing: 6) {
                        if changingSOPIN { ProgressView().controlSize(.small) }
                        Image(systemName: "key.radiowaves.forward")
                        Text(loc.t("Tokens_ChangeSoPin_Action"))
                    }
                }
                .buttonStyle(.primary)
                .disabled(changingSOPIN || !canChangeSOPIN)

                if !soPINStatus.isEmpty {
                    InlineBanner(text: soPINStatus, variant: soPINIsError ? .error : .success)
                }
            }
        }
    }

    // MARK: - Objects card (Windows TokenDetailPage "Objects")

    private var objectsCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Tokens_Objects_Section"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)

                pinField(loc.t("Tokens_Objects_Pin"), binding: $objectsPIN)

                HStack(spacing: 12) {
                    Button {
                        Task { await loadObjects() }
                    } label: {
                        HStack(spacing: 6) {
                            if objectsLoading { ProgressView().controlSize(.small) }
                            Image(systemName: "list.bullet.rectangle")
                            Text(loc.t("Tokens_Objects_Load"))
                        }
                    }
                    .buttonStyle(.primary)
                    .disabled(objectsLoading || objectsPIN.isEmpty)

                    Button {
                        Task { await importCertificate() }
                    } label: {
                        HStack(spacing: 6) {
                            Image(systemName: "square.and.arrow.down")
                            Text(loc.t("Tokens_Objects_Import"))
                        }
                    }
                    .buttonStyle(.secondary)
                    .disabled(objectsLoading || objectsPIN.isEmpty)
                }

                if objects.isEmpty {
                    Text(loc.t("Tokens_Objects_Empty")).labelStyle()
                } else {
                    VStack(spacing: 0) {
                        ForEach(Array(objects.enumerated()), id: \.offset) { idx, obj in
                            objectRow(obj)
                            if idx < objects.count - 1 {
                                Divider().background(Color.eidCardStroke)
                            }
                        }
                    }
                }

                if !objectsStatus.isEmpty {
                    InlineBanner(text: objectsStatus, variant: objectsIsError ? .error : .success)
                }
            }
        }
    }

    private func objectRow(_ obj: GeregeTokenKit.PKCS11Module.TokenObjectInfo) -> some View {
        HStack(spacing: 12) {
            Image(systemName: objectIcon(obj.kind))
                .font(.system(size: 15))
                .foregroundStyle(Color.eidAccent)
                .frame(width: 22)
            VStack(alignment: .leading, spacing: 2) {
                Text(obj.label.isEmpty ? obj.kind : obj.label)
                    .font(.eidBody)
                    .foregroundStyle(Color.textPrimary)
                Text("\(obj.kind)\(obj.keyType.isEmpty ? "" : " · \(obj.keyType)") · id=\(obj.idHex)")
                    .font(.eidMonoSmall)
                    .foregroundStyle(Color.eidMuted)
                    .textSelection(.enabled)
            }
            Spacer()
            Button(role: .destructive) {
                Task { await deleteObject(obj) }
            } label: {
                Image(systemName: "trash").font(.system(size: 13))
            }
            .buttonStyle(.secondary)
            .disabled(objectsLoading)
        }
        .padding(.vertical, 8)
    }

    private func objectIcon(_ kind: String) -> String {
        switch kind {
        case "certificate": return "doc.text"
        case "privateKey":  return "key.fill"
        case "publicKey":   return "key"
        default:            return "doc"
        }
    }

    // MARK: - Generate signing key card

    private var generateKeyCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Tokens_GenKey_Section"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text(loc.t("Tokens_GenKey_Hint")).labelStyle()

                pinField(loc.t("Tokens_Objects_Pin"), binding: $objectsPIN)

                Button {
                    Task { await generateKey() }
                } label: {
                    HStack(spacing: 6) {
                        if genKeyLoading { ProgressView().controlSize(.small) }
                        Image(systemName: "key.viewfinder")
                        Text(loc.t("Tokens_GenKey_Action"))
                    }
                }
                .buttonStyle(.primary)
                .disabled(genKeyLoading || objectsPIN.isEmpty)
            }
        }
    }

    // MARK: - Logic

    private var canProvision: Bool {
        pin.count >= 6 && pin == pinConfirm && puk.count >= 8
    }

    private var canChangeUserPIN: Bool {
        curUserPIN.count >= 4 && newUserPIN.count >= 4 && newUserPIN == newUserPINConfirm
    }

    private var canUnlockPIN: Bool {
        unlockSOPIN.count >= 4 && unlockNewPIN.count >= 4 && unlockNewPIN == unlockNewPINConfirm
    }

    private var canChangeSOPIN: Bool {
        curSOPIN.count >= 4 && newSOPIN.count >= 4 && newSOPIN == newSOPINConfirm
    }

    private func changeUserPIN() async {
        changingUserPIN = true
        userPINStatus = ""; userPINIsError = false
        defer { changingUserPIN = false }
        do {
            try await TokenProvisioner().changeUserPIN(oldPIN: curUserPIN, newPIN: newUserPIN)
            userPINStatus = "User PIN амжилттай солигдлоо."
            curUserPIN = ""; newUserPIN = ""; newUserPINConfirm = ""
        } catch {
            userPINStatus = error.localizedDescription
            userPINIsError = true
        }
    }

    private func unlockPIN() async {
        unlockingPIN = true
        unlockStatus = ""; unlockIsError = false
        defer { unlockingPIN = false }
        do {
            try await TokenProvisioner().unlockUserPIN(soPIN: unlockSOPIN, newUserPIN: unlockNewPIN)
            unlockStatus = "PIN тайлагдлаа. Шинэ PIN-ээр нэвтэрнэ үү."
            unlockSOPIN = ""; unlockNewPIN = ""; unlockNewPINConfirm = ""
        } catch {
            unlockStatus = error.localizedDescription
            unlockIsError = true
        }
    }

    private func changeSOPIN() async {
        changingSOPIN = true
        soPINStatus = ""; soPINIsError = false
        defer { changingSOPIN = false }
        do {
            try await TokenProvisioner().changeSOPIN(oldPIN: curSOPIN, newPIN: newSOPIN)
            soPINStatus = "SO PIN амжилттай солигдлоо."
            curSOPIN = ""; newSOPIN = ""; newSOPINConfirm = ""
        } catch {
            soPINStatus = error.localizedDescription
            soPINIsError = true
        }
    }

    // MARK: - Object / key handlers

    private func loadObjects() async {
        objectsLoading = true; objectsStatus = ""; objectsIsError = false
        defer { objectsLoading = false }
        do {
            objects = try await TokenProvisioner().listTokenObjects(pin: objectsPIN)
        } catch {
            objects = []
            objectsStatus = error.localizedDescription
            objectsIsError = true
        }
    }

    private func importCertificate() async {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        guard panel.runModal() == .OK, let url = panel.url,
              let raw = try? Data(contentsOf: url) else { return }
        let der = [UInt8](derBytes(from: raw))

        objectsLoading = true; objectsStatus = ""; objectsIsError = false
        defer { objectsLoading = false }
        do {
            try await TokenProvisioner().importCertificatePKCS11(
                pin: objectsPIN, label: "eid-cert", certificateDER: der)
            objectsStatus = "Сертификат импортлогдлоо."
            objects = (try? await TokenProvisioner().listTokenObjects(pin: objectsPIN)) ?? objects
        } catch {
            objectsStatus = error.localizedDescription
            objectsIsError = true
        }
    }

    private func deleteObject(_ obj: GeregeTokenKit.PKCS11Module.TokenObjectInfo) async {
        objectsLoading = true; objectsStatus = ""; objectsIsError = false
        defer { objectsLoading = false }
        do {
            try await TokenProvisioner().deleteTokenObject(
                pin: objectsPIN, idHex: obj.idHex, kind: obj.kind)
            objects.removeAll { $0.idHex == obj.idHex && $0.kind == obj.kind }
        } catch {
            objectsStatus = error.localizedDescription
            objectsIsError = true
        }
    }

    private func generateKey() async {
        genKeyLoading = true; objectsStatus = ""; objectsIsError = false
        defer { genKeyLoading = false }
        do {
            try await TokenProvisioner().generateSigningKeyPKCS11(pin: objectsPIN, label: genKeyLabel)
            objectsStatus = "Түлхүүр амжилттай үүсгэгдлэа."
            objects = (try? await TokenProvisioner().listTokenObjects(pin: objectsPIN)) ?? objects
        } catch {
            objectsStatus = error.localizedDescription
            objectsIsError = true
        }
    }

    /// PEM → DER (markers + base64), эсвэл аль хэдийн DER бол хэвээр.
    private func derBytes(from data: Data) -> Data {
        if let str = String(data: data, encoding: .utf8), str.contains("-----BEGIN") {
            let body = str.split(whereSeparator: \.isNewline)
                .filter { !$0.contains("-----") }
                .joined()
            if let decoded = Data(base64Encoded: body) { return decoded }
        }
        return data
    }

    private func clearFields() {
        pin = ""; pinConfirm = ""; puk = ""
        statusMessage = ""; statusIsError = false
    }

    private func testConnection() async {
        isLoading = true
        statusMessage = ""; statusIsError = false
        defer { isLoading = false }
        let ok = await tokenManager.testConnection()
        statusMessage = ok ? "Холболт амжилттай" : "Холболт амжилтгүй"
        statusIsError = !ok
    }

    private func startProvision() async {
        isLoading = true
        statusMessage = ""; statusIsError = false
        provisionStep = .generating
        progressStage = "SM session + erase + filesystem + PIN/PUK"

        do {
            let provisioner = TokenProvisioner()
            try await provisioner.initialize(pin: pin, puk: puk)
            statusMessage = "PIN/PUK тогтоогдлоо"

            progressStage = "Түлхүүр хос үүсгэж байна"
            try await provisioner.generateKeyPair(pin: pin)
            statusMessage = "Түлхүүр хос үүсгэгдлэа"

            provisionStep = .done
            progressStage = ""
            clearFields()
        } catch {
            statusMessage = error.localizedDescription
            statusIsError = true
            provisionStep = .setPIN
            progressStage = ""
        }

        isLoading = false
    }

    private func startProvisionPKCS11() async {
        isLoading = true
        statusMessage = ""; statusIsError = false
        provisionStep = .generating
        progressStage = "PKCS#11: InitToken → InitPIN → EC keypair"

        let soPIN = puk
        let userPIN = pin

        do {
            try await TokenProvisioner().fullProvisionViaPKCS11(
                soPIN: soPIN, userPIN: userPIN
            )
            statusMessage = "PKCS#11 provision амжилттай — EC keypair бэлэн"
            provisionStep = .done
            progressStage = ""
            clearFields()
        } catch {
            statusMessage = error.localizedDescription
            statusIsError = true
            provisionStep = .setPINPKCS11
            progressStage = ""
        }

        isLoading = false
    }

    private func runCSRFlow() async {
        isLoading = true
        statusMessage = ""; statusIsError = false
        csrResultPEM = nil
        progressStage = "CSR үүсгэж байна..."
        defer { isLoading = false; progressStage = "" }

        let pin = csrPIN
        let nationalID = csrNationalID
        let label = csrLabel.isEmpty ? nil : csrLabel

        do {
            let provisioner = TokenProvisioner()
            let pem = try await provisioner.generateCSR(
                pin: pin, nationalID: nationalID, name: nationalID, keyLabel: label
            )
            csrResultPEM = pem
            // v3 RP-API-д token CSR enroll endpoint байхгүй — PEM-ийг
            // харуулаад хэрэглэгч гараар CA-д илгээнэ (clipboard/файл).
            statusMessage = "CSR үүсгэгдлэа (\(pem.count) bytes) — PEM-ийг хуулж CA-д илгээнэ үү"
        } catch {
            statusMessage = error.localizedDescription
            statusIsError = true
        }
    }

    private func testPKCS11() async {
        isLoading = true
        statusMessage = ""; statusIsError = false
        progressStage = "PKCS#11 слот шалгаж байна..."
        defer { isLoading = false; progressStage = "" }

        do {
            let result = try await Task.detached(priority: .userInitiated) { () -> String in
                let module = try PKCS11Module.open()
                try module.initialize()
                defer { module.finalize() }
                let slots = try module.getSlotList(tokenPresent: true)
                return "Слот олдлоо: \(slots.count)"
            }.value
            statusMessage = result
        } catch {
            statusMessage = "PKCS#11 алдаа: \(error.localizedDescription)"
            statusIsError = true
        }
    }

    private func eraseOnly() async {
        isLoading = true
        statusMessage = ""; statusIsError = false
        progressStage = "Токен устгаж байна..."

        do {
            try await TokenProvisioner().eraseCard()
            statusMessage = "Токен устгагдлаа (factory erase)"
        } catch {
            statusMessage = "Erase алдаа: \(error.localizedDescription)"
            statusIsError = true
        }

        progressStage = ""
        isLoading = false
    }
}

private enum ProvisionStep {
    case idle, setPIN, setPINPKCS11, generating, done
}
