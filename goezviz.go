package ezviz

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	//VERSION is SDK version
	VERSION  = "0.1"
	typeJSON = "application/json"
)

//DingTalkClient is the Client to access DingTalk Open API
type EzvizClient struct {
	AppKey      string
	AppSecret   string
	AccessToken string
	HTTPClient  *http.Client
	Cache       Cache
}

//Unmarshallable is
type Unmarshallable interface {
	checkError() error
	getWriter() io.Writer
}

//OAPIResponse is
type OAPIResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

func (data *OAPIResponse) checkError() (err error) {
	if data.Code != "200" {
		err = fmt.Errorf("%s: %s", data.Code, data.Msg)
	}
	return err
}

func (data *OAPIResponse) getWriter() io.Writer {
	return nil
}

type AccessToken struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  int64  `json:"expireTime"`
}

//AccessTokenResponse is
type AccessTokenResponse struct {
	OAPIResponse
	Data AccessToken `json:"data"`
}

//ExpiresIn is how soon the access token is expired
func (e *AccessTokenResponse) GetExpireTime() int64 {
	return e.Data.ExpireTime
}

//NewEzvizClientClient creates a EzvizClientClient instance
func NewEzvizClient(appKey string, appSecret string) *EzvizClient {
	c := new(EzvizClient)
	c.AppKey = appKey
	c.AppSecret = appSecret
	c.HTTPClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	c.Cache = NewFileCache(fmt.Sprintf("ezviz_%s.auth_file", appKey))
	return c
}

//RefreshAccessToken is to get a valid access token
func (c *EzvizClient) RefreshAccessToken() error {
	var res AccessTokenResponse
	err := c.Cache.Get(&res)
	if err == nil {
		c.AccessToken = res.Data.AccessToken
		return nil
	}

	params := url.Values{}
	params["appKey"] = []string{c.AppKey}
	params["appSecret"] = []string{c.AppSecret}
	err = c.httpRPC("/lapp/token/get", params, nil, &res)
	if err == nil {
		c.AccessToken = res.Data.AccessToken
		err = c.Cache.Set(&res)
	}
	return err
}

func (c *EzvizClient) httpRPC(path string, params url.Values, requestData interface{}, responseData Unmarshallable) error {
	if c.AccessToken != "" {
		if params == nil {
			params = url.Values{}
		}
		if params.Get("accessToken") == "" {
			params.Set("accessToken", c.AccessToken)
		}
	}
	return c.httpRequest(path, params, requestData, responseData)
}

func (c *EzvizClient) httpRequest(path string, params url.Values, requestData interface{}, responseData Unmarshallable) error {
	client := c.HTTPClient
	var request *http.Request
	ROOT := os.Getenv("oapi_server")
	if ROOT == "" {
		ROOT = "open.ys7.com/api"
	}
	DEBUG := os.Getenv("debug") != ""
	url2 := "https://" + ROOT + "/" + path + "?" + params.Encode()
	// log.Println(url2)
	if DEBUG {
		log.Printf("url: %s", url2)
	}
	request, _ = http.NewRequest("POST", url2, nil)
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("Server error: " + resp.Status)
	}

	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if DEBUG {
		log.Printf("url: %s response content type: %s", url2, contentType)
	}
	pos := len(typeJSON)
	if len(contentType) >= pos && contentType[0:pos] == typeJSON {
		content, err := ioutil.ReadAll(resp.Body)
		if DEBUG {
			log.Println(string(content))
		}
		if err == nil {
			json.Unmarshal(content, responseData)
			return responseData.checkError()
		}
	} else {
		io.Copy(responseData.getWriter(), resp.Body)
		return responseData.checkError()
	}
	return err
}
