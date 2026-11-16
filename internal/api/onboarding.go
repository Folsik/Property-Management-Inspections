package api

import (
 "crypto/rand"
 "encoding/hex"
 "errors"
 "fmt"
 "math/big"
 "net/http"
 "os"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgconn"
 "golang.org/x/crypto/bcrypt"
)

func (s *Server) onboardingStatus(w http.ResponseWriter,r *http.Request){
 var phone,name string
 var email *string
 var verified,complete *time.Time
 var pinSet bool
 err:=s.db.QueryRow(r.Context(),`SELECT phone,full_name,email,email_verified_at,onboarding_completed_at,(pin_hash IS NOT NULL)
 FROM users WHERE id=$1`,who(r).UserID).Scan(&phone,&name,&email,&verified,&complete,&pinSet)
 if err!=nil{fail(w,503,"onboarding unavailable");return}
 var ownOrg,hasProperty,hasMembership bool
 err=s.db.QueryRow(r.Context(),`SELECT
 EXISTS(SELECT 1 FROM organizations WHERE created_by=$1),
 EXISTS(SELECT 1 FROM properties p JOIN organizations o ON o.id=p.organization_id WHERE o.created_by=$1),
 EXISTS(SELECT 1 FROM organization_members WHERE user_id=$1)`,who(r).UserID).Scan(&ownOrg,&hasProperty,&hasMembership)
 if err!=nil{fail(w,503,"onboarding unavailable");return}
 write(w,200,map[string]any{"phone":phone,"full_name":name,"email":email,"pin_set":pinSet,
 "email_verified":verified!=nil,"completed":complete!=nil,"has_organization":hasMembership,
 "owns_organization":ownOrg,"has_property":hasProperty})
}

func (s *Server) requestEmailOTP(w http.ResponseWriter,r *http.Request){
 var in struct{Email string `json:"email"`}
 if !decode(w,r,&in){return}
 email:=normalizedEmail(in.Email)
 if email==""{fail(w,400,"invalid email");return}
 var count int
 err:=s.db.QueryRow(r.Context(),`SELECT count(*) FROM email_challenges WHERE user_id=$1 AND created_at > now()-interval '15 minutes'`,who(r).UserID).Scan(&count)
 if err!=nil{fail(w,503,"unavailable");return}
 if count>=3{fail(w,429,"rate limited");return}
 var other bool
 err=s.db.QueryRow(r.Context(),`SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=$1 AND email_verified_at IS NOT NULL AND id<>$2)`,email,who(r).UserID).Scan(&other)
 if err!=nil{fail(w,503,"unavailable");return}
 if other{fail(w,409,"email already in use");return}
 value,err:=rand.Int(rand.Reader,big.NewInt(1000000));if err!=nil{fail(w,503,"unavailable");return}
 code:=fmt.Sprintf("%06d",value.Int64())
 hashed,err:=bcrypt.GenerateFromPassword([]byte(code),bcrypt.DefaultCost);if err!=nil{fail(w,503,"unavailable");return}
 provider:=os.Getenv("RESEND_API_KEY")!=""&&os.Getenv("EMAIL_FROM")!=""
 if !provider&&s.env!="development"{fail(w,503,"email provider not configured");return}
 tx,err:=s.db.Begin(r.Context());if err!=nil{fail(w,503,"unavailable");return};defer tx.Rollback(r.Context())
 _,err=tx.Exec(r.Context(),`INSERT INTO email_challenges(user_id,email,code_hash,expires_at) VALUES($1,$2,$3,now()+interval '10 minutes')`,who(r).UserID,email,string(hashed))
 if err!=nil{fail(w,503,"unavailable");return}
 if provider {
  err=sendTransactionalEmail(r.Context(),email,"Verify your Inspectly email","<p>Your Inspectly verification code is <strong>"+code+"</strong>.</p><p>It expires in 10 minutes.</p>")
  if err!=nil{s.logger.Error("email delivery failed","error",err);fail(w,503,"email delivery unavailable");return}
 }
 if err=tx.Commit(r.Context());err!=nil{fail(w,503,"unavailable");return}
 if !provider {write(w,202,map[string]string{"status":"local_development","dev_code":code});return}
 write(w,202,map[string]string{"status":"sent"})
}

