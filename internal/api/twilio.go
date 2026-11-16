package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type twilioConfig struct {
	accountSID string
	authToken string
	from string
}
var twilioSID=regexp.MustCompile(`^AC[0-9a-fA-F]{32}$`)

func twilioConfigFromEnv() twilioConfig {
	return twilioConfig{
		accountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		authToken: os.Getenv("TWILIO_AUTH_TOKEN"),
		from: os.Getenv("TWILIO_FROM_NUMBER"),
	}
}
func (c twilioConfig) configured() bool {
	return twilioSID.MatchString(c.accountSID) && c.authToken!="" && phonePattern.MatchString(c.from)
}

func sendTwilioSMS(ctx context.Context, cfg twilioConfig, recipient, message string) error {
	if !cfg.configured() || !phonePattern.MatchString(recipient) {
		return fmt.Errorf("invalid Twilio configuration or recipient")
	}
	endpoint := "https://api.twilio.com/2010-04-01/Accounts/"+cfg.accountSID+"/Messages.json"
	form := url.Values{"To":{recipient},"From":{cfg.from},"Body":{message}}
	req, err := http.NewRequestWithContext(ctx,http.MethodPost,endpoint,strings.NewReader(form.Encode()))
	if err != nil { return err }
	req.SetBasicAuth(cfg.accountSID,cfg.authToken)
	req.Header.Set("Content-Type","application/x-www-form-urlencoded")
	client:=&http.Client{Timeout:10*time.Second}
	response,err:=client.Do(req)
	if err!=nil { return err }
	defer response.Body.Close()
	_,_ = io.Copy(io.Discard,io.LimitReader(response.Body,2048))
	if response.StatusCode<200||response.StatusCode>=300 {
		return fmt.Errorf("Twilio message request returned HTTP %d",response.StatusCode)
	}
	return nil
}
