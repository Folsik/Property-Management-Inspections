import Foundation

struct InspectionMedia: Identifiable, Decodable, Hashable {
    let id: String
    let mimeType: String
    let sizeBytes: Int64
    let state: String
    let version: Int64
    let createdAt: Date
    let updatedAt: Date

    var isAudio: Bool { mimeType.hasPrefix("audio/") }
}

struct MediaUploadTicket: Decodable {
    let id: String
    let uploadUrl: URL
    let method: String
    let headers: [String: String]
    let expiresInSeconds: Int
    let state: String
}

struct MediaDownloadTicket: Decodable {
    let downloadUrl: URL
    let mimeType: String
}
