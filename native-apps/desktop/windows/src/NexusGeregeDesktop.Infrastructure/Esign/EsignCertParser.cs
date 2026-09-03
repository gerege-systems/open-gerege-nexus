using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using NexusGeregeDesktop.Application.Abstractions;

namespace NexusGeregeDesktop.Infrastructure.Esign;

/// <summary>
/// ESIGN гүүрт гэрчилгээ (DER)-ээс харуулах мэдээлэл (EsignCertOption) болон
/// ESIGN танигчид (sn, keyID) гаргах дундын туслах. Физик токен ба программ
/// токен хоёр адилхан задлалт ашиглана.
/// </summary>
internal static class EsignCertParser
{
    /// <summary>DER-ээс сонгох/харуулах мэдээлэл (эзэмшигч, РД, байгууллага, зориулалт).</summary>
    public static EsignCertOption ParseOption(byte[] der)
    {
        try
        {
            using var x = new X509Certificate2(der);
            string usage = "";
            foreach (var ext in x.Extensions)
                if (ext is X509KeyUsageExtension ku)
                    usage = (ku.KeyUsages & X509KeyUsageFlags.NonRepudiation) != 0 ? "Гарын үсэг"
                          : (ku.KeyUsages & X509KeyUsageFlags.DigitalSignature) != 0 ? "Нэвтрэлт" : "";
            return new EsignCertOption(
                Field(x.Subject, "CN"),
                Clean(FieldAny(x.Subject, "SERIALNUMBER", "OID.2.5.4.5")),
                Field(x.Subject, "O"),
                Clean(FieldAny(x.Subject, "OID.2.5.4.97", "organizationIdentifier")),
                usage, x.NotAfter);
        }
        catch { return new EsignCertOption("(гэрчилгээ)", "", "", "", "", DateTime.MinValue); }
    }

    /// <summary>ESIGN "keyID" — SubjectKeyIdentifier байвал түүнийг, үгүй бол public key-ийн SHA1.</summary>
    public static byte[] KeyId(byte[] der)
    {
        try
        {
            using var x = new X509Certificate2(der);
            foreach (var ext in x.Extensions)
                if (ext is X509SubjectKeyIdentifierExtension ski && !string.IsNullOrEmpty(ski.SubjectKeyIdentifier))
                    return Convert.FromHexString(ski.SubjectKeyIdentifier);
            return SHA1.HashData(x.GetPublicKey());
        }
        catch { return Array.Empty<byte>(); }
    }

    /// <summary>ESIGN "sn" — гэрчилгээний серийн дугаар (hex).</summary>
    public static string Serial(byte[] der)
    {
        try { using var x = new X509Certificate2(der); return x.SerialNumber ?? ""; }
        catch { return ""; }
    }

    private static string Clean(string v)
    {
        if (string.IsNullOrEmpty(v)) return "";
        int dash = v.IndexOf('-');
        return (dash >= 0 ? v[(dash + 1)..] : v).Trim();
    }

    private static string Field(string dn, string key)
    {
        foreach (var p in dn.Split(','))
        {
            var kv = p.Trim();
            if (kv.StartsWith(key + "=", StringComparison.OrdinalIgnoreCase)) return kv[(key.Length + 1)..].Trim();
        }
        return "";
    }

    private static string FieldAny(string dn, params string[] keys)
    {
        foreach (var k in keys) { var v = Field(dn, k); if (!string.IsNullOrEmpty(v)) return v; }
        return "";
    }
}
