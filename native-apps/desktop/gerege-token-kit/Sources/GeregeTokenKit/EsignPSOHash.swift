import Foundation

/// ESPK токен (FEITIAN ePass2003 COS) дээрх **PSO HASH (`00 2A 90 A0`)**-ийн ачааллыг
/// Windows дундын программын (middleware) хийдэгтэй ЯГ ижил байтаар угсарна.
///
/// Windows нэвтрэлтийн PC/SC трэйсийг (SM-ийн session key-ээр задалж) урвуулан задлахад
/// PSO HASH-ийн ачаалал нь ГУРВАН DO болов:
///
///   `81 04 <N>` — SHA-256-д АЛЬ ХЭДИЙН боловсруулсан **бүтэн блокийн тоо** (64 байт/блок),
///                 big-endian 32 бит. (Трэйс: `0000002C` = 44 блок = 2816 байт.)
///   `90 20 <H>` — тэр 44 блокийн ДАРААХ SHA-256-ийн **завсрын төлөв** (H0..H7, 8×BE32 = 32 байт).
///   `80 <r> <D>` — үлдсэн боловсруулаагүй **сүүлчийн хэсэг** (0<r≤64 байт).
///
/// Карт `H`-ээс SHA-256-г үргэлжлүүлж `D`-г шингээгээд нийт битийн урт = (N·64 + r)·8 гэж
/// finalize хийдэг → яг `SHA256(P)`. Дараа нь PSO SIGN (`00 2A 9E 9A`) энэ digest дээр
/// RSA PKCS#1 v1.5 гарын үсэг зурна. Трэйсийн 256B гарын үсгийг токены гэрчилгээний нийтийн
/// түлхүүрээр урвуулахад дотор нь яг энэ digest-ийн DigestInfo гарсан → механизм батлагдсан.
public enum EsignPSOHash {

    // MARK: - SHA-256 core (завсрын төлөв гаргахад CryptoKit хүрэлцэхгүй тул өөрсдөө)

    private static let k: [UInt32] = [
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
        0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
        0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
        0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
        0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
        0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
        0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2]

    private static let iv: [UInt32] = [
        0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]

    /// Нэг 64-байт блокийг `state`-д шахна (SHA-256 compression). `block.count == 64`.
    private static func compress(_ state: inout [UInt32], _ block: ArraySlice<UInt8>) {
        var w = [UInt32](repeating: 0, count: 64)
        let base = block.startIndex
        for i in 0..<16 {
            let o = base + i * 4
            w[i] = (UInt32(block[o]) << 24) | (UInt32(block[o + 1]) << 16)
                 | (UInt32(block[o + 2]) << 8) | UInt32(block[o + 3])
        }
        for i in 16..<64 {
            let s0 = rotr(w[i-15], 7) ^ rotr(w[i-15], 18) ^ (w[i-15] >> 3)
            let s1 = rotr(w[i-2], 17) ^ rotr(w[i-2], 19) ^ (w[i-2] >> 10)
            w[i] = w[i-16] &+ s0 &+ w[i-7] &+ s1
        }
        var a = state[0], b = state[1], c = state[2], d = state[3]
        var e = state[4], f = state[5], g = state[6], h = state[7]
        for i in 0..<64 {
            let S1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)
            let ch = (e & f) ^ (~e & g)
            let t1 = h &+ S1 &+ ch &+ k[i] &+ w[i]
            let S0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)
            let maj = (a & b) ^ (a & c) ^ (b & c)
            let t2 = S0 &+ maj
            h = g; g = f; f = e; e = d &+ t1; d = c; c = b; b = a; a = t1 &+ t2
        }
        state[0] &+= a; state[1] &+= b; state[2] &+= c; state[3] &+= d
        state[4] &+= e; state[5] &+= f; state[6] &+= g; state[7] &+= h
    }

    private static func rotr(_ x: UInt32, _ n: UInt32) -> UInt32 { (x >> n) | (x << (32 - n)) }

    private static func beBytes(_ words: [UInt32]) -> [UInt8] {
        var out = [UInt8]()
        out.reserveCapacity(words.count * 4)
        for v in words {
            out.append(UInt8((v >> 24) & 0xFF)); out.append(UInt8((v >> 16) & 0xFF))
            out.append(UInt8((v >> 8) & 0xFF));  out.append(UInt8(v & 0xFF))
        }
        return out
    }

    // MARK: - PSO HASH TLV

    /// Нэгэн бүтэн `message` (=P payload)-д зориулж PSO HASH-ийн ачааллыг угсарна.
    /// Сүүлчийн хэсэг (`80` DO) ҮРГЭЛЖ 1..64 байт байхаар блокийн тоог сонгоно —
    /// урт нь 64-д хуваагдах үед ч сүүлчийн бүтэн блокийг тасалж `80`-д үлдээнэ.
    public static func hashTLV(for message: [UInt8]) -> [UInt8] {
        let n = message.count
        precondition(n >= 1, "PSO HASH-д хоосон message болохгүй")
        var blocks = n / 64
        var rem = n % 64
        if rem == 0 { blocks -= 1; rem = 64 }   // сүүлчийн блокийг заавал `80`-д үлдээнэ

        var state = iv
        var off = 0
        for _ in 0..<blocks {
            compress(&state, message[off ..< off + 64])
            off += 64
        }
        let midstate = beBytes(state)                     // 32 байт
        let tail = Array(message[off ..< n])              // rem байт (1..64)

        var tlv = [UInt8]()
        tlv += [0x81, 0x04,
                UInt8((blocks >> 24) & 0xFF), UInt8((blocks >> 16) & 0xFF),
                UInt8((blocks >> 8) & 0xFF),  UInt8(blocks & 0xFF)]
        tlv += [0x90, 0x20] + midstate
        tlv += [0x80, UInt8(rem)] + tail
        return tlv
    }

    /// Бүтэн SHA-256 (compression-оо CryptoKit-тэй тулгаж шалгах self-check-д).
    static func sha256(_ message: [UInt8]) -> [UInt8] {
        var state = iv
        var padded = message
        let bitLen = UInt64(message.count) * 8
        padded.append(0x80)
        while padded.count % 64 != 56 { padded.append(0x00) }
        for i in (0..<8).reversed() { padded.append(UInt8((bitLen >> (UInt64(i) * 8)) & 0xFF)) }
        var off = 0
        while off < padded.count { compress(&state, padded[off ..< off + 64]); off += 64 }
        return beBytes(state)
    }
}
