package api
import "testing"
func TestPINPolicy(t *testing.T){for _,tc:=range []struct{pin string;valid bool}{{"123456",false},{"654321",false},{"111111",false},{"918245",true},{"12345",false},{"abc123",false}}{if pinOK(tc.pin)!=tc.valid{t.Errorf("pin policy for %q",tc.pin)}}}}
