package api

import (
	"bytes"
	"context"
 "crypto/rand"
 "encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Validating signatures guards against misleading Content-Type metadata.
// This is a bounded header check, not a replacement for malware scanning.
func validMediaSignature(mime string, data []byte) bool {
	switch mime {
	case "image/jpeg":
		return len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff})
	case "image/png":
		return len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10})
	case "image/heic":
		return len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) &&
			(bytes.Equal(data[8:12], []byte("heic")) || bytes.Equal(data[8:12], []byte("heix")) ||
				bytes.Equal(data[8:12], []byte("hevc")) || bytes.Equal(data[8:12], []byte("mif1")))
	case "audio/mp4":
		return len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) &&
			(bytes.Equal(data[8:12], []byte("M4A ")) || bytes.Equal(data[8:12], []byte("isom")) ||
				bytes.Equal(data[8:12], []byte("mp42")) || bytes.Equal(data[8:12], []byte("mp41")))
	case "audio/mpeg":
		return len(data) >= 3 && (bytes.Equal(data[:3], []byte("ID3")) ||
			(data[0] == 0xff && (data[1]&0xe0) == 0xe0))
	}
	return false
}

type mediaStorage struct{ endpoint, bucket, region, access, secret string }

func storageEnv() (mediaStorage, error) {
	c := mediaStorage{os.Getenv("S3_ENDPOINT"), os.Getenv("S3_BUCKET"),
		os.Getenv("S3_REGION"), os.Getenv("S3_ACCESS_KEY_ID"), os.Getenv("S3_SECRET_ACCESS_KEY")}
	if _, err := presignObjectURL(c.endpoint, c.bucket, c.region, c.access, c.secret,
		"probe", "HEAD", "", time.Now(), 60); err != nil {
		return mediaStorage{}, err
	}
	return c, nil
}

