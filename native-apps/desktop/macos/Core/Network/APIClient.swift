import Foundation
import os

/// First-party HTTP client — web backend-ийн нийтийн `/api/*` route-ууд руу.
///
/// iOS app шиг клиентэд ямар ч secret байхгүй: web сервер л Go RP-API-ийн
/// secret-ийг барьдаг. Desktop нь browser-тэй ижил нийтийн route-уудыг
/// дууддаг (rate limit тэдэн дээрээ адилхан үйлчилнэ).
actor APIClient {

    static let shared = APIClient()

    private let session: URLSession
    private let logger = Logger(subsystem: "mn.gerege.nexus.desktop", category: "APIClient")

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 30
        config.timeoutIntervalForResource = 180
        config.httpCookieAcceptPolicy = .never
        config.httpShouldSetCookies = false
        config.waitsForConnectivity = true
        #if os(iOS)
        config.shouldUseExtendedBackgroundIdleMode = true
        #endif
        // SEC-1: SPKI pinning (Release + `security.tlsPinning` идэвхтэй үед).
        self.session = URLSession(configuration: config,
                                  delegate: PinnedSessionDelegate(),
                                  delegateQueue: nil)
    }

    // MARK: - Transient error helper

    static func isTransientNetworkError(_ error: Error) -> Bool {
        if let urlError = error as? URLError {
            switch urlError.code {
            case .networkConnectionLost,
                 .timedOut,
                 .cannotConnectToHost,
                 .notConnectedToInternet,
                 .dnsLookupFailed,
                 .dataNotAllowed,
                 .internationalRoamingOff:
                return true
            default:
                return false
            }
        }
        let nsError = error as NSError
        if nsError.domain == NSURLErrorDomain {
            switch nsError.code {
            case NSURLErrorNetworkConnectionLost,
                 NSURLErrorTimedOut,
                 NSURLErrorCannotConnectToHost,
                 NSURLErrorNotConnectedToInternet,
                 NSURLErrorDNSLookupFailed,
                 NSURLErrorDataNotAllowed,
                 NSURLErrorInternationalRoamingOff:
                return true
            default:
                return false
            }
        }
        return false
    }

    // MARK: - Request

    func request<T: Decodable>(_ endpoint: Endpoint) async throws -> T {
        let request = try buildRequest(endpoint)

        logger.info("→ \(endpoint.method) \(endpoint.path)")
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw APIError.invalidResponse
        }
        logger.info("← \(http.statusCode) \(endpoint.path)")
        // Нэвтрэлт амжилттай болоход энд `session_token` ирнэ — ажлын мужийн
        // цорын ганц нэвтрэлт. Дуудагч бүрт биш ЭНД байгаа нь санаатай: аль
        // endpoint session өгөхийг клиент шийдэх ёсгүй, сервер өөрөө хэлнэ.
        WorkAreaSession.capture(from: http, for: request.url)

        if http.statusCode >= 400 {
            throw Self.serverError(status: http.statusCode, data: data)
        }

        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            logger.error("Decode алдаа: \(error.localizedDescription)")
            throw APIError.invalidResponse
        }
    }

    // MARK: - Session poll (auth)

    /// `/api/status?sessionId=&pollToken=`-г COMPLETE болтол давтан дуудна (сервер
    /// тал 1с барьдаг тул богино завсарлагатай loop). `pollToken` нь start/login-notify
    /// хариунд ирсэн байх ёстой. Web талын verify алдаа (`error` талбар) ирвэл шууд шидна.
    func waitForAuth(
        sessionID: String,
        pollToken: String,
        timeout: Duration = .seconds(300)
    ) async throws -> StatusResponse {
        let deadline = ContinuousClock.now + timeout
        var transientErrorCount = 0
        while ContinuousClock.now < deadline {
            try Task.checkCancellation()
            do {
                let status: StatusResponse = try await request(.status(sessionID: sessionID, pollToken: pollToken))
                transientErrorCount = 0
                if let err = status.error, !err.isEmpty {
                    throw APIError.server(200, err)
                }
                if status.isComplete { return status }
            } catch is CancellationError {
                throw CancellationError()
            } catch let error as APIError {
                if case .server(let status, _) = error, (status == 502 || status == 503 || status == 504) {
                    transientErrorCount += 1
                    logger.warning("Auth poll transient gateway error (\(status)), retrying in 1s...")
                    try await Task.sleep(for: .seconds(1))
                    continue
                }
                throw error
            } catch {
                if Self.isTransientNetworkError(error) {
                    transientErrorCount += 1
                    logger.warning("Auth poll transient network error (\(transientErrorCount)): \(error.localizedDescription), retrying in 1s...")
                    try await Task.sleep(for: .seconds(1))
                    continue
                }
                throw error
            }
            try await Task.sleep(for: .milliseconds(400))
        }
        throw APIError.timeout
    }

    // MARK: - Session poll (платформын өөрийн RP)

    /// `/api/v1/auth/eid/poll`-ыг терминал төлөв хүртэл давтана.
    ///
    /// Сервер тал хүсэлтийг 25 секунд хүртэл барьдаг (`eid.PollWindow`) тул энд
    /// богино завсарлага хэрэггүй — RUNNING буцаж ирэх нь тэр цонх дүүрсэн гэсэн
    /// үг. COMPLETE боловч баталгаажаагүй бол сервер 401 өгнө: тэр нь энд
    /// `APIError.server` болж шидэгдэнэ, дуудагч түүнийг л харуулна.
    ///
    /// Хэрэв app background руу шилжих үед эсвэл буцаж сэргэхэд сүлжээ түр
    /// тасарсан (`networkConnectionLost` / timeout) бол шууд уналгүйгээр
    /// дахин оролдоно — хүн eID апп дээр зөвшөөрөөд буцахад poll тасраагүй байна.
    func waitForPlatformAuth(
        sessionID: String,
        timeout: Duration = .seconds(300)
    ) async throws -> AuthPollResponse {
        let deadline = ContinuousClock.now + timeout
        var transientErrorCount = 0
        while ContinuousClock.now < deadline {
            try Task.checkCancellation()
            do {
                let poll: AuthPollResponse = try await request(.authPoll(sessionID: sessionID))
                transientErrorCount = 0
                if !poll.isRunning { return poll }
            } catch is CancellationError {
                throw CancellationError()
            } catch let error as APIError {
                if case .server(let status, _) = error, (status == 502 || status == 503 || status == 504) {
                    transientErrorCount += 1
                    logger.warning("Platform poll transient gateway error (\(status)), retrying in 1s...")
                    try await Task.sleep(for: .seconds(1))
                    continue
                }
                throw error
            } catch {
                if Self.isTransientNetworkError(error) {
                    transientErrorCount += 1
                    logger.warning("Platform poll transient network error (\(transientErrorCount)): \(error.localizedDescription), retrying in 1s...")
                    try await Task.sleep(for: .seconds(1))
                    continue
                }
                throw error
            }
        }
        throw APIError.timeout
    }

    // MARK: - Sign (PDF — web demo хуудастай яг ижил урсгал)

    /// `POST /api/sign-pdf-download` — эх PDF + sessionId + pollToken →
    /// тамгалагдсан (PAdES/PKCS#7 + баталгаажуулах хуудас) PDF bytes.
    /// `pollToken` заавал — sessionId-г мэдсэн этгээд иргэний cert/гарын
    /// үсгийг PDF-д шингээж авахаас сэргийлнэ (web талын шалгалт).
    func downloadStampedPdf(
        sessionID: String, pollToken: String, pdfData: Data, fileName: String
    ) async throws -> Data {
        try await multipart(
            path: "/api/sign-pdf-download",
            fields: ["sessionId": sessionID, "pollToken": pollToken],
            fileField: "file", fileName: fileName,
            fileType: "application/pdf", fileData: pdfData,
            headers: [:]
        )
    }

    // MARK: - Health (`GET /api/health`)

    func healthOK() async -> Bool {
        guard let url = URL(string: AppConfig.baseURL + "/api/health") else { return false }
        var request = URLRequest(url: url)
        request.timeoutInterval = 5
        guard let (_, response) = try? await session.data(for: request),
              let http = response as? HTTPURLResponse else { return false }
        return (200..<300).contains(http.statusCode)
    }

    // MARK: - Multipart helper

    private func multipart(
        path: String,
        fields: [String: String],
        fileField: String, fileName: String, fileType: String, fileData: Data,
        headers: [String: String]
    ) async throws -> Data {
        guard let url = URL(string: AppConfig.baseURL + path) else {
            throw APIError.invalidResponse
        }
        let boundary = "Boundary-\(UUID().uuidString)"
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
        request.setValue("Gerege-Desktop/1.0", forHTTPHeaderField: "User-Agent")
        for (k, v) in headers { request.setValue(v, forHTTPHeaderField: k) }

        var body = Data()
        func append(_ s: String) { body.append(Data(s.utf8)) }
        for (name, value) in fields {
            append("--\(boundary)\r\n")
            append("Content-Disposition: form-data; name=\"\(name)\"\r\n\r\n")
            append("\(value)\r\n")
        }
        append("--\(boundary)\r\n")
        append("Content-Disposition: form-data; name=\"\(fileField)\"; filename=\"\(fileName)\"\r\n")
        append("Content-Type: \(fileType)\r\n\r\n")
        body.append(fileData)
        append("\r\n--\(boundary)--\r\n")
        request.httpBody = body

        logger.info("→ POST \(path) (\(fileData.count) bytes)")
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw APIError.invalidResponse }
        logger.info("← \(http.statusCode) \(path)")
        if http.statusCode >= 400 {
            throw Self.serverError(status: http.statusCode, data: data)
        }
        return data
    }

    // MARK: - Build Request

    private func buildRequest(_ endpoint: Endpoint) throws -> URLRequest {
        var components = URLComponents(string: AppConfig.baseURL + endpoint.path)!
        components.queryItems = endpoint.queryItems
        guard let url = components.url else { throw APIError.invalidResponse }

        var request = URLRequest(url: url)
        request.httpMethod = endpoint.method

        if let body = try endpoint.body(), !body.isEmpty {
            request.httpBody = body
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue("Gerege-Desktop/1.0", forHTTPHeaderField: "User-Agent")
        // Платформын poll нь урт: сервер тал хүсэлтийг 25 секунд хүртэл барина
        // (`eid.PollWindow`) бөгөөд бодит хэмжилтээр 26с ирдэг. Session-ий
        // анхдагч 30с нь тэр цонхны ирмэг дээр — RUNNING бүрийг заримдаа
        // timeout болгож, нэвтрэлт «сүлжээ тасарлаа» гэж унана.
        if case .authPoll = endpoint { request.timeoutInterval = 60 }
        for (key, value) in endpoint.extraHeaders {
            request.setValue(value, forHTTPHeaderField: key)
        }
        return request
    }

    private static func serverError(status: Int, data: Data) -> APIError {
        if let err = try? JSONDecoder().decode(ErrorResponse.self, from: data),
           let msg = err.error, !msg.isEmpty {
            if msg == "rate_limited" {
                return .server(status, "Хэт олон оролдлого — 1 минут хүлээгээд дахин оролдоно уу.")
            }
            return .server(status, msg)
        }
        let text = String(data: data.prefix(300), encoding: .utf8) ?? ""
        return .server(status, text.isEmpty ? "Unknown error" : text)
    }
}

// MARK: - Error Models

/// Web route-ууд алдаанд `{"error": "message"}` буцаана.
struct ErrorResponse: Decodable {
    let error: String?
}

enum APIError: LocalizedError {
    case invalidResponse
    case server(Int, String)
    case timeout
    case networkError(String)

    var errorDescription: String? {
        switch self {
        case .invalidResponse:           return "Серверийн хариу буруу байна."
        case .server(let code, let msg): return "Сервер алдаа (\(code)): \(msg)"
        case .timeout:                   return "Хүлээх хугацаа дууслаа."
        case .networkError(let msg):     return "Сүлжээний алдаа: \(msg)"
        }
    }
}
