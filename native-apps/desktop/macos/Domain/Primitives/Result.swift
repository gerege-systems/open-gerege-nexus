import Foundation

public typealias DomainResult<T> = Swift.Result<T, ApiError>

public extension Swift.Result where Failure == ApiError {
    var isSuccess: Bool {
        if case .success = self { return true }
        return false
    }

    var isFailure: Bool { !isSuccess }

    var value: Success? {
        if case .success(let v) = self { return v }
        return nil
    }

    var error: ApiError? {
        if case .failure(let e) = self { return e }
        return nil
    }
}
