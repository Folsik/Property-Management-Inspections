import Foundation

// Matches the existing /v1 JSON contract. The server owns versions and timestamps.
struct PagedResponse<T: Decodable>: Decodable {
    let items: [T]
    let nextCursor: String
    let hasMore: Bool
}

struct PropertyRecord: Identifiable, Decodable, Hashable {
    let id: String
    let name: String
    let version: Int64
    let updatedAt: Date
}

struct InspectionRecord: Identifiable, Decodable, Hashable {
    let id: String
    let propertyId: String
    let propertyName: String
    let type: String
    let status: String
    let createdAt: Date
    let updatedAt: Date
    let version: Int64
    let openItems: Int
    let totalItems: Int

    var displayType: String {
        type.replacingOccurrences(of: "_", with: " ").capitalized
    }
}

struct InspectionItemRecord: Identifiable, Decodable, Hashable {
    let id: String
    let clientId: String
    let title: String
    let notes: String
    let severity: String
    let status: String
    let version: Int64
    let createdAt: Date
    let updatedAt: Date
}

struct CreateResult: Decodable {
    let id: String
}

struct ItemMutationResult: Decodable {
    let id: String
    let version: Int64
    let status: String?
}

struct SyncPage: Decodable {
    let changes: [SyncChange]
    let nextCursor: String
    let hasMore: Bool
}

struct SyncChange: Decodable {
    let seq: Int64
    let entity: String
    let id: String
    let version: Int64
    let deleted: Bool
    let updatedAt: Date
}

// Safe for URL path interpolation: never accept user-supplied arbitrary paths.
func validUUID(_ value: String) -> Bool {
    UUID(uuidString: value) != nil
}
