import PhotosUI
import PDFKit
import SwiftUI
import UIKit

struct InspectionMediaView: View {
    @ObservedObject var model: WorkspaceModel
    let inspection: InspectionRecord

    @StateObject private var recorder = VoiceRecorder()
    @State private var photoSelection: PhotosPickerItem?
    @State private var showCamera = false
    @State private var isUploading = false
    @State private var isLoadingReport = false
    @State private var reportBytes = Data()
    @State private var showReport = false
    @State private var localError: String?
    @State private var failedRecording: URL?

    var body: some View {
        List {
            Section("Capture") {
                PhotosPicker(selection: $photoSelection, matching: .images) {
                    Label("Choose a photo", systemImage: "photo.on.rectangle")
                }
                .disabled(isUploading)

                Button {
                    showCamera = true
                } label: {
                    Label("Take a photo", systemImage: "camera")
                }
                .disabled(!UIImagePickerController.isSourceTypeAvailable(.camera) || isUploading)

                Button {
                    if recorder.isRecording {
                        finishRecording()
                    } else {
                        Task { await startRecording() }
                    }
                } label: {
                    Label(recorder.isRecording ? "Stop and upload voice note" : "Record voice note",
                          systemImage: recorder.isRecording ? "stop.circle.fill" : "mic.fill")
                        .foregroundStyle(recorder.isRecording ? .red : .primary)
                }
                .disabled(isUploading)

                if let failedRecording {
                    Button("Retry voice upload") {
                        Task { await upload(file: failedRecording, mime: "audio/mp4", preserveOnFailure: true) }
                    }
                    .disabled(isUploading)
                }
                if isUploading {
                    ProgressView("Uploading and verifying media…")
                }
            }

            Section("Verified attachments") {
                if model.media.isEmpty {
                    Text("No attachments yet")
                        .foregroundStyle(.secondary)
                }
                ForEach(model.media) { media in
                    VStack(alignment: .leading, spacing: 10) {
                        HStack {
                            Image(systemName: media.isAudio ? "waveform" : "photo")
                                .foregroundStyle(.blue)
                            VStack(alignment: .leading, spacing: 3) {
                                Text(media.isAudio ? "Voice note" : "Photo")
                                    .font(.headline)
                                Text(media.state.capitalized + " · " + ByteCountFormatter.string(fromByteCount: media.sizeBytes, countStyle: .file))
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                            Spacer()
                            if media.state == "uploaded" {
                                Image(systemName: "checkmark.circle.fill").foregroundStyle(.green)
                            }
                        }
                        if media.isAudio && media.state == "uploaded" {
                            HStack {
                                Button("Transcribe") {
                                    Task {
                                        if await model.transcribe(inspectionID: inspection.id, mediaID: media.id) {
                                            localError = "Transcription queued. The PDF will be available after processing."
                                        }
                                    }
                                }
                                .buttonStyle(.bordered)
                                Button("View PDF") {
                                    Task { await openReport(mediaID: media.id) }
                                }
                                .buttonStyle(.bordered)
                                .disabled(isLoadingReport)
                            }
                        }
                    }
                    .padding(.vertical, 5)
                }
            }

            if let error = localError ?? model.message {
                Section("Status") {
                    Text(error)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .navigationTitle("Media & notes")
        .navigationBarTitleDisplayMode(.inline)
        .task { await model.loadMedia(inspectionID: inspection.id) }
        .refreshable { await model.loadMedia(inspectionID: inspection.id) }
        .onChange(of: photoSelection) { _, newSelection in
            guard let newSelection else { return }
            Task { await importPhoto(newSelection) }
        }
        .sheet(isPresented: $showCamera) {
            NativeCamera { image in
                Task { await uploadPhoto(image) }
            }
        }
        .sheet(isPresented: $showReport) {
            NavigationStack {
                NativePDFView(content: reportBytes)
                    .navigationTitle("Inspection report")
                    .navigationBarTitleDisplayMode(.inline)
                    .toolbar {
                        ToolbarItem(placement: .confirmationAction) {
                            Button("Close") { showReport = false }
                        }
                    }
            }
        }
    }

    private func importPhoto(_ item: PhotosPickerItem) async {
        do {
            guard let data = try await item.loadTransferable(type: Data.self),
                  let image = UIImage(data: data) else {
                throw APIError.invalidResponse
            }
            await uploadPhoto(image)
        } catch {
            localError = error.localizedDescription
        }
        photoSelection = nil
    }

    private func uploadPhoto(_ image: UIImage) async {
        guard let data = image.jpegData(compressionQuality: 0.85) else {
            localError = "Unable to encode a photo."
            return
        }
        let file = FileManager.default.temporaryDirectory
            .appendingPathComponent("inspectly-\(UUID().uuidString).jpg")
        do {
            try data.write(to: file, options: .atomic)
            await upload(file: file, mime: "image/jpeg")
        } catch {
            localError = error.localizedDescription
        }
    }

    private func startRecording() async {
        do {
            try await recorder.start()
            localError = nil
        } catch {
            localError = error.localizedDescription
        }
    }

    private func finishRecording() {
        do {
            let file = try recorder.stop()
            Task { await upload(file: file, mime: "audio/mp4", preserveOnFailure: true) }
        } catch {
            localError = error.localizedDescription
        }
    }

    private func upload(file: URL, mime: String, preserveOnFailure: Bool = false) async {
        guard !isUploading else { return }
        isUploading = true
        localError = nil
        defer { isUploading = false }
        let saved = await model.uploadMedia(inspectionID: inspection.id, file: file, mime: mime)
        if saved {
            try? FileManager.default.removeItem(at: file)
            if failedRecording == file { failedRecording = nil }
        } else if preserveOnFailure {
            failedRecording = file
            localError = "Upload was not confirmed. You can retry the voice note."
        } else {
            try? FileManager.default.removeItem(at: file)
            localError = "Photo upload failed. Please select the photo again."
        }
    }

    private func openReport(mediaID: String) async {
        isLoadingReport = true
        defer { isLoadingReport = false }
        do {
            let bytes = try await model.report(inspectionID: inspection.id, mediaID: mediaID)
            guard bytes.starts(with: Data("%PDF-".utf8)) else {
                throw APIError.invalidResponse
            }
            reportBytes = bytes
            showReport = true
            localError = nil
        } catch {
            localError = "Report not ready or unavailable: " + error.localizedDescription
        }
    }
}

private struct NativePDFView: UIViewRepresentable {
    let content: Data

    func makeUIView(context: Context) -> PDFView {
        let view = PDFView()
        view.autoScales = true
        view.backgroundColor = .systemBackground
        return view
    }

    func updateUIView(_ view: PDFView, context: Context) {
        view.document = PDFDocument(data: content)
    }
}
