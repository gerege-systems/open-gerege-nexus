import SwiftUI

/// Gerege Nexus — iOS/iPadOS клиент.
///
/// Ширээний аппын хоёр зарчмыг хэвээр барина:
///
/// 1. **Клиентэд secret байхгүй.** Бүх дуудлага өөрийн web backend-ийн нийтийн
///    `/api/*` route-уудаар явна (хөтөчтэй яг ижил зам); RP-ийн нууцыг web
///    сервер л барина. Тиймээс энэ апп-ыг задлан үзсэн хүнд авах юм алга.
/// 2. **Identity нь Keychain дэх snapshot.** Bearer session байхгүй —
///    `documentNumber` нь дараагийн үйлдлийн бариул. Сэргээхэд Face ID хамгаална.
///
/// Ширээнээс ЯЛГААТАЙ нь ганцхан зүйл — нэвтрэлт. Мак дээр QR/РД push-ыг
/// ХӨРШ утас зөвшөөрдөг бол энд тэр утас нь ӨӨРӨӨ: eID Mongolia апп руу
/// app-to-app үсэрч, зөвшөөрөөд буцна (`MobileLoginView`).
@main
struct EIDGeregeApp: App {
    @StateObject private var appState = AppState()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(appState)
                // Хэл солиход бүх модыг дахин барина (араб/RTL-ийн үлдэц
                // хуучин модод үлддэг — ширээний аппад ч ижил дүрэм).
                .id(LocalizationService.shared.language)
                // eID апп зөвшөөрсний дараа `gerege-nexus://auth?sessionId=…`-ээр
                // буцаж ирнэ. Callback ирэхэд `authCallbackURL`-ээр дамжуулан
                // login дэлгэцэнд мэдэгдэж, түр тасарсан эсвэл хүлээгдэж буй poll-ыг
                // шуурхай дуусгана (нэвтрэлтийн үр дүнг link-ээс биш, серверээс баталгаажуулна).
                .onOpenURL { url in
                    appState.authCallbackURL = url
                }
        }
    }
}

/// Нэвтэрсэн эсэхээр хоёр л төлөв — ширээний `ContentView`-ийн дүйцэл.
struct RootView: View {
    @EnvironmentObject private var appState: AppState

    var body: some View {
        Group {
            switch appState.screen {
            case .login:     MobileLoginView()
            case .dashboard: MainTabView()
            }
        }
        .animation(.easeInOut(duration: 0.2), value: appState.screen)
    }
}
