import SwiftUI

struct SettingsView: View {
    @AppStorage(AppConfig.baseURLKey) private var apiURL: String = ""
    @AppStorage(AppState.biometricGateKey) private var biometricGate: Bool = true
    @ObservedObject private var loginItem = LoginItemManager.shared
    @AppStorage(AppActivation.backgroundModeKey) private var backgroundMode: Bool = false
    @AppStorage(AppState.esignUseTokenKey) private var esignUseToken: Bool = false
    @ObservedObject private var esign = EsignBridge.shared
    @State private var esignSelfTest: String = ""

    var body: some View {
        Form {
            Section("Эхлүүлэлт") {
                Toggle("Нэвтрэхэд автоматаар нээх", isOn: Binding(
                    get: { loginItem.launchAtLogin },
                    set: { loginItem.setLaunchAtLogin($0) }
                ))
                Toggle("Менюбар горим (Dock-д харагдахгүй)", isOn: $backgroundMode)
                    .onChange(of: backgroundMode) { _, on in AppActivation.apply(background: on) }
                Text("Менюбар горимд апп зөвхөн дэлгэцийн дээд менюбарт ажиллана — цонхыг хаасан ч тэндээ ажилласаар байна.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("Аюулгүй байдал") {
                Toggle("App нээхэд Touch ID асуух", isOn: $biometricGate)
            }

            Section("Программ токен (ESIGN)") {
                LabeledContent("Төлөв:") {
                    Text(esign.isRunning ? "ws://127.0.0.1:\(EsignBridge.port) дээр ажиллаж байна"
                                         : "Зогссон — нэвтэрсний дараа автоматаар асна")
                        .foregroundStyle(esign.isRunning ? .primary : .secondary)
                }
                Text("Төрийн вэбсайтад (ДАН / sso.gov.mn) физик USB токенгүйгээр цахим гарын үсгээр нэвтрэх. Хүсэлт бүрд утсан дээр PIN2-оор баталгаажуулна.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Toggle("USB токеноор зурах", isOn: $esignUseToken)
                Text("Асаалттай ба токен залгаастай бол ДАН-ий гарын үсгийг физик USB токеноор (токены гэрчилгээ + локал PIN) зурна; эс бөгөөс утас/PIN2-оор.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                HStack {
                    Button("Крипто өөрөө шалгах") { esignSelfTest = EsignCrypto.selfTest() }
                    if !esignSelfTest.isEmpty {
                        Text(esignSelfTest).font(.caption).textSelection(.enabled)
                    }
                }
            }

            Section("Сервер") {
                TextField("Сервер URL:", text: $apiURL, prompt: Text(AppConfig.baseURL))
                    .textFieldStyle(.roundedBorder)
                Text("e-ID web үйлчилгээний хаяг. App нь browser-тэй ижил нийтийн /api route-уудыг ашигладаг — нууц түлхүүр шаардахгүй.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("Тухай") {
                LabeledContent("Хувилбар:") {
                    Text(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0")
                }
                LabeledContent("Баг:") {
                    Text("Gerege Systems LLC")
                }
            }
        }
        .formStyle(.grouped)
        .frame(width: 460, height: 480)
    }
}
