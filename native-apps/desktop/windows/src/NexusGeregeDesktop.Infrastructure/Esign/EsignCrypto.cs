using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json.Nodes;

namespace NexusGeregeDesktop.Infrastructure.Esign;

/// <summary>
/// ESIGN-ий "ENCRYPTED_DATA_SIGN" (bb4702f31917793f) схемийн ЯГ хэрэгжүүлэлт.
/// (ESIGNClient.exe эх кодоос сэргээж, амьд ДАН нэвтрэлтээр батлав.)
///
///   P = JSON { "data": &lt;data объектийн JSON string&gt;, "certificate": &lt;b64 userCert DER&gt;,
///              "sn": &lt;токен серийн дугаар&gt;, "keyID": &lt;b64 CKA_ID&gt; }
///   signature = RSA-SHA256-PKCS1( P )          — токенээр (P-г бүхэлд нь sign)
///   K = random 16B (AES-128!), IV = random 16B
///   cipher    = AES-128-CBC/PKCS7( P )
///   meta      = RSA-PKCS1v1.5( IV || K )       — серверийн гэрчилгээ (32 байт)
/// </summary>
public static class EsignCrypto
{
    public sealed record Output(string Signature, string Cipher, string Meta);

    /// <param name="dataJson">P.data — хүсэлтийн data объектийн JSON string.</param>
    /// <param name="userCertDer">Хэрэглэгчийн гэрчилгээ (DER).</param>
    /// <param name="tokenSerial">Токен төхөөрөмжийн серийн дугаар.</param>
    /// <param name="keyIdRaw">Гарын үсгийн түлхүүрийн CKA_ID (түүхий байт).</param>
    /// <param name="serverCertDer">Серверийн шифрлэлтийн гэрчилгээ (DER).</param>
    /// <param name="signAsync">P-г токенээр RSA-SHA256 гарын үсэг зурах делегат.</param>
    public static async Task<Output> BuildAsync(
        string dataJson, byte[] userCertDer, string tokenSerial, byte[] keyIdRaw,
        byte[] serverCertDer, Func<byte[], Task<byte[]>> signAsync)
    {
        // 1) P
        var obj = new JsonObject
        {
            ["data"] = dataJson,
            ["certificate"] = Convert.ToBase64String(userCertDer),
            ["sn"] = tokenSerial,
            ["keyID"] = Convert.ToBase64String(keyIdRaw),
        };
        byte[] p = Encoding.UTF8.GetBytes(obj.ToJsonString());

        // 2) signature = RSA-SHA256-PKCS1( P )
        byte[] signature = await signAsync(p).ConfigureAwait(false);

        // 3) AES-128-CBC/PKCS7( P )
        using var aes = Aes.Create();
        aes.KeySize = 128;
        aes.Mode = CipherMode.CBC;
        aes.Padding = PaddingMode.PKCS7;
        aes.GenerateKey();
        aes.GenerateIV();
        byte[] cipher = aes.EncryptCbc(p, aes.IV);

        // 4) meta = RSA-PKCS1( IV || K )  серверийн cert-ээр
        byte[] keyBlob = new byte[aes.IV.Length + aes.Key.Length];
        Buffer.BlockCopy(aes.IV, 0, keyBlob, 0, aes.IV.Length);
        Buffer.BlockCopy(aes.Key, 0, keyBlob, aes.IV.Length, aes.Key.Length);
        using var serverCert = new X509Certificate2(serverCertDer);
        using RSA serverRsa = serverCert.GetRSAPublicKey()
            ?? throw new InvalidOperationException("Серверийн гэрчилгээнд RSA public key алга.");
        byte[] meta = serverRsa.Encrypt(keyBlob, RSAEncryptionPadding.Pkcs1);

        return new Output(
            Convert.ToBase64String(signature),
            Convert.ToBase64String(cipher),
            Convert.ToBase64String(meta));
    }

    /// <summary>data объектийг Newtonsoft-той ойролцоо indented JSON string болгох.</summary>
    public static string IndentedJson(JsonNode? node)
    {
        if (node is not JsonObject obj) return node?.ToJsonString() ?? "{}";
        var sb = new StringBuilder();
        sb.Append("{\r\n");
        var items = obj.ToList();
        for (int i = 0; i < items.Count; i++)
        {
            string valStr = items[i].Value is null ? "null"
                : items[i].Value!.GetValueKind() == System.Text.Json.JsonValueKind.String
                    ? "\"" + Esc(items[i].Value!.GetValue<string>()) + "\""
                    : items[i].Value!.ToJsonString();
            sb.Append("  \"").Append(Esc(items[i].Key)).Append("\": ").Append(valStr);
            if (i < items.Count - 1) sb.Append(',');
            sb.Append("\r\n");
        }
        sb.Append('}');
        return sb.ToString();
    }

    private static string Esc(string s)
    {
        var sb = new StringBuilder(s.Length + 8);
        foreach (char c in s)
            sb.Append(c switch
            {
                '"' => "\\\"",
                '\\' => "\\\\",
                '\b' => "\\b",
                '\f' => "\\f",
                '\n' => "\\n",
                '\r' => "\\r",
                '\t' => "\\t",
                _ => c < 0x20 ? "\\u" + ((int)c).ToString("x4") : c.ToString(),
            });
        return sb.ToString();
    }
}
