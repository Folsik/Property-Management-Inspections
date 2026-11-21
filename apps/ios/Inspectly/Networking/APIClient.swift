import Foundation

// One actor owns token rotation. Concurrent requests cannot reuse a consumed
// refresh token, which would cause the backend to revoke the device session.
actor APIClient {
    private let baseURL: URL
    private let session: URLSession
    private let keychain = KeychainRefreshStore()
    private let decoder: JSONDecoder
    private var accessToken: String?
    private var refreshTask: Task<AuthPair, Error>?

    init(session: URLSession = .shared) {
        self.session = session
        let configured = Bundle.main.object(forInfoDictionaryKey: "InspectlyAPIBaseURL") as? String ?? ""
        guard let url = URL(string: configured), url.host != nil,
              url.scheme == "https" || (url.scheme == "http" && ["localhost", "127.0.0.1"].contains(url.host ?? "")) else {
            self.baseURL = URL(string: "https://invalid.example")!
            self.configurationValid = false
            self.decoder = JSONDecoder()
            self.decoder.keyDecodingStrategy = .convertFromSnakeCase
            return
        }
        self.baseURL = url
        self.configurationValid = !url.host!.hasSuffix(".invalid")
        self.decoder = JSONDecoder()
        self.decoder.keyDecodingStrategy = .convertFromSnakeCase
    }

    private let configurationValid: Bool

    private func url(_ path: String) throws -> URL {
        guard configurationValid else { throw APIError.invalidConfiguration }
        guard path.hasPrefix("v1/"), !path.contains(".."), !path.contains("?") else {
            throw APIError.invalidConfiguration
        }
        return baseURL.appending(path: path)
    }

    private func transport(_ path: String, method: String = "GET", body: Data? = nil, token: String? = nil) async throws -> (Data, HTTPURLResponse) {
        var request = URLRequest(url: try url(path))
        request.httpMethod = method
        request.timeoutInterval = 20
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            request.httpBody = body
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        if let token {
            request.setValue("Bearer " + token, forHTTPHeaderField: "Authorization")
        }
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw APIError.invalidResponse }
        return (data, http)
    }

    private func encode(_ payload: [String: String]) throws -> Data {
        try JSONSerialization.data(withJSONObject: payload)
    }

    private func persist(_ pair: AuthPair) throws {
        // Persist replacement before exposing the new access token in memory.
        try keychain.save(pair.refreshToken)
        accessToken = pair.accessToken
    }

    func signIn(identifier: String, password: String) async throws {
        guard !identifier.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty, !password.isEmpty else {
            throw APIError.invalidResponse
        }
        let data = try encode(["identifier": identifier, "password": password])
        let (response, status) = try await transport("v1/auth/password/login", method: "POST", body: data)
        guard status.statusCode == 200 else {
            throw status.statusCode == 401 ? APIError.unauthorized : APIError.server(status.statusCode)
        }
        let pair = try decoder.decode(AuthPair.self, from: response)
        try persist(pair)
    }

    private func exchangeRefresh(_ previous: String) async throws -> AuthPair {
        let body = try encode(["refresh_token": previous])
        let (response, status) = try await transport("v1/auth/refresh", method: "POST", body: body)
        guard status.statusCode == 200 else {
            throw status.statusCode == 401 ? APIError.unauthorized : APIError.server(status.statusCode)
        }
        return try decoder.decode(AuthPair.self, from: response)
    }

    private func refresh() async throws -> String {
        if let active = refreshTask {
            let pair = try await active.value
            return pair.accessToken
        }
        guard let stored = try keychain.read() else { throw APIError.noSession }
        let task = Task { try await exchangeRefresh(stored) }
        refreshTask = task
        defer { refreshTask = nil }
        do {
            let pair = try await task.value
            try persist(pair)
            return pair.accessToken
        } catch APIError.unauthorized {
            try keychain.clear()
            accessToken = nil
            throw APIError.unauthorized
        }
    }

    func restore() async throws {
        _ = try await refresh()
    }

    private func authenticated(_ path: String) async throws -> Data {
        let current: String
        if let token = accessToken {
            current = token
        } else {
            current = try await refresh()
        }
        let (data, response) = try await transport(path, token: current)
        if response.statusCode == 401 {
            // Exactly one retry, with a serialized refresh operation.
            let replacement = try await refresh()
            let (retry, status) = try await transport(path, token: replacement)
            if status.statusCode == 401 {
                accessToken = nil
                try keychain.clear()
                throw APIError.unauthorized
            }
            guard status.statusCode == 200 else { throw APIError.server(status.statusCode) }
            return retry
        }
        guard response.statusCode == 200 else { throw APIError.server(response.statusCode) }
        return data
    }

    func profile() async throws -> UserProfile {
        try decoder.decode(UserProfile.self, from: authenticated("v1/me"))
    }

    func organizations() async throws -> [Organization] {
        try decoder.decode(OrganizationList.self, from: authenticated("v1/organizations")).items
    }

    func signOut() async throws {
        if let current = try keychain.read() {
            let body = try encode(["refresh_token": current])
            let (_, response) = try await transport("v1/auth/refresh/logout", method: "POST", body: body)
            guard response.statusCode == 200 || response.statusCode == 401 else {
                throw APIError.server(response.statusCode)
            }
        }
        try keychain.clear()
        accessToken = nil
    }
}
