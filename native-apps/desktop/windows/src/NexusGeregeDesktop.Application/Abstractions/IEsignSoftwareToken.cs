using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// "Программ токен" — физик USB токен байхгүй үед, апп-д нэвтэрсэн хэрэглэгчийн
/// identity-ээр (гар утасны threshold түлхүүр) ESIGN гарын үсэг зурах.
/// Хэрэглэгчийн гэрчилгээ + гарын үсгийг backend-аас (`/api/*`) авна.
/// Backend endpoint хараахан бэлэн биш бол дуудлагууд цэвэрхэн алдаа буцаана.
/// </summary>
public interface IEsignSoftwareToken
{
    /// <summary>Апп-д нэвтэрсэн (identity бэлэн) эсэх.</summary>
    bool IsAvailable { get; }

    /// <summary>Нэвтэрсэн хэрэглэгчийн гэрчилгээ + танигчийг backend-аас авах.</summary>
    Task<Result<EsignSoftwareIdentity>> GetIdentityAsync(CancellationToken ct = default);

    /// <summary>
    /// SHA256 digest-ийг гар утсаар зуруулж түүхий гарын үсэг авах. Гар утас руу
    /// баталгаажуулах push илгээж, баталгаажих кодыг <paramref name="onVerificationCode"/>-оор
    /// дамжуулан харуулна (хэрэглэгч утсан дээрээ баталгаажуулна).
    /// </summary>
    Task<Result<byte[]>> SignDigestAsync(
        byte[] sha256Digest,
        string displayText,
        Func<string, Task>? onVerificationCode = null,
        CancellationToken ct = default);
}

/// <summary>Программ токены нэвтэрсэн хэрэглэгчийн гэрчилгээ + ESIGN танигч.</summary>
public sealed record EsignSoftwareIdentity(
    byte[] CertDer,             // хэрэглэгчийн гэрчилгээ (DER)
    string Serial,              // "sn" — гэрчилгээний серийн дугаар (токен серийг орлоно)
    byte[] KeyId,               // "keyID" — SKI (SubjectPublicKey-ийн SHA1)
    EsignCertOption Option);    // сонгох/харуулах мэдээлэл
