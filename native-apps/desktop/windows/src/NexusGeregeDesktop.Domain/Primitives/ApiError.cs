namespace NexusGeregeDesktop.Domain.Primitives;

public sealed record ApiError(ApiErrorCode Code, string Message, string? Detail = null)
{
    public static ApiError Network(string message, string? detail = null) =>
        new(ApiErrorCode.Network, message, detail);

    public static ApiError Unauthorized(string message, string? detail = null) =>
        new(ApiErrorCode.Unauthorized, message, detail);

    public static ApiError CertificatePinFailure(string message, string? detail = null) =>
        new(ApiErrorCode.CertificatePinFailure, message, detail);

    public static ApiError BadRequest(string message, string? detail = null) =>
        new(ApiErrorCode.BadRequest, message, detail);

    public static ApiError NotFound(string message, string? detail = null) =>
        new(ApiErrorCode.NotFound, message, detail);

    public static ApiError Server(string message, string? detail = null) =>
        new(ApiErrorCode.ServerError, message, detail);

    public static ApiError Timeout(string message, string? detail = null) =>
        new(ApiErrorCode.Timeout, message, detail);

    public static ApiError Cancelled(string message, string? detail = null) =>
        new(ApiErrorCode.Cancelled, message, detail);

    public static ApiError Internal(string message, string? detail = null) =>
        new(ApiErrorCode.Internal, message, detail);
}

public enum ApiErrorCode
{
    Network,
    Timeout,
    Unauthorized,
    CertificatePinFailure,
    BadRequest,
    NotFound,
    ServerError,
    Cancelled,
    Internal,
}
