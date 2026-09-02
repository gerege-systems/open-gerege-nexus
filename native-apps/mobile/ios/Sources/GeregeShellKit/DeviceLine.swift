import Foundation

/// iOS/iPadOS-ийн domain шугам.
///
/// Backend цор ганц. Гэхдээ төхөөрөмж бүр өөрийн host-оор ханддаг, тэр host нь
/// өөрийн `/api/v1`-ээ мөн үйлчилдэг. Ингэснээр webview доторх дуудлага
/// same-origin болж, session cookie нь `SameSite=Strict` хэвээр ажиллаж, CORS
/// preflight огт үүсэхгүй.
///
/// Бүртгэл: `native-apps/shared/device_lines.json`.
public enum GeregeDeviceLine {
    /// iOS-ийн domain шугам. 2026-08-12-нд асав: DNS (wildcard), nginx vhost,
    /// TLS гэрчилгээ, API-гийн origin allowlist дөрвүүлэн бэлэн.
    public static let origin = "https://mobile.nexus.gerege.mn"

    /// Ажиллахаа больсон хуучин анхдагчууд. Зөвхөн эдгээрийг зөөнө.
    ///
    /// `https://nexus.gerege.mn` энд ЗОРИУДААР байхгүй: тэр хаяг ижил backend
    /// руу очдог тул ажилласаар байгаа бөгөөд түүнийг хүчээр зөөвөл хөтчийн
    /// шугамыг санаатай сонгосон суулгацыг булааж авна.
    public static let supersededOrigins = ["https://api.nexus.gerege.mn", "http://localhost:8080", "https://ios.nexus.gerege.mn"]

    /// Хадгалагдсан утгыг шаардлагатай бол зөөж, эцсийн утгыг буцаана.
    public static func migrate(_ stored: String?) -> String {
        guard let stored, !stored.isEmpty else { return origin }
        return supersededOrigins.contains(stored) ? origin : stored
    }
}
