package jobs

import (
 "testing"
 "time"
)
func TestBackoff(t *testing.T) {
 cases:=[]struct{attempt int;want time.Duration}{
  {0,5*time.Second},{1,5*time.Second},{2,10*time.Second},{3,20*time.Second},{4,40*time.Second},{11,2560*time.Second},
 }
 for _,c:=range cases { if got:=Backoff(c.attempt);got!=c.want {t.Errorf("attempt %d: %v not %v",c.attempt,got,c.want)} }
}
