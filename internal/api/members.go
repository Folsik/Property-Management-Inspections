package api
import ("errors";"net/http";"github.com/jackc/pgx/v5")
func(s *Server) addMember(w http.ResponseWriter,r *http.Request){
 caller,ok:=s.role(r,"owner","manager");if !ok{fail(w,403,"forbidden");return}
 var in struct{Phone string `json:"phone"`; Role string `json:"role"`}
 if !decode(w,r,&in){return}
 if !phonePattern.MatchString(in.Phone)||in.Role!="inspector"&&in.Role!="manager"{fail(w,400,"invalid member");return}
 if in.Role=="manager"&&caller!="owner"{fail(w,403,"only owner can add a manager");return}
 var id string
 err:=s.db.QueryRow(r.Context(),"SELECT id FROM users WHERE phone=$1",in.Phone).Scan(&id)
 if errors.Is(err,pgx.ErrNoRows){fail(w,404,"user must register first");return}
 if err!=nil{fail(w,503,"unavailable");return}
 _,err=s.db.Exec(r.Context(),"INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT(organization_id,user_id) DO NOTHING",r.PathValue("org"),id,in.Role)
 if err!=nil{fail(w,503,"unavailable");return}
 write(w,201,map[string]string{"user_id":id,"role":in.Role})
}
