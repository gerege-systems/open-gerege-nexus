namespace NexusGeregeDesktop.Domain.Guardian;

/// <summary>Асран хамгаалагчийн зөвшөөрлөөр бүртгэгдсэн нэг хүүхэд (docs/CHILD_EID.md).</summary>
public sealed record GuardianChild(
    string Etsi,
    string RegNo,                    // ТОМ (харуулахад)
    string Name,                     // "Овог Нэр"
    string? BirthDate,               // YYYY-MM-DD
    bool Registered,                 // хүүхэд утсан дээрээ бүртгүүлсэн эсэх
    DateTimeOffset? CertNotAfter);   // гэрчилгээ дуусах (18 нас) — бүртгэлгүй бол null

/// <summary>Хүүхэд нэмэх — асран хамгаалагчийн PIN2 зөвшөөрлийн SIGN session.</summary>
public sealed record ChildAddSession(
    string SessionId,
    string VerificationCode,         // асран хамгаалагчийн утсанд харуулах VC
    string ChildName,
    string ChildRegNo);

/// <summary>Хүүхдийн утсанд дамжуулах нэг удаагийн бүртгэлийн код.</summary>
public sealed record ChildEnrollCode(
    string EnrollCode,
    int ExpiresInSeconds);
