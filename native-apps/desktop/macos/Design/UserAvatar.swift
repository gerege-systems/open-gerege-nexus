import SwiftUI
#if canImport(AppKit)
import AppKit
#endif
#if canImport(UIKit)
import UIKit
#endif

/// Base64 зургийг харуулах, зураг байхгүй бол initials.
///
/// Зургийн төрөл платформ бүрт өөр (`NSImage`/`UIImage`) тул `Image`-ийг эндээс
/// угсарна — үлдсэн бүх зүйл нь ижил. Энэ дүрсийг ширээ ба утас ХОЁУЛАА ашиглана.
struct UserAvatar: View {
    let photo: String?
    let initials: String
    let size: CGFloat
    /// Зураггүй үеийн үсэг ба дэвсгэрийн өнгө. Анхдагч нь ширээний брэндийн
    /// цэнхэр — тиймээс одоо байгаа дуудлагууд өөрчлөгдөхгүй. Утасны самбарын
    /// hero нь градиент цэнхэр дээр суудаг тул тэнд цагаан руу эргүүлнэ:
    /// цэнхэр дээр цэнхэр үсэг уншигдахгүй.
    var tint: Color = .primaryBlue
    var tintBackground: Color? = nil

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: size * 0.28)
                .fill(tintBackground ?? tint.opacity(0.1))
                .frame(width: size, height: size)

            if let image = decodedImage() {
                image
                    .resizable()
                    .scaledToFill()
                    .frame(width: size, height: size)
                    .clipShape(RoundedRectangle(cornerRadius: size * 0.28))
            } else {
                Text(initials.isEmpty ? "?" : initials)
                    .font(.system(size: size * 0.36, weight: .bold))
                    .foregroundStyle(tint)
            }
        }
        .frame(width: size, height: size)
    }

    private func decodedImage() -> Image? {
        guard let base64 = photo, !base64.isEmpty else { return nil }
        // DAN зураг "data:image/..." prefix-тэй байж болно
        let cleaned = base64.contains(",") ? String(base64.split(separator: ",").last ?? "") : base64
        guard let data = Data(base64Encoded: cleaned, options: .ignoreUnknownCharacters) else { return nil }
        #if canImport(AppKit) && !targetEnvironment(macCatalyst)
        return NSImage(data: data).map { Image(nsImage: $0) }
        #elseif canImport(UIKit)
        return UIImage(data: data).map { Image(uiImage: $0) }
        #else
        return nil
        #endif
    }
}
