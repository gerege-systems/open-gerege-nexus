import Foundation

struct NativeSettings: Codable {
    /// Ширээний domain шугам. 2026-09-02-оос хойш хаяг нь ПЛАТФОРМ биш
    /// FORM FACTOR-ыг нэрлэнэ: macOS ба Windows хоёр нэг шугам хуваалцана.
    ///
    /// Web ба API нэг host дээр байгаа нь санаатай: ажлын мужаас гарах дуудлага
    /// same-origin болж, session cookie нь `SameSite=Strict` хэвээр ажиллана.
    /// Бүртгэл: `native-apps/shared/device_lines.json`.
    static let lineOrigin = "https://desktop.nexus.gerege.mn"

    var schemaVersion = 5
    var launchAtLogin = false
    var language = "mn"
    var webEndpoint = NativeSettings.lineOrigin
    var apiEndpoint = NativeSettings.lineOrigin
    var printerTransport = "USB"
    var printerHost = ""
    var printerPort = 9100
    var serialPort = ""
    var baudRate = 9600
    var paperWidth = "80 mm"
    var scannerMode = "Keyboard wedge"
    var scannerSuffix = "Enter"
    var biometricLock = true
    var idleLockMinutes = 5
    var updateChannel = "Stable"
    var telemetry = true
    var deviceName = Host.current().localizedName ?? "Mac"
    var site = ""
    var deviceID = ""

    static let storageKey = "mn.gerege.nexus.native-settings.v1"
    static func load() -> NativeSettings {
        guard let data = UserDefaults.standard.data(forKey: storageKey),
              var value = try? JSONDecoder().decode(Self.self, from: data) else { return Self() }
        if value.schemaVersion < 2 {
            if value.webEndpoint == "http://localhost:3000" { value.webEndpoint = "https://nexus.gerege.mn" }
            if value.apiEndpoint == "http://localhost:8080" { value.apiEndpoint = "https://nexus.gerege.mn" }
            value.schemaVersion = 2
            value.save()
        }
        // v4: schemaVersion-ыг зөвхөн урагшлуулна. Хадгалагдсан хаягийг
        // хөндөхгүй — шугам асахаас өмнөх богино хугацаанд хадгалагдсан утга
        // (`https://nexus.gerege.mn`) нь ижил backend руу очдог тул ажилласаар
        // байна. Түүнийг хүчээр зөөвөл хөтчийн шугамыг САНААТАЙ сонгосон
        // суулгацыг булааж авах болно; шинэ суулгац `lineOrigin`-оор эхэлнэ.
        if value.schemaVersion < 4 {
            value.schemaVersion = 4
            value.save()
        }
        // v5: шугам платформоор биш form factor-оор нэрлэгдэх болов
        // (2026-09-02). `mac.nexus.gerege.mn` нь nginx дээр байхаа больсон тул
        // хадгалагдсан тэр хаяг нь ажиллахаа больсон анхдагч — түүнийг зөөнө.
        // Дээрх дүрэм хэвээр: хүн өөрөө сонгосон өөр хаягийг хөндөхгүй.
        if value.schemaVersion < 5 {
            let superseded = ["https://mac.nexus.gerege.mn"]
            if superseded.contains(value.webEndpoint) { value.webEndpoint = lineOrigin }
            if superseded.contains(value.apiEndpoint) { value.apiEndpoint = lineOrigin }
            value.schemaVersion = 5
            value.save()
        }
        return value
    }
    func save() {
        guard let data = try? JSONEncoder().encode(self) else { return }
        UserDefaults.standard.set(data, forKey: Self.storageKey)
    }
}