// Sign GET/HEAD/DELETE with the same AWS SigV4 query method as presigned PUT.
func presignObjectURL(endpoint, bucket, region, access, secret, key, method, mime string, now time.Time, ttl int) (string, error) {
	if method != "GET" && method != "HEAD" && method != "DELETE" && method != "PUT" {
		return "", errors.New("invalid S3 method")
	}
	if ttl < 1 || ttl > 300 {
		return "", errors.New("invalid signing lifetime")
	}
	if key == "" || strings.ContainsAny(key, "\\?#") || strings.Contains(key, "..") {
		return "", errors.New("invalid S3 object key")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return "", errors.New("invalid storage endpoint")
	}
	if bucket == "" || strings.ContainsAny(bucket, " /\\?#") || region == "" || access == "" || secret == "" {
		return "", errors.New("incomplete storage configuration")
	}
	date := now.UTC().Format("20060102")
	stamp := now.UTC().Format("20060102T150405Z")
	scope := date + "/" + region + "/s3/aws4_request"
	u.Path = "/" + bucket + "/" + key
	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", access+"/"+scope)
	q.Set("X-Amz-Date", stamp)
	q.Set("X-Amz-Expires", strconv.Itoa(ttl))
	headers := "host"
	canonHeaders := "host:" + u.Host + "\n"
	if method == "PUT" {
		if mime == "" {
			return "", errors.New("missing content type")
		}
		headers = "content-type;host"
		canonHeaders = "content-type:" + mime + "\nhost:" + u.Host + "\n"
	}
	q.Set("X-Amz-SignedHeaders", headers)
	u.RawQuery = q.Encode()
	canonical := method + "\n" + u.EscapedPath() + "\n" + u.RawQuery + "\n" + canonHeaders + "\n" + headers + "\nUNSIGNED-PAYLOAD"
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hexHash(canonical)
	signing := hmacSHA([]byte("AWS4"+secret), date)
	signing = hmacSHA(signing, region)
	signing = hmacSHA(signing, "s3")
	signing = hmacSHA(signing, "aws4_request")
	q.Set("X-Amz-Signature", fmt.Sprintf("%x", hmacSHA(signing, toSign)))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
// A CopyObject is server-authorized and moves verified bytes into a key for
// which clients never receive a presigned PUT. This closes the time-of-check
// versus time-of-use window from still-live staging upload tickets.
func presignCopyURL(storage mediaStorage,source,target string,now time.Time)(string,error){
 if source==""||target==""||strings.ContainsAny(source,"\\?#")||strings.ContainsAny(target,"\\?#"){return "",errors.New("invalid storage copy key")}
 u,err:=url.Parse(storage.endpoint)
 if err!=nil||u.Scheme!="https"||u.Host==""||u.User!=nil||u.RawQuery!=""||u.Fragment!=""||u.Path!=""{return "",errors.New("invalid storage endpoint")}
 date:=now.UTC().Format("20060102")
 stamp:=now.UTC().Format("20060102T150405Z")
 scope:=date+"/"+storage.region+"/s3/aws4_request"
 u.Path="/"+storage.bucket+"/"+target
 sourceHeader:="/"+storage.bucket+"/"+source
 q:=url.Values{}
 q.Set("X-Amz-Algorithm","AWS4-HMAC-SHA256")
 q.Set("X-Amz-Credential",storage.access+"/"+scope)
 q.Set("X-Amz-Date",stamp)
 q.Set("X-Amz-Expires","60")
 q.Set("X-Amz-SignedHeaders","host;x-amz-copy-source")
 u.RawQuery=q.Encode()
 canonical:="PUT\n"+u.EscapedPath()+"\n"+u.RawQuery+"\nhost:"+u.Host+"\nx-amz-copy-source:"+sourceHeader+"\n\nhost;x-amz-copy-source\nUNSIGNED-PAYLOAD"
 signedText:="AWS4-HMAC-SHA256\n"+stamp+"\n"+scope+"\n"+hexHash(canonical)
 key:=hmacSHA([]byte("AWS4"+storage.secret),date)
 key=hmacSHA(key,storage.region);key=hmacSHA(key,"s3");key=hmacSHA(key,"aws4_request")
 q.Set("X-Amz-Signature",fmt.Sprintf("%x",hmacSHA(key,signedText)))
 u.RawQuery=q.Encode()
 return u.String(),nil
}
func (s *Server) storageCopy(ctx context.Context,storage mediaStorage,source,target string)error{
 link,err:=presignCopyURL(storage,source,target,time.Now())
 if err!=nil{return err}
 req,err:=http.NewRequestWithContext(ctx,"PUT",link,nil)
 if err!=nil{return err}
 req.Header.Set("X-Amz-Copy-Source","/"+storage.bucket+"/"+source)
 res,err:=s.storageHTTPClient.Do(req)
 if err!=nil{return err}
 defer res.Body.Close()
 data,err:=io.ReadAll(io.LimitReader(res.Body,65537))
 if err!=nil{return err}
 if len(data)>65536{return errors.New("oversized storage copy response")}
 if res.StatusCode!=200||bytes.Contains(bytes.ToLower(data),[]byte("<error>")){
  return fmt.Errorf("storage copy failed: HTTP %d",res.StatusCode)
 }
 return nil
}
func newVerifiedKey(stage string)(string,error){
 raw:=make([]byte,16)
 if _,err:=rand.Read(raw);err!=nil{return "",err}
 i:=strings.LastIndex(stage,"/")
 if i<0{return "",errors.New("invalid stage key")}
 return stage[:i]+"/verified/"+hex.EncodeToString(raw),nil
}
func (s *Server) storageDo(ctx context.Context, storage mediaStorage, method, key string, rangeHeader string) (*http.Response, error) {
	url, err := presignObjectURL(storage.endpoint, storage.bucket, storage.region, storage.access, storage.secret, key, method, "", time.Now(), 60)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	return s.storageHTTPClient.Do(req)
}
func (s *Server) verifyStoredMedia(ctx context.Context, storage mediaStorage, key, mime string, size int64) (int, error) {
	head, err := s.storageDo(ctx, storage, "HEAD", key, "")
	if err != nil {
		return 503, err
	}
	io.Copy(io.Discard, io.LimitReader(head.Body, 1024))
	head.Body.Close()
	if head.StatusCode == http.StatusNotFound {
		return 409, errors.New("upload not found in storage")
	}
	if head.StatusCode != http.StatusOK {
		return 503, fmt.Errorf("storage HEAD returned %d", head.StatusCode)
	}
	if head.ContentLength != size {
		return 422, errors.New("stored file size does not match upload ticket")
	}
	typ := strings.ToLower(strings.TrimSpace(strings.Split(head.Header.Get("Content-Type"), ";")[0]))
	if typ != mime {
		return 422, errors.New("stored MIME type does not match upload ticket")
	}
	response, err := s.storageDo(ctx, storage, "GET", key, "bytes=0-63")
	if err != nil {
		return 503, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusPartialContent {
		return 503, fmt.Errorf("storage did not honor a bounded content range: %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65))
	if err != nil {
		return 503, err
	}
	if len(data) > 64 {
		return 503, errors.New("storage ignored requested byte range")
	}
	if !validMediaSignature(mime, data) {
		return 422, errors.New("file signature does not match MIME type")
	}
	return 200, nil
}
func (s *Server) authorizedMedia(w http.ResponseWriter, r *http.Request) (string, string, string, bool) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return "", "", "", false
	}
	org, inspection, id := r.PathValue("org"), r.PathValue("inspection"), r.PathValue("media")
	if !validID(inspection) || !validID(id) {
		fail(w, 400, "invalid media ID")
		return "", "", "", false
	}
	return org, inspection, id, true
}
func (s *Server) confirmMedia(w http.ResponseWriter, r *http.Request) {
	org, inspection, id, ok := s.authorizedMedia(w, r)
	if !ok {
		return
	}
	storage, err := storageEnv()
	if err != nil {
		fail(w, 503, "storage configuration unavailable")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var key, mime, state string
	var size int64
	var created time.Time
	err = tx.QueryRow(r.Context(), `SELECT object_key,mime_type,size_bytes,state,created_at
 FROM media_attachments WHERE organization_id=$1 AND inspection_id=$2 AND id=$3 FOR UPDATE`,
		org, inspection, id).Scan(&key, &mime, &size, &state, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "media not found")
		return
	}
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	if state == "uploaded" {
		write(w, 200, map[string]string{"id": id, "state": "uploaded"})
		return
	}
	if state != "pending" {
		fail(w, 409, "media is not pending")
		return
	}
	if time.Since(created) > 20*time.Minute {
		fail(w, 409, "upload ticket expired")
		return
	}
	status, err := s.verifyStoredMedia(r.Context(), storage, key, mime, size)
	if err != nil {
		if status == 422 {
			// Invalid uploads cannot be retrieved; leave remote object to the periodic
			// cleanup task so deletion failures can be retried independently.
			_, dbErr := tx.Exec(r.Context(), `UPDATE media_attachments SET state='rejected',rejected_at=now(),verification_error=$2 WHERE id=$1`, id, err.Error())
			if dbErr != nil {
				fail(w, 503, "database unavailable")
				return
			}
			if dbErr = tx.Commit(r.Context()); dbErr != nil {
				fail(w, 503, "database unavailable")
				return
			}
		}
		fail(w, status, err.Error())
		return
	}
	verifiedKey,copyErr:=newVerifiedKey(key)
 if copyErr!=nil{fail(w,503,"storage unavailable");return}
 if copyErr=s.storageCopy(r.Context(),storage,key,verifiedKey);copyErr!=nil{
  fail(w,503,"storage copy failed");return
 }
 // Independently verify the copied object before granting read permission.
 if checked,verifyErr:=s.verifyStoredMedia(r.Context(),storage,verifiedKey,mime,size);verifyErr!=nil{
  cleanup,deleteErr:=s.storageDo(r.Context(),storage,"DELETE",verifiedKey,"")
  if deleteErr==nil{io.Copy(io.Discard,io.LimitReader(cleanup.Body,1024));cleanup.Body.Close()}
  fail(w,checked,"verified copy unavailable");return
 }
	_, err = tx.Exec(r.Context(), `UPDATE media_attachments SET object_key=$2,state='uploaded',verified_at=now(),verification_error=NULL WHERE id=$1`, id,verifiedKey)
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, map[string]string{"id": id, "state": "uploaded"})
}
func (s *Server) downloadMedia(w http.ResponseWriter, r *http.Request) {
	org, inspection, id, ok := s.authorizedMedia(w, r)
	if !ok {
		return
	}
	var key, mime, state string
	err := s.db.QueryRow(r.Context(), `SELECT object_key,mime_type,state FROM media_attachments
 WHERE organization_id=$1 AND inspection_id=$2 AND id=$3`, org, inspection, id).Scan(&key, &mime, &state)
	if errors.Is(err, pgx.ErrNoRows) || state == "deleted" {
		fail(w, 404, "media not found")
		return
	}
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	if state != "uploaded" {
		fail(w, 409, "media is not verified")
		return
	}
	storage, err := storageEnv()
	if err != nil {
		fail(w, 503, "storage configuration unavailable")
		return
	}
	link, err := presignObjectURL(storage.endpoint, storage.bucket, storage.region, storage.access, storage.secret, key, "GET", "", time.Now(), 60)
	if err != nil {
		fail(w, 503, "storage signing failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, map[string]any{"download_url": link, "expires_in_seconds": 60, "mime_type": mime})
}
func (s *Server) deleteMedia(w http.ResponseWriter, r *http.Request) {
	org, inspection, id, ok := s.authorizedMedia(w, r)
	if !ok {
		return
	}
	storage, err := storageEnv()
	if err != nil {
		fail(w, 503, "storage configuration unavailable")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var key, state string
	err = tx.QueryRow(r.Context(), `SELECT object_key,state FROM media_attachments
 WHERE organization_id=$1 AND inspection_id=$2 AND id=$3 FOR UPDATE`, org, inspection, id).Scan(&key, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "media not found")
		return
	}
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	if state == "deleted" {
		write(w, 200, map[string]string{"id": id, "state": "deleted"})
		return
	}
	response, err := s.storageDo(r.Context(), storage, "DELETE", key, "")
	if err != nil {
		fail(w, 503, "storage unavailable")
		return
	}
	io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	response.Body.Close()
	if response.StatusCode != 200 && response.StatusCode != 204 && response.StatusCode != 404 {
		fail(w, 503, "storage deletion failed")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE media_attachments SET state='deleted',deleted_at=now() WHERE id=$1`, id)
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]string{"id": id, "state": "deleted"})
}

// CleanupStaleMedia is safe to call repeatedly and across replicas (SKIP LOCKED).
// It also removes rejected objects; a storage failure preserves the DB row for retry.
func (s *Server) CleanupStaleMedia(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	storage, err := storageEnv()
	if err != nil {
		return 0, nil
	} // no configured bucket
	cleaned := 0
	for range limit {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return cleaned, err
		}
		var id,key,state string
        var staging *string
		err = tx.QueryRow(ctx, `SELECT id,object_key,staging_key,state FROM media_attachments WHERE
  (state='pending' AND created_at<now()-interval '20 minutes')
  OR (state='rejected' AND deleted_at IS NULL)
  OR (staging_key IS NOT NULL AND created_at<now()-interval '20 minutes')
  ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id,&key,&staging,&state)
		if errors.Is(err, pgx.ErrNoRows) {
			tx.Rollback(ctx)
			break
		}
		if err != nil {
			tx.Rollback(ctx)
			return cleaned, err
		}
		removeKey:=key
        if staging!=nil{removeKey=*staging}
		response, err := s.storageDo(ctx, storage, "DELETE", removeKey, "")
		if err != nil {
			tx.Rollback(ctx)
			return cleaned, err
		}
		io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
		response.Body.Close()
		if response.StatusCode != 200 && response.StatusCode != 204 && response.StatusCode != 404 {
			tx.Rollback(ctx)
			return cleaned, fmt.Errorf("storage cleanup DELETE returned %d", response.StatusCode)
		}
        if state=="uploaded"||state=="deleted" {
         _,err=tx.Exec(ctx,`UPDATE media_attachments SET staging_key=NULL,staging_cleaned_at=now() WHERE id=$1`,id)
        }else{
		 _,err=tx.Exec(ctx,`UPDATE media_attachments SET state='rejected',rejected_at=COALESCE(rejected_at,now()),
         verification_error=COALESCE(verification_error,'upload expired'),deleted_at=now(),
         staging_key=NULL,staging_cleaned_at=now() WHERE id=$1`,id)
        }
		if err != nil {
			tx.Rollback(ctx)
			return cleaned, err
		}
		if err = tx.Commit(ctx); err != nil {
			return cleaned, err
		}
		cleaned++
	}
	return cleaned, nil
}
