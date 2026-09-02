import Foundation

public enum PushRegistration {
    public static func registerAPNS(token: String, apiEndpoint: String, appID: String = "mn.gerege.nexus", session: URLSession = .shared) async throws {
        let root = apiEndpoint.trimmingCharacters(in: CharacterSet(charactersIn: "/")); let base = root.hasSuffix("/api/v1") ? root : root + "/api/v1"
        guard let url = URL(string: base + "/push-tokens") else { throw AuthHTTPError(status: -1, message: "API endpoint буруу байна") }
        var request=URLRequest(url:url);request.httpMethod="POST";request.setValue("application/json",forHTTPHeaderField:"Content-Type");request.httpBody=try JSONSerialization.data(withJSONObject:["provider":"APNS","token":token,"app_id":appID])
        let (data,response)=try await session.data(for:request);guard let http=response as? HTTPURLResponse,(200..<300).contains(http.statusCode) else {throw AuthHTTPError(status:(response as? HTTPURLResponse)?.statusCode ?? -1,message:String(data:data,encoding:.utf8) ?? "Push token бүртгэсэнгүй")}
    }
}
