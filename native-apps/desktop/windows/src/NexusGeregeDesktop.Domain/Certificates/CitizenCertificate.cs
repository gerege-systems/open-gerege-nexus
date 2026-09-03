namespace NexusGeregeDesktop.Domain.Certificates;

/// <summary>
/// Нэвтэрсэн иргэний PKI гэрчилгээ — `POST /api/certificates` (web backend нь
/// RP-API-ийн `/v3/certificates/etsi/{etsi}`-руу first-party RP-ээр proxy хийнэ).
/// <see cref="CertificateInfo"/> нь DER-ээс задалсан X.509 талбарууд бол энэ нь
/// бүртгэлийн мета (төрөл, статус, түвшин) + түүхий DER.
/// </summary>
public sealed record CitizenCertificate(
    string DocumentNumber,
    string Type,                 // "SIGN" (PIN2, contentCommitment) | "AUTH" (PIN1, clientAuth)
    string SerialNumber,
    string CertificateLevel,     // "QUALIFIED" | "ADVANCED" | …
    string Status,               // "VALID" | "REVOKED" | "EXPIRED" | "SUSPENDED"
    DateTimeOffset? NotBefore,
    DateTimeOffset? NotAfter,
    string IssuerDn,
    string CertValueB64)         // base64 DER — хуучин бичлэгт хоосон байж болно
{
    public bool IsValid => Status.Equals("VALID", StringComparison.OrdinalIgnoreCase);

    public bool IsSigning => Type.Equals("SIGN", StringComparison.OrdinalIgnoreCase);
}

/// <summary>Гэрчилгээний статусын тоолол (backend-ийн `counts`).</summary>
public sealed record CitizenCertificateCounts(
    int Valid,
    int Revoked,
    int Expired,
    int Suspended,
    int Total);

/// <summary>`POST /api/certificates`-ийн бүтэн хариу.</summary>
public sealed record CitizenCertificateList(
    string PersonEtsi,
    CitizenCertificateCounts Counts,
    IReadOnlyList<CitizenCertificate> Certificates);
