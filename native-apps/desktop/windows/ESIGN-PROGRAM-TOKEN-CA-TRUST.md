# Программ токен ESIGN нэвтрэлт — CA итгэлцлийн блокер (PKI/backend)

**Уншигч:** Gerege PKI / backend / ДАН интеграц хариуцсан баг.

**Товч:** Windows/macOS desktop-ийн "программ токен" (физик токенгүйгээр eID Mongolia
identity-ээр) ДАН/ESIGN нэвтрэлт нь **`sso.gov.mn` дээр "Таны тоон гэрчилгээ хүчин
төгөлдөр бус байна" гэж татгалздаг.** Клиент тал бүрэн зөв ажиллаж байгааг mitmproxy +
cert задлалтаар баталсан. Цорын ганц шалтгаан: **gov-ийн ESIGN нь `eID Mongolia Issuing CA`-г
итгэмжлэгдсэн тоон гарын үсгийн CA гэж танихгүй** (физик токены `Tridum Key Issuing SubCA`-г
танидаг). Энэ нь кодоор биш, **CA бүртгэл/итгэлцлээр** шийдэгдэнэ.

---

## 1. Нотолгоо — ижил endpoint, 2 өөр үр дүн

`POST https://sso.gov.mn/api/auth/callback/esign` (NextAuth "esign" credentials):

| Нэвтрэлт | callback/esign хариу |
|---|---|
| **Физик токен** | ✅ Амжилттай (login үргэлжилсэн, алдаагүй) |
| **Программ токен** | ❌ `{"error":"CredentialsSignin","code":{"message":"Таны тоон гэрчилгээ хүчин төгөлдөр бус байна.","type":"invalid_login_error"}}` |

Манай ESIGN гүүр хоёр тохиолдолд ч **хүчинтэй payload** (`{"status":"success","signature","cipher","meta"}`) буцаасан — ялгаа зөвхөн доторх **гэрчилгээ**-д.

## 2. Гэрчилгээний харьцуулалт (cert-ийг задалж авав)

| | Физик токен (АЖИЛЛАДАГ) | Программ токен (ТАТГАЛЗДАГ) |
|---|---|---|
| **Issuer (CA)** | `CN=Tridum Key Issuing SubCA, O=Tridum Key, OU=CA Service, C=MN` | `CN=eID Mongolia Issuing CA, O=Gerege Systems LLC, C=MN` |
| **Түлхүүр** | RSA 2048 | RSA (1.2.840.113549.1.1.1) |
| **Subject** | `O=…, OID.2.5.4.97=5731089 (organizationIdentifier), T=…, SN, G, CN, E=altangerel@gerege.com` | `C=MN, SN=TSENDDORJ, G=ERDENEBAT, SERIALNUMBER=PNOMN-111949212017` |
| **Extensions** | KeyUsage, EKU, **CertificatePolicies (2.5.29.32)**, CRL, AIA, AKI, **SKI (2.5.29.14)**, BasicConstraints | KeyUsage, EKU, BasicConstraints, AKI, AIA, CRL, **SubjectDirectoryAttributes (2.5.29.9)** |
| Төрөл | Байгууллага/ажилтны тоон гарын үсэг | Хувь хүний eID |

**Гол дүгнэлт:**
- Хоёулаа **RSA** — алгоритмын асуудал БИШ.
- Ялгаа нь **issuer (CA)**: gov `Tridum Key Issuing SubCA`-г итгэдэг, `eID Mongolia Issuing CA`-г итгэдэггүй.
- Хоёрдогч ялгаа: физик cert-д `CertificatePolicies` + `SKI` байна; eID cert-д байхгүй, оронд нь `SubjectDirectoryAttributes`. Физик cert-д `organizationIdentifier` байна. Гэхдээ эдгээр нь issuer блокерын дараах хоёрдогч зүйл.

## 3. Клиент тал — зөв, дууссан

- Программ токен: /api/certificates-аас eID Mongolia cert аваад, SHA256(P)-ийг гар утсаар (PIN2) зуруулж, ESIGN payload угсарч ws://59001-д зөв буцаана.
- Физик токен: PKCS#11-ээр токены cert + гарын үсэг — амжилттай нэвтэрдэг.
- Session-bound `/api/*` auth ({sessionId, pollToken}) зассан — dashboard/representations/certificates ажиллана.

Өөрөөр хэлбэл татгалзал нь клиентээс биш, **gov ESIGN-ий cert итгэлцлээс**.

## 4. Хийх ажил (PKI/backend — сонголт)

**A. `eID Mongolia Issuing CA`-г gov-ийн ESIGN итгэлцэлд бүртгүүлэх** (санал болгож буй).
sso.gov.mn / ДАН-ий тоон гарын үсгийн итгэмжлэгдсэн CA жагсаалтад eID Mongolia Issuing CA-г
(мөн шаардлагатай бол cert policy / EKU-г тэдний шаардлагад тааруулж) оруулна. Хийгдвэл
**программ токен ямар ч кодын өөрчлөлтгүйгээр шууд ажиллана** (клиент бэлэн).

**B. Эсвэл** eID Mongolia бүртгэлийн тоон гарын үсгийн cert-ийг **Tridum Key (итгэмжлэгдсэн)
CA-гаас** олгодог болгох (issuer chain-ийг тэдэнд аль хэдийн итгэдэг CA руу шилжүүлэх).

**Тодруулах асуултууд ДАН/gov талд:**
1. sso.gov.mn ESIGN callback ямар CA-нуудыг (root/subCA) итгэдэг вэ? `eID Mongolia Issuing CA`-г нэмэх процесс юу вэ?
2. Cert-д тавих нэмэлт шаардлага (CertificatePolicy OID, EKU, `organizationIdentifier` эсэх) байгаа юу?

## 5. Тестийн орчин (лавлагаа)

- RP: `eID Desktop` (client_id 80d5fd39…), redirect `sso-api.isf.mn/sso-api/sso/access-grant`.
- ESIGN type `bb4702f31917793f`, локал гүүр ws://127.0.0.1:59001 / wss://59005.
- Тестэд: физик токен (Tridum Key, altangerel@gerege.com, org 5731089) → амжилттай;
  программ токен (eID Mongolia CA, PNOMN-111949212017 / Erdenebat) → "хүчин төгөлдөр бус".
- Нотолгоо: mitmproxy capture (callback/esign хариу) + аппын cert-задлалтын лог.
