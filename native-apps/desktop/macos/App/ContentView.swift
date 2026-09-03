import SwiftUI

struct ContentView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var esign = EsignBridge.shared

    var body: some View {
        Group {
            switch appState.screen {
            case .login:
                LoginView()
            case .dashboard:
                DashboardView()
            }
        }
        .animation(.easeInOut(duration: 0.3), value: appState.screen)
        .task {
            EsignBridge.shared.start()
            #if DEBUG
            appState.startDebugTabSwitchIfRequested()
            #endif
        }
        // ESIGN гүүр нь браузераас ирсэн хүсэлтийг утас руу PIN2-оор явуулдаг. Хэрэглэгч
        // баталгаажуулах кодыг утсан дээрхтэй ТУЛГАХ ёстой тул хүсэлтийг чимээгүй хийж
        // болохгүй — цонх нь ард байсан ч харагдахаар overlay-гээр гаргана.
        .overlay { if let p = esign.pending { EsignPendingOverlay(pending: p) } }
        .animation(.easeInOut(duration: 0.2), value: esign.pending)
        // USB токен зам — гарын үсгийн PIN-ийг локалаар асууна (утас руу push явуулахгүй).
        .overlay { if let r = esign.pinRequest { EsignPINPromptOverlay(request: r) } }
        .animation(.easeInOut(duration: 0.2), value: esign.pinRequest)
    }
}

/// USB ESPK токен зам — гарын үсэг зурахын өмнө токены PIN-ийг локалаар асууна.
private struct EsignPINPromptOverlay: View {
    let request: EsignBridge.PINRequest
    @State private var pin = ""
    @FocusState private var focused: Bool

    var body: some View {
        ZStack {
            Color.black.opacity(0.45).ignoresSafeArea()
            VStack(spacing: 14) {
                Text("USB токеноор нэвтрэх")
                    .font(.headline)
                Text("Токены PIN кодоо оруулж гарын үсэг зурна уу.")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                if !request.regNum.isEmpty {
                    Text(request.regNum).font(.caption).foregroundStyle(.secondary)
                }
                SecureField("Токены PIN", text: $pin)
                    .textFieldStyle(.roundedBorder)
                    .frame(maxWidth: 220)
                    .focused($focused)
                    .onSubmit(submit)
                HStack(spacing: 10) {
                    Button("Цуцлах") { EsignBridge.shared.cancelPIN(); pin = "" }
                        .keyboardShortcut(.cancelAction)
                    Button("Зурах", action: submit)
                        .keyboardShortcut(.defaultAction)
                        .disabled(pin.isEmpty)
                }
            }
            .padding(28)
            .frame(maxWidth: 380)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 16))
        }
        .onAppear { focused = true }
    }

    private func submit() {
        guard !pin.isEmpty else { return }
        EsignBridge.shared.providePIN(pin)
        pin = ""
    }
}

/// ДАН-аас ирсэн гарын үсгийн хүсэлт — утсан дээр баталгаажуулах кодыг харуулна.
private struct EsignPendingOverlay: View {
    let pending: EsignBridge.Pending

    var body: some View {
        ZStack {
            Color.black.opacity(0.45).ignoresSafeArea()
            VStack(spacing: 14) {
                Text("Төрийн үйлчилгээнд нэвтрэх")
                    .font(.headline)
                Text("Утсан дээрх e-ID Mongolia апп-аас баталгаажуулаад PIN2-оо оруулна уу.")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                Text("БАТАЛГААЖУУЛАХ КОД")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text(pending.verificationCode)
                    .font(.system(size: 34, weight: .bold, design: .rounded))
                    .monospacedDigit()
                if !pending.regNum.isEmpty {
                    Text(pending.regNum).font(.caption).foregroundStyle(.secondary)
                }
                ProgressView().controlSize(.small)
            }
            .padding(28)
            .frame(maxWidth: 380)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 16))
        }
    }
}