func (s *Server) verifyEmailOTP(w http.ResponseWriter,r *http.Request){
 var in struct{Email string `json:"email"`;Code string `json:"code"`}
 if !decode(w,r,&in){return}
 email:=normalizedEmail(in.Email)
 if email==""||len(in.Code)!=6{fail(w,400,"invalid email or code");return}
 for _,c:=range in.Code{if c<'0'||c>'9'{fail(w,400,"invalid code");return}}
 tx,err:=s.db.Begin(r.Context());if err!=nil{fail(w,503,"unavailable");return};defer tx.Rollback(r.Context())
 var id,hash string
 var attempts int
 err=tx.QueryRow(r.Context(),`SELECT id,code_hash,attempts FROM email_challenges WHERE user_id=$1 AND email=$2
 AND consumed_at IS NULL AND expires_at>now() ORDER BY created_at DESC LIMIT 1 FOR UPDATE`,who(r).UserID,email).Scan(&id,&hash,&attempts)
 if errors.Is(err,pgx.ErrNoRows){fail(w,401,"invalid or expired code");return}
 if err!=nil{fail(w,503,"unavailable");return}
 if attempts>=5{fail(w,429,"code locked");return}
 correct:=bcrypt.CompareHashAndPassword([]byte(hash),[]byte(in.Code))==nil
 _,err=tx.Exec(r.Context(),`UPDATE email_challenges SET attempts=attempts+1,consumed_at=CASE WHEN $2 THEN now() ELSE consumed_at END WHERE id=$1`,id,correct)
 if err!=nil{fail(w,503,"unavailable");return}
 if !correct {_=tx.Commit(r.Context());fail(w,401,"invalid or expired code");return}
 _,err=tx.Exec(r.Context(),`UPDATE users SET email=$2,email_verified_at=now(),updated_at=now() WHERE id=$1`,who(r).UserID,email)
 var pgerr *pgconn.PgError
 if errors.As(err,&pgerr)&&pgerr.Code=="23505"{fail(w,409,"email already in use");return}
 if err!=nil{fail(w,503,"unavailable");return}
 if err=tx.Commit(r.Context());err!=nil{fail(w,503,"unavailable");return}
 write(w,200,map[string]string{"email":email,"status":"verified"})
}

func (s *Server) completeOnboarding(w http.ResponseWriter,r *http.Request){
 var allowed bool
 err:=s.db.QueryRow(r.Context(),`SELECT (
 u.pin_hash IS NOT NULL AND u.email_verified_at IS NOT NULL AND length(trim(u.full_name))>0
 AND EXISTS(SELECT 1 FROM organization_members m WHERE m.user_id=u.id AND (
 m.role<>'owner' OR EXISTS(SELECT 1 FROM properties p WHERE p.organization_id=m.organization_id)
 )))
 ) FROM users u WHERE u.id=$1`,who(r).UserID).Scan(&allowed)
 if err!=nil{fail(w,503,"unavailable");return}
 if !allowed{fail(w,409,"complete your PIN, verified email, name, organization and first property");return}
 _,err=s.db.Exec(r.Context(),`UPDATE users SET onboarding_completed_at=COALESCE(onboarding_completed_at,now()) WHERE id=$1`,who(r).UserID)
 if err!=nil{fail(w,503,"unavailable");return}
 write(w,200,map[string]string{"status":"completed"})
}

// Invites are 32-byte random bearer tokens. Only their SHA-256 digest is stored.
func generateInvitationToken()(string,string,error){
 data:=make([]byte,32)
 if _,err:=rand.Read(data);err!=nil{return "","",err}
 token:=hex.EncodeToString(data)
 return token,hashInvitationToken(token),nil
}
