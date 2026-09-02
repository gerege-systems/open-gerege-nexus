#if os(iOS)
import GeregeShellKit
import SwiftUI

public struct NativeLoginView: View {
    @ObservedObject private var auth: AuthController
    @Environment(\.openURL) private var openURL
    @State private var email = ""
    @State private var password = ""
    @State private var nationalID = ""

    public init(auth: AuthController) { self.auth = auth }

    public var body: some View {
        GeometryReader { geometry in
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    // The brand mark the web app uses, at the size their login
                    // screen gives it. Not tinted: the mark carries its own
                    // colour and flattening it would lose what identifies it.
                    Image("brand", bundle: .module)
                        .resizable()
                        .aspectRatio(contentMode: .fill)
                        .frame(width: 56, height: 56)
                        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))

                    Text("GEREGE / NEXUS")
                        .font(.caption.weight(.bold)).tracking(2)
                        .foregroundStyle(WalletTheme.Brand.hi)
                    Text("Таны баталгаатай\nажлын орчин")
                        .font(.system(.largeTitle, design: .rounded, weight: .semibold))
                        .foregroundStyle(WalletTheme.Text.primary)
                        .padding(.bottom, WalletTheme.Space.sm)

                    TextField(loginText("auth.field.email"), text: $email).textContentType(.emailAddress)
                        .keyboardType(.emailAddress).textInputAutocapitalization(.never).loginField()
                    SecureField(loginText("auth.field.password"), text: $password).textContentType(.password).loginField()
                    Button(loginText("auth.action.admin_sign_in")) { auth.password(email: email, password: password) }
                        .buttonStyle(GeregePrimaryButton()).disabled(isPending)

                    HStack { Rectangle().frame(height: 1); Text("eID Mongolia").font(.caption); Rectangle().frame(height: 1) }
                        .foregroundStyle(WalletTheme.Text.tertiary)
                        .padding(.vertical, WalletTheme.Space.sm)

                    TextField(loginText("auth.eid.reg_number"), text: $nationalID)
                        .textInputAutocapitalization(.characters).autocorrectionDisabled().loginField()
                    Button(loginText("auth.eid.send_request")) { auth.push(nationalID: nationalID) }
                        .buttonStyle(GeregePrimaryButton()).disabled(isPending)
                    Button(loginText("auth.action.app_to_app")) {
                        auth.appToApp(callbackURL: "https://nexus.gerege.mn/auth/eid/callback")
                    }.buttonStyle(GeregeSecondaryButton()).disabled(isPending)

                    status
                    if isPending { Button(loginText("auth.action.cancel"), action: auth.cancel).frame(maxWidth: .infinity, alignment: .trailing) }
                }
                .frame(maxWidth: geometry.size.width > 700 ? 520 : 420, alignment: .leading)
                .padding(WalletTheme.Space.xl)
                .frame(maxWidth: .infinity, minHeight: geometry.size.height)
            }
        }
        .background(WalletTheme.Surface.page.ignoresSafeArea())
        .onChange(of: auth.phase) { phase in
            if case .waiting(_, let link?) = phase { openURL(link) }
        }
    }

    private var isPending: Bool {
        if case .starting = auth.phase { return true }
        if case .waiting = auth.phase { return true }
        return false
    }

    @ViewBuilder private var status: some View {
        let message: String = switch auth.phase {
        case .idle: ""
        case .starting: loginText("auth.message.starting")
        case .waiting(let code, _): "eID апп дээрх кодтой тулгана уу  ·  \(code)"
        case .success: loginText("auth.message.success")
        case .expired: loginText("auth.message.expired")
        case .refused: loginText("auth.message.refused")
        case .error(let error): error
        }
        if !message.isEmpty {
            Text(message).font(.body.monospacedDigit())
                .foregroundStyle(WalletTheme.Text.primary)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(WalletTheme.Space.lg)
                .background(WalletTheme.Surface.card,
                            in: RoundedRectangle(cornerRadius: WalletTheme.Radius.sm))
                .accessibilityLabel(message)
        }
    }
}

private func loginText(_ key: String.LocalizationValue) -> String {
    String(localized: key, table: "Login", bundle: .module)
}

private extension View {
    /// Their input treatment: a card-coloured field inside a hairline outline,
    /// so a form reads as a set of surfaces rather than as a set of boxes.
    func loginField() -> some View {
        self.padding(WalletTheme.Space.md + 2)
            .background(WalletTheme.Surface.card,
                        in: RoundedRectangle(cornerRadius: WalletTheme.Radius.sm))
            .overlay(RoundedRectangle(cornerRadius: WalletTheme.Radius.sm)
                .stroke(WalletTheme.Surface.border))
    }
}
private struct GeregePrimaryButton: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label.fontWeight(.semibold)
            .frame(maxWidth: .infinity)
            .padding(WalletTheme.Space.md + 2)
            // Pressed steps to the deep end of the brand family rather than
            // fading the primary, which on a dark surface reads as disabled.
            .background(configuration.isPressed ? WalletTheme.Brand.lo : WalletTheme.Brand.primary,
                        in: RoundedRectangle(cornerRadius: WalletTheme.Radius.sm))
            .foregroundStyle(WalletTheme.Brand.onBrand)
    }
}
private struct GeregeSecondaryButton: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .frame(maxWidth: .infinity)
            .padding(WalletTheme.Space.md + 1)
            .overlay(RoundedRectangle(cornerRadius: WalletTheme.Radius.sm)
                .stroke(WalletTheme.Brand.hi))
            .foregroundStyle(WalletTheme.Brand.hi)
    }
}
#endif
