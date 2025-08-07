package clms

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/quantifyearth/reclaimer/internal/utils"
)

// This is the structure you download from the CLMS website
// as your API Key
type CLMSAuthenticationDetails struct {
	ClientID string  `json:"client_id"`
	IPRange  *string `json:"ip_range"`
	// this can't be parsed as a time.Time, as CLMS doesn't
	// generate a timezone offset on its string
	Issued     string `json:"issued"`
	KeyID      string `json:"key_id"`
	PrivateKey string `json:"private_key"`
	Title      string `json:"title"`
	TokenURI   string `json:"token_uri"`
	UserID     string `json:"user_id"`
}

type CLMSAuthResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresInSeconds int    `json:"expires_in"`
	TokenType        string `json:"token_type"`
}

type CLMSErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func LoadAPIKey(path string) (CLMSAuthenticationDetails, error) {
	contents, err := os.ReadFile(path)
	if nil != err {
		return CLMSAuthenticationDetails{}, err
	}

	var token CLMSAuthenticationDetails
	err = json.Unmarshal(contents, &token)
	if nil != err {
		return CLMSAuthenticationDetails{}, fmt.Errorf("failed to parse: %w", err)
	}

	return token, nil
}

func (c CLMSAuthenticationDetails) key() (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(c.PrivateKey))
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func (c CLMSAuthenticationDetails) GetSessionToken() (string, error) {

	claims := jwt.StandardClaims{
		Issuer:    c.ClientID,
		Subject:   c.UserID,
		Audience:  c.TokenURI,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Unix() + (60 * 60),
	}

	claim := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	key, err := c.key()
	if nil != err {
		return "", err
	}
	assertion, err := claim.SignedString(key)
	if nil != err {
		return "", err
	}

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
	}
	body := "grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=" + assertion
	resp, err := utils.HTTPPost(c.TokenURI, headers, body)
	if nil != err {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r, err := io.ReadAll(resp.Body)
		body := resp.Status
		if nil == err {
			body = string(r)
		}
		return "", fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, body)
	}

	var res CLMSAuthResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if nil != err {
		return "", fmt.Errorf("failed to decode response for: %w", err)
	}

	return res.AccessToken, nil
}
