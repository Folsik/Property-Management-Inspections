package api

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// DownloadWorkerAudio reads only an already verified media object belonging to
// the claimed job's organization and inspection. Secrets/URLs never enter jobs.
func (s *Server) DownloadWorkerAudio(ctx context.Context, org, inspection, media string) ([]byte, string, error) {
	var key, mime string
	var size int64
	err := s.db.QueryRow(ctx, `SELECT object_key,mime_type,size_bytes FROM media_attachments
 WHERE organization_id=$1 AND inspection_id=$2 AND id=$3 AND state='uploaded'
 AND deleted_at IS NULL AND mime_type IN ('audio/mp4','audio/mpeg')`, org, inspection, media).Scan(&key, &mime, &size)
	if err != nil {
		return nil, "", err
	}
	if size > 52428800 || size <= 0 {
		return nil, "", errors.New("invalid media size")
	}
	storage, err := storageEnv()
	if err != nil {
		return nil, "", err
	}
	res, err := s.storageDo(ctx, storage, "GET", key, "")
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", errors.New("verified audio download failed")
	}
	if res.ContentLength != size {
		return nil, "", errors.New("audio size mismatch")
	}
	bytes, err := io.ReadAll(io.LimitReader(res.Body, size+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(bytes)) != size {
		return nil, "", errors.New("audio length mismatch")
	}
	return bytes, mime, nil
}
