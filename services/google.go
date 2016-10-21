package services

import (
	"fmt"
	"net/http"
	"net/mail"

	"github.com/kevinburke/rest"
)

var UserDataBase = "https://www.googleapis.com"
var UserDataPath = "/oauth2/v3/userinfo"

type GoogleUser struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Profile       string `json:"profile"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Gender        string `json:"gender"`
	Locale        string `json:"locale"`
	HD            string `json:"hd"`
}

func GetGoogleUserData(client *http.Client) (*GoogleUser, error) {
	if client == nil {
		client = http.DefaultClient
	}
	rc := rest.NewClient("", "", UserDataBase)
	rc.Client = client
	req, err := rc.NewRequest("GET", UserDataPath, nil)
	if err != nil {
		return nil, err
	}
	u := new(GoogleUser)
	err = rc.Do(req, u)
	if err != nil {
		return nil, err
	}
	if u.Email == "" {
		return nil, fmt.Errorf("No email address for user: %s", u.Name)
	}
	if _, err := mail.ParseAddress(u.Email); err != nil {
		return nil, err
	}
	if u.EmailVerified == false {
		return nil, fmt.Errorf("User %s does not have a verified email address", u.Name)
	}
	return u, err
}
