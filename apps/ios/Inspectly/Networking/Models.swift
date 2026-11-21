import Foundation

struct AuthPair: Decodable {
    let accessToken: String
    let refreshToken: String
    let expiresAt: String
    let refreshExpiresAt: String
    let sessionId: String
}

struct UserProfile: Decodable {
    let id: String
    let phone: String
    let fullName: String
}

struct Organization: Identifiable, Decodable {
    let id: String
    let name: String
    let role: String
    let kind: String
    let country: String
}

struct OrganizationList: Decodable {
    let items: [Organization]
}

struct ErrorResponse: Decodable {
    let error: String
}

enum APIError: LocalizedError {
    case invalidConfiguration
    case invalidResponse
    case unauthorized
    case server(Int)
    case noSession
    case keychain(OSStatus)

    var errorDescription: String? {
        switch self {
        case .invalidConfiguration: return "Configure an HTTPS API endpoint before connecting."
        case .invalidResponse: return "The server returned an invalid response."
        case .unauthorized: return "Your session has expired. Sign in again."
        case .server(let code): return "Request failed (HTTP \(code))."
        case .noSession: return "Sign in to continue."
        case .keychain: return "Unable to securely save your session on this device."
        }
    }
}
