import Foundation

// A missing or malformed deployment origin is an error, never a fallback.
// This is intentionally checked at runtime as well as in the release CI guard.
enum EndpointConfiguration {
    static func resolve(environment: String?, value: String?) -> URL? {
        guard let environment, let value, !value.isEmpty,
              value == value.trimmingCharacters(in: .whitespacesAndNewlines),
              !value.contains("$("), !value.contains("\\"),
              let parts = URLComponents(string: value),
              let scheme = parts.scheme?.lowercased(),
              let host = parts.host?.lowercased(),
              !host.isEmpty,
              parts.user == nil, parts.password == nil,
              parts.query == nil, parts.fragment == nil,
              parts.path.isEmpty || parts.path == "/",
              let url = parts.url else { return nil }

        if environment == "local" {
            #if DEBUG
            return scheme == "http" && ["localhost", "127.0.0.1"].contains(host)
                && (parts.port == nil || parts.port == 8080) ? url : nil
            #else
            return nil
            #endif
        }

        guard environment == "staging" || environment == "production",
              scheme == "https", parts.port == nil || parts.port == 443,
              host.contains("."), !host.hasPrefix("."), !host.hasSuffix("."),
              !host.hasSuffix(".invalid"), !host.hasSuffix(".test"),
              !host.hasSuffix(".example"), !host.hasSuffix(".local"),
              host != "localhost", host != "example.com",
              host != "127.0.0.1", host != "0.0.0.0",
              !host.hasPrefix("10."), !host.hasPrefix("192.168."),
              !host.hasPrefix("172."), !host.hasPrefix("169.254."),
              !host.hasPrefix("127."), !host.contains(":"),
              !host.allSatisfy({ $0.isNumber || $0 == "." })
        else { return nil }

        #if DEBUG
        // Never ship a staging or production origin with a local Debug binary.
        return nil
        #else
        return url
        #endif
    }
}
