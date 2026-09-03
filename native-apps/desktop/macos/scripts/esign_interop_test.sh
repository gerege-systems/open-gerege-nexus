#!/usr/bin/env bash
# ESIGN криптын interop шалгалт — аппын ЖИНХЭНЭ `Core/Esign/EsignCrypto.swift`-ийг компайл хийж,
# гаргасан cipher/meta-г **OpenSSL-ээр** (ДАН-ий сервер яг ингэж хийнэ) буцааж тайлна.
#
# Яагаад: Security.framework өөрөө seal → өөрөө unseal хийвэл өөрийнхөө буруутай тохироо ч
# "зөв" харагдана. Хөндлөнгийн RSA/AES хэрэгжүүлэлтээр тайлж байж л wire-нийцтэйг батална.
#
# Ажиллуулах:  bash scripts/esign_interop_test.sh
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT

# 1) "sso.gov.mn"-ий шифрлэлтийн гэрчилгээ (RSA-2048)
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$tmp/srv.key" -outform DER -out "$tmp/srv.der" \
  -days 1 -subj "/CN=sso.gov.mn/O=National Data Center/C=MN" 2>/dev/null

# 1b) "Иргэний" гэрчилгээ — sn/keyID гаргалтыг шалгахад (SubjectKeyIdentifier-тэй, EC түлхүүр:
#     eID-ийн бодит гэрчилгээ ECDSA тул тэр замыг шалгана)
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -keyout "$tmp/usr.key" \
  -outform DER -out "$tmp/usr.der" -days 1 -subj "/CN=Test Irgen/serialNumber=PNOMN-99010101/C=MN" \
  -addext "subjectKeyIdentifier=hash" 2>/dev/null

# 2) Аппын крипто модулиар P угсарч seal хийх
cat > "$tmp/main.swift" <<'SWIFT'
import Foundation
let certDER = try! Data(contentsOf: URL(fileURLWithPath: CommandLine.arguments[1]))
let dataJSON = EsignCrypto.indentedJSON(keys: ["_ott", "reg-num"],
                                        values: ["_ott": "ott-TEST-123", "reg-num": "ФА92040910"])
let p = EsignCrypto.payload(dataJSON: dataJSON, certificateB64: "dXNlckNlcnQ=", sn: "0A1B2C", keyIDB64: "a2V5SUQ=")
let sealed = try! EsignCrypto.seal(payload: p, serverCertDER: certDER)
let userDER = try! Data(contentsOf: URL(fileURLWithPath: CommandLine.arguments[2]))
print(EsignCrypto.selfTest())
print("SN=" + EsignCrypto.certificateSerial(userDER))
print("KEYID_HEX=" + EsignCrypto.certificateKeyID(userDER).map { String(format: "%02X", $0) }.joined())
print("P_B64=" + p.base64EncodedString())
print("CIPHER=" + sealed.cipherB64)
print("META=" + sealed.metaB64)
print("SHA256P=" + EsignCrypto.sha256(p).base64EncodedString())
SWIFT
swiftc -O Core/Esign/EsignCrypto.swift "$tmp/main.swift" -o "$tmp/esigntest"
"$tmp/esigntest" "$tmp/srv.der" "$tmp/usr.der" | tee "$tmp/out.txt" | head -1

# 3) СЕРВЕР тал: meta → IV‖K → cipher → P, дараа нь P-г JSON гэж унших
python3 - "$tmp" <<'PY'
import base64, hashlib, json, subprocess, sys
tmp = sys.argv[1]
v = dict(l.split("=", 1) for l in open(tmp + "/out.txt").read().splitlines() if "=" in l and l[0].isupper())
p, cipher, meta = (base64.b64decode(v[k]) for k in ("P_B64", "CIPHER", "META"))

open(tmp + "/meta.bin", "wb").write(meta)
blob = subprocess.run(["openssl", "pkeyutl", "-decrypt", "-inkey", tmp + "/srv.key",
                       "-pkeyopt", "rsa_padding_mode:pkcs1", "-in", tmp + "/meta.bin"],
                      capture_output=True, check=True).stdout
assert len(blob) == 32, f"IV||K урт {len(blob)} (32 байх ёстой)"
iv, key = blob[:16], blob[16:]
print("✓ meta → RSA-PKCS1 тайлагдаж IV(16)+K(16) гарлаа")

open(tmp + "/cipher.bin", "wb").write(cipher)
back = subprocess.run(["openssl", "enc", "-d", "-aes-128-cbc", "-K", key.hex(), "-iv", iv.hex(),
                       "-in", tmp + "/cipher.bin"], capture_output=True, check=True).stdout
assert back == p, "cipher тайлсан үр дүн P-тэй зөрлөө"
print(f"✓ cipher → AES-128-CBC/PKCS7 тайлагдаж P ({len(p)}B) яг тулав")

pj = json.loads(p)
assert list(pj.keys()) == ["data", "certificate", "sn", "keyID"], pj.keys()
inner = json.loads(pj["data"])
assert inner["_ott"] == "ott-TEST-123" and inner["reg-num"] == "ФА92040910", inner
print("✓ P нь зөв JSON, талбарын дараалал C#-тай ижил:", list(pj.keys()))
print("✓ P.data доторх _ott/reg-num сэргээгдэв")

assert base64.b64encode(hashlib.sha256(p).digest()).decode() == v["SHA256P"]
print("✓ SHA256(P) тэнцүү — утас руу явуулах digest зөв")

# 4) sn / keyID — Windows клиент (EsignCertParser)-тэй ижил семантик эсэхийг OpenSSL-ээр тулгана.
#    sn    = ASN.1 INTEGER-ийн утга (тэргүүлэх 00 padding-гүй), ТОМ hex
#    keyID = SubjectKeyIdentifier өргөтгөл
txt = subprocess.run(["openssl", "x509", "-inform", "DER", "-in", tmp + "/usr.der", "-noout",
                      "-serial", "-ext", "subjectKeyIdentifier"], capture_output=True, check=True).stdout.decode()
serial = [l.split("=", 1)[1] for l in txt.splitlines() if l.startswith("serial=")][0].strip().upper()
assert v["SN"] == serial, f"sn зөрлөө: {v['SN']} != {serial}"
print(f"✓ sn == openssl serial ({serial})")

ski = "".join(l.strip().replace(":", "") for l in txt.splitlines()
              if all(c in "0123456789ABCDEFabcdef: " for c in l) and ":" in l).upper()
assert ski, "openssl-оос SKI гарсангүй"
assert v["KEYID_HEX"] == ski, f"keyID зөрлөө: {v['KEYID_HEX']} != {ski}"
print(f"✓ keyID == гэрчилгээний SubjectKeyIdentifier ({ski[:16]}…)")
PY
echo "ESIGN interop шалгалт АМЖИЛТТАЙ"
