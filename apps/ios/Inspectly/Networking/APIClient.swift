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
            self.decoder.dateDecodingStrategy = .iso8601
            return
        }
        self.baseURL = url
        self.configurationValid = !url.host!.hasSuffix(".invalid")
        self.decoder = JSONDecoder()
        self.decoder.keyDecodingStrategy = .convertFromSnakeCase
        self.decoder.dateDecodingStrategy = .iso8601
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

    private func authenticated(_ path: String, method: String = "GET", body: Data? = nil) async throws -> Data {
        let current = try await token()
        let (data, response) = try await transport(path, method: method, body: body, token: current)
        if response.statusCode == 401 {
            // If another call rotated the token while this request was in flight,
            // reuse the newer token rather than rotating a second time.
            let replacement: String
            if let newer = accessToken, newer != current {
                replacement = newer
            } else {
                replacement = try await refresh()
            }
            let (retry, status) = try await transport(path, method: method, body: body, token: replacement)
            if status.statusCode == 401 {
                accessToken = nil
                try keychain.clear()
                throw APIError.unauthorized
            }
            guard (200...299).contains(status.statusCode) else { throw APIError.server(status.statusCode) }
            return retry
        }
        guard (200...299).contains(response.statusCode) else {
            throw APIError.server(response.statusCode)
        }
        return data
    }

    private func token() async throws -> String {
        if let accessToken { return accessToken }
        return try await refresh()
    }

    private func request<T: Decodable>(_ path: String, as type: T.Type = T.self) async throws -> T {
        let data = try await authenticated(path)
        return try decoder.decode(type, from: data)
    }

    private func paginate<T: Decodable>(_ path: String, maxPages: Int = 30) async throws -> [T] {
        var cursor: String?
        var results: [T] = []
        for _ in 0..<maxPages {
            var nextPath = path + "?limit=100"
            if let cursor {
                guard let escaped = cursor.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) else {
                    throw APIError.invalidResponse
                }
                nextPath += "&cursor=" + escaped
            }
            let page: PagedResponse<T> = try await requestPage(nextPath)
            results.append(contentsOf: page.items)
            if !page.hasMore { return results }
            if page.nextCursor.isEmpty || page.nextCursor == cursor { throw APIError.invalidResponse }
            cursor = page.nextCursor
        }
        throw APIError.invalidResponse // Never silently show a partial collection.
    }

    private func requestPage<T: Decodable>(_ path: String) async throws -> PagedResponse<T> {
        let data = try await authenticatedURL(path)
        return try decoder.decode(PagedResponse<T>.self, from: data)
    }

    private func authenticatedURL(_ path: String) async throws -> Data {
        guard let question = path.firstIndex(of: "?") else { return try await authenticated(path) }
        let route = String(path[..<question])
        let query = String(path[path.index(after: question)...])
        guard !query.contains("#") else { throw APIError.invalidConfiguration }
        let current = try await token()
        var components = URLComponents(url: try url(route), resolvingAgainstBaseURL: false)!
        components.percentEncodedQuery = query
        guard let finalURL = components.url else { throw APIError.invalidConfiguration }
        var request = URLRequest(url: finalURL)
        request.timeoutInterval = 20
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue("Bearer " + current, forHTTPHeaderField: "Authorization")
        let (data, raw) = try await session.data(for: request)
        guard let http = raw as? HTTPURLResponse else { throw APIError.invalidResponse }
        if http.statusCode == 401 {
            let fresh: String
            if let newer = accessToken, newer != current { fresh = newer }
            else { fresh = try await refresh() }
            request.setValue("Bearer " + fresh, forHTTPHeaderField: "Authorization")
            let (retry, rawAgain) = try await session.data(for: request)
            guard let again = rawAgain as? HTTPURLResponse else { throw APIError.invalidResponse }
            guard again.statusCode == 200 else {
                if again.statusCode == 401 { try keychain.clear(); accessToken = nil; throw APIError.unauthorized }
                throw APIError.server(again.statusCode)
            }
            return retry
        }
        guard http.statusCode == 200 else { throw APIError.server(http.statusCode) }
        return data
    }

    func properties(in organizationID: String) async throws -> [PropertyRecord] {
        guard validUUID(organizationID) else { throw APIError.invalidConfiguration }
        return try await paginate("v1/organizations/\\(organizationID)/properties")
    }

    func inspections(in organizationID: String) async throws -> [InspectionRecord] {
        guard validUUID(organizationID) else { throw APIError.invalidConfiguration }
        return try await paginate("v1/organizations/\\(organizationID)/inspections")
    }

    func items(in organizationID: String, inspectionID: String) async throws -> [InspectionItemRecord] {
        guard validUUID(organizationID), validUUID(inspectionID) else { throw APIError.invalidConfiguration }
        return try await paginate("v1/organizations/\\(organizationID)/inspections/\\(inspectionID)/items")
    }

    func createProperty(in organizationID: String, named name: String) async throws {
        guard validUUID(organizationID), !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw APIError.invalidConfiguration
        }
        let body = try encode(["name": name])
        _ = try await authenticated("v1/organizations/\\(organizationID)/properties", method: "POST", body: body)
    }

    func createInspection(in organizationID: String, propertyID: String, type: String) async throws {
        guard validUUID(organizationID), validUUID(propertyID),
              ["move_in", "move_out", "turnover", "maintenance"].contains(type) else {
            throw APIError.invalidConfiguration
        }
        let body = try encode(["property_id": propertyID, "type": type])
        _ = try await authenticated("v1/organizations/\\(organizationID)/inspections", method: "POST", body: body)
    }

    func createItem(in organizationID: String, inspectionID: String, title: String, notes: String, severity: String, clientID: UUID) async throws {
        guard validUUID(organizationID), validUUID(inspectionID) else { throw APIError.invalidConfiguration }
        let body = try encode(["client_id": clientID.uuidString.lowercased(), "title": title,
                               "notes": notes, "severity": severity])
        _ = try await authenticated("v1/organizations/\\(organizationID)/inspections/\\(inspectionID)/items", method: "POST", body: body)
    }

    func updateItem(in organizationID: String, inspectionID: String, itemID: String, version: Int64, status: String) async throws {
        guard validUUID(organizationID), validUUID(inspectionID), validUUID(itemID),
              ["open", "in_progress", "resolved"].contains(status) else { throw APIError.invalidConfiguration }
        let body = try JSONSerialization.data(withJSONObject: ["version": version, "status": status])
        _ = try await authenticated("v1/organizations/\\(organizationID)/inspections/\\(inspectionID)/items/\\(itemID)", method: "PATCH", body: body)
    }

    func profile() async throws -> UserProfile {
        let data = try await authenticated("v1/me")
        return try decoder.decode(UserProfile.self, from: data)
    }

    func organizations() async throws -> [Organization] {
        let data = try await authenticated("v1/organizations")
        return try decoder.decode(OrganizationList.self, from: data).items
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
