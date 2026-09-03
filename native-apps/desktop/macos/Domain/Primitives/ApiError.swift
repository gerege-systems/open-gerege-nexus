import Foundation

public enum ApiErrorCode: String, Sendable {
    case network
    case timeout
    case unauthorized
    case certificatePinFailure = "certificate_pin_failure"
    case badRequest = "bad_request"
    case notFound = "not_found"
    case serverError = "server_error"
    case cancelled
    case `internal` = "internal"
}

public struct ApiError: Error, Sendable, Equatable {
    public let code: ApiErrorCode
    public let message: String
    public let detail: String?

    public init(code: ApiErrorCode, message: String, detail: String? = nil) {
        self.code = code
        self.message = message
        self.detail = detail
    }

    public static func network(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .network, message: message, detail: detail)
    }
    public static func timeout(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .timeout, message: message, detail: detail)
    }
    public static func unauthorized(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .unauthorized, message: message, detail: detail)
    }
    public static func certificatePinFailure(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .certificatePinFailure, message: message, detail: detail)
    }
    public static func badRequest(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .badRequest, message: message, detail: detail)
    }
    public static func notFound(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .notFound, message: message, detail: detail)
    }
    public static func server(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .serverError, message: message, detail: detail)
    }
    public static func cancelled(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .cancelled, message: message, detail: detail)
    }
    public static func `internal`(_ message: String, detail: String? = nil) -> ApiError {
        .init(code: .internal, message: message, detail: detail)
    }
}

extension ApiError: LocalizedError {
    public var errorDescription: String? { message }
}
