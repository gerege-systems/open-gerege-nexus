using System.Diagnostics.CodeAnalysis;

namespace NexusGeregeDesktop.Domain.Primitives;

public readonly record struct Result<T>
{
    private readonly T? _value;
    private readonly ApiError? _error;

    private Result(T value)
    {
        _value = value;
        _error = null;
        IsSuccess = true;
    }

    private Result(ApiError error)
    {
        _value = default;
        _error = error;
        IsSuccess = false;
    }

    public bool IsSuccess { get; }

    public bool IsFailure => !IsSuccess;

    public T Value => IsSuccess
        ? _value!
        : throw new InvalidOperationException("Cannot access value on failed Result.");

    public ApiError Error => IsFailure
        ? _error!
        : throw new InvalidOperationException("Cannot access error on successful Result.");

    public static Result<T> Success(T value) => new(value);

    public static Result<T> Failure(ApiError error) => new(error);

    public bool TryGetValue([MaybeNullWhen(false)] out T value)
    {
        value = _value;
        return IsSuccess;
    }
}

public readonly record struct Result
{
    private readonly ApiError? _error;

    private Result(bool isSuccess, ApiError? error)
    {
        IsSuccess = isSuccess;
        _error = error;
    }

    public bool IsSuccess { get; }

    public bool IsFailure => !IsSuccess;

    public ApiError Error => IsFailure
        ? _error!
        : throw new InvalidOperationException("Cannot access error on successful Result.");

    public static Result Success() => new(true, null);

    public static Result Failure(ApiError error) => new(false, error);
}
