import Foundation

public enum TokenError: LocalizedError {
    case readerNotConnected
    case cardNotFound
    case sessionFailed
    case invalidAPDU
    case transmitFailed(String)
    case authFailed
    case pinVerifyFailed(retriesLeft: Int)
    case pinBlocked
    case keyGenFailed
    case signFailed(String)
    case certificateNotFound
    case certificateImportFailed
    case unsupportedAlgorithm
    case secureMessagingFailed
    case unexpectedStatus(UInt16)

    public var errorDescription: String? {
        switch self {
        case .readerNotConnected:       return "Уншигч холбогдоогүй байна."
        case .cardNotFound:             return "Токен олдсонгүй."
        case .sessionFailed:            return "Сэссион эхлүүлэхэд алдаа гарлаа."
        case .invalidAPDU:              return "APDU команд буруу байна."
        case .transmitFailed(let msg):  return "APDU дамжуулалт амжилтгүй: \(msg)"
        case .authFailed:               return "Токен баталгаажуулалт амжилтгүй."
        case .pinVerifyFailed(let n):   return "PIN буруу байна. \(n) оролдлого үлдсэн."
        case .pinBlocked:               return "PIN блоклогдсон. PUK ашиглана уу."
        case .keyGenFailed:             return "Түлхүүр хос үүсгэхэд алдаа гарлаа."
        case .signFailed(let msg):      return "Гарын үсэг зурахад алдаа: \(msg)"
        case .certificateNotFound:      return "Сертификат олдсонгүй."
        case .certificateImportFailed:  return "Сертификат суулгахад алдаа гарлаа."
        case .unsupportedAlgorithm:     return "Дэмжигдэхгүй алгоритм."
        case .secureMessagingFailed:    return "Secure messaging алдаа."
        case .unexpectedStatus(let sw): return String(format: "Токен алдаа: SW=%04X", sw)
        }
    }
}
