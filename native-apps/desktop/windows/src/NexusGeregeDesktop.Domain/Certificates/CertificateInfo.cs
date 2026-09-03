namespace NexusGeregeDesktop.Domain.Certificates;

public sealed record CertificateInfo(
    string Subject,
    string Issuer,
    string SerialNumberHex,
    DateTimeOffset NotBefore,
    DateTimeOffset NotAfter,
    string SignatureAlgorithm,
    string PublicKeyAlgorithm,
    int PublicKeyBits,
    string SpkiSha256Base64,
    string ThumbprintSha1Hex,
    string ThumbprintSha256Hex,
    IReadOnlyList<string> KeyUsages,
    IReadOnlyList<string> ExtendedKeyUsages,
    IReadOnlyList<string> SubjectAlternativeNames,
    string PemSource)
{
    public bool IsValidNow(DateTimeOffset utcNow) =>
        utcNow >= NotBefore && utcNow <= NotAfter;

    public bool IsExpired(DateTimeOffset utcNow) => utcNow > NotAfter;

    public bool IsNotYetValid(DateTimeOffset utcNow) => utcNow < NotBefore;

    public TimeSpan? RemainingValidity(DateTimeOffset utcNow) =>
        utcNow > NotAfter ? null : NotAfter - utcNow;
}
