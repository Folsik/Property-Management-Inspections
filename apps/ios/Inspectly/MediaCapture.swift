import AVFoundation
import Combine
import SwiftUI
import UIKit

enum MediaCaptureError: LocalizedError {
    case microphoneDenied
    case recordingFailed

    var errorDescription: String? {
        switch self {
        case .microphoneDenied: return "Microphone access is required to record voice notes."
        case .recordingFailed: return "Unable to start or finish the recording."
        }
    }
}

// Keeps audio in a real local file (required by URLSession file uploads).
@MainActor
final class VoiceRecorder: ObservableObject {
    @Published private(set) var isRecording = false
    private var recorder: AVAudioRecorder?

    func start() async throws {
        let granted = await withCheckedContinuation { (continuation: CheckedContinuation<Bool, Never>) in
            AVAudioSession.sharedInstance().requestRecordPermission { approved in
                continuation.resume(returning: approved)
            }
        }
        guard granted else { throw MediaCaptureError.microphoneDenied }
        let session = AVAudioSession.sharedInstance()
        try session.setCategory(.playAndRecord, mode: .default, options: [.defaultToSpeaker])
        try session.setActive(true)
        let file = FileManager.default.temporaryDirectory
            .appendingPathComponent("inspectly-\(UUID().uuidString).m4a")
        let settings: [String: Any] = [
            AVFormatIDKey: Int(kAudioFormatMPEG4AAC),
            AVSampleRateKey: 44_100.0,
            AVNumberOfChannelsKey: 1,
            AVEncoderBitRateKey: 64_000
        ]
        let instance = try AVAudioRecorder(url: file, settings: settings)
        instance.prepareToRecord()
        guard instance.record() else { throw MediaCaptureError.recordingFailed }
        recorder = instance
        isRecording = true
    }

    func stop() throws -> URL {
        guard let recorder else { throw MediaCaptureError.recordingFailed }
        recorder.stop()
        self.recorder = nil
        isRecording = false
        try? AVAudioSession.sharedInstance().setActive(false)
        return recorder.url
    }
}

struct NativeCamera: UIViewControllerRepresentable {
    let onCapture: (UIImage) -> Void
    @Environment(\.dismiss) private var dismiss

    func makeUIViewController(context: Context) -> UIImagePickerController {
        let picker = UIImagePickerController()
        picker.sourceType = UIImagePickerController.isSourceTypeAvailable(.camera) ? .camera : .photoLibrary
        picker.delegate = context.coordinator
        picker.allowsEditing = false
        return picker
    }

    func updateUIViewController(_ controller: UIImagePickerController, context: Context) {}

    func makeCoordinator() -> Coordinator { Coordinator(parent: self) }

    final class Coordinator: NSObject, UINavigationControllerDelegate, UIImagePickerControllerDelegate {
        let parent: NativeCamera

        init(parent: NativeCamera) { self.parent = parent }

        func imagePickerController(_ picker: UIImagePickerController, didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]) {
            if let image = info[.originalImage] as? UIImage { parent.onCapture(image) }
            parent.dismiss()
        }

        func imagePickerControllerDidCancel(_ picker: UIImagePickerController) {
            parent.dismiss()
        }
    }
}
