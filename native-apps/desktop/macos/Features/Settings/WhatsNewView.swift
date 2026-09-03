import SwiftUI

/// What's New — шинэ хувилбарт юу өөрчлөгдсөнийг харуулах.
/// App эхлэхэд хувилбар өөрчлөгдсөн бол автоматаар нээгдэнэ.
struct WhatsNewView: View {
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        VStack(spacing: 0) {
            // Header
            VStack(spacing: 8) {
                Image(systemName: "sparkles")
                    .font(.system(size: 36))
                    .foregroundStyle(Color.primaryBlue)
                Text("Шинэчлэлт")
                    .font(.smartTitle)
                    .foregroundStyle(Color.textPrimary)
                Text("Gerege Desktop v\(currentVersion)")
                    .font(.smartBody)
                    .foregroundStyle(Color.textSecondary)
            }
            .padding(.top, 24)
            .padding(.bottom, 16)

            Divider()

            // Changelog
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    ForEach(changelog, id: \.version) { entry in
                        changelogEntry(entry)
                    }
                }
                .padding(24)
            }

            Divider()

            // Close
            Button("Ойлголоо") { dismiss() }
                .buttonStyle(.borderedProminent)
                .tint(Color.primaryBlue)
                .controlSize(.large)
                .padding(16)
        }
        .frame(width: 480, height: 520)
    }

    private func changelogEntry(_ entry: ChangelogEntry) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("v\(entry.version)")
                    .font(.system(size: 14, weight: .bold, design: .monospaced))
                    .foregroundStyle(Color.primaryBlue)
                Text(entry.date)
                    .font(.smartCaption)
                    .foregroundStyle(Color.textSecondary)
                Spacer()
            }

            ForEach(entry.items, id: \.self) { item in
                HStack(alignment: .top, spacing: 8) {
                    Image(systemName: item.icon)
                        .font(.system(size: 11))
                        .foregroundStyle(item.color)
                        .frame(width: 16, height: 16)
                    Text(item.text)
                        .font(.smartBody)
                        .foregroundStyle(Color.textPrimary)
                }
            }
        }
    }

    private var currentVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0"
    }
}

// MARK: - Changelog Data

struct ChangelogEntry {
    let version: String
    let date: String
    let items: [ChangelogItem]
}

struct ChangelogItem: Hashable {
    let icon: String
    let text: String
    let type: ItemType

    enum ItemType { case feature, fix, improvement }

    var color: Color {
        switch type {
        case .feature: return .primaryBlue
        case .fix: return .success
        case .improvement: return .accentGold
        }
    }
}

let changelog: [ChangelogEntry] = [
    ChangelogEntry(version: "1.0.9", date: "2026.09.01", items: [
        ChangelogItem(icon: "plus.circle.fill",
                      text: "Программ токен: төрийн вэбсайтад (ДАН) USB токенгүйгээр, утасны PIN2-оор нэвтрэх",
                      type: .feature),
        ChangelogItem(icon: "shield.checkered",
                      text: "Гарын үсгийн хүсэлт бүрд баталгаажуулах код харагдана — утсан дээрхтэй тулгана",
                      type: .improvement),
    ]),
    ChangelogEntry(version: "1.0.0", date: "2026.04.16", items: [
        ChangelogItem(icon: "plus.circle.fill", text: "3 аргаар нэвтрэх: РД + push, QR код, вэб хөтөч", type: .feature),
        ChangelogItem(icon: "plus.circle.fill", text: "Dashboard: avatar, stat card, devices, audit log", type: .feature),
        ChangelogItem(icon: "plus.circle.fill", text: "Байгууллага: бүртгэл, XYP lookup, гишүүд удирдлага", type: .feature),
        ChangelogItem(icon: "plus.circle.fill", text: "Cloud Sign: Google Drive + Dropbox + local файл", type: .feature),
        ChangelogItem(icon: "plus.circle.fill", text: "USB Токен: SM mutual auth, PIN/PUK init, key generation", type: .feature),
        ChangelogItem(icon: "plus.circle.fill", text: "Хэрэглэгчийн зураг (DAN photo)", type: .feature),
        ChangelogItem(icon: "plus.circle.fill", text: "Sparkle auto-update", type: .feature),
        ChangelogItem(icon: "shield.checkered", text: "FIPS AES-128 Secure Messaging дэмжлэг", type: .improvement),
        ChangelogItem(icon: "shield.checkered", text: "Release build obfuscation (class нэр нуух)", type: .improvement),
    ]),
]

// MARK: - Version check helper

struct WhatsNewChecker {
    private static let lastSeenVersionKey = "whatsNewLastSeenVersion"

    static var shouldShow: Bool {
        let current = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? ""
        let lastSeen = UserDefaults.standard.string(forKey: lastSeenVersionKey) ?? ""
        return current != lastSeen && !current.isEmpty
    }

    static func markAsSeen() {
        let current = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? ""
        UserDefaults.standard.set(current, forKey: lastSeenVersionKey)
    }
}
