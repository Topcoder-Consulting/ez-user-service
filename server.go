package main

import (
	"encoding/json"
	"fmt"
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/nimajalali/go-force/force"
	"github.com/nimajalali/go-force/sobjects"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

// SECURITY -- get the url params and check for an apiKey
func middleware(req *http.Request, res http.ResponseWriter, c martini.Context, params martini.Params) {
	qs := req.URL.Query()
	// no url param 'apiKey'!
	if len(qs["apiKey"]) == 0 {
		unauthorized(res)
	} else {
		// found but does not match
		if qs["apiKey"][0] != os.Getenv("API_KEY") {
			unauthorized(res)
		}
	}
}

// returns 403 forbidden
func unauthorized(res http.ResponseWriter) {
	http.Error(res, "Forbidden", http.StatusForbidden)
}

// struct for contact object returned from query
type ContactSObject struct {
	sobjects.BaseSObject
	Firstname                 string `force:",omitempty"`
	Lastname                  string `force:",omitempty"`
	Email                     string `force:",omitempty"`
	MailingCountry            string `force:",omitempty"`
	Topcoder_Handle__c        string `force:",omitempty"`
	Topcoder_Last_Login__c    string `force:",omitempty"`
	Topcoder_Member_Status__c string `force:",omitempty"`
	Topcoder_User_Id__c       string `force:",omitempty"`
}

// struct for contact object returned from query
type SlackWhois struct {
	Token        string `form:"token"`
	Team_id      string `form:"team_id"`
	Team_domain  string `form:"team_domain"`
	Channel_id   string `form:"channel_id"`
	Channel_name string `form:"channel_name"`
	User_id      string `form:"user_id"`
	User_name    string `form:"user_name"`
	Command      string `form:"command"`
	Text         string `form:"text"`
}

type ContactQueryResponse struct {
	sobjects.BaseQuery
	Records []ContactSObject `json:"Records" force:"records"`
}

func main() {
	fmt.Println("Attempting to authenticate to Salesforce....")
	forceApi, err := force.Create(
		"v32.0",
		os.Getenv("SFDC_CLIENT_ID"),
		os.Getenv("SFDC_CLIENT_SECRET"),
		os.Getenv("SFDC_USERNAME"),
		os.Getenv("SFDC_PASSWORD"),
		os.Getenv("SFDC_SECURITY_TOKEN"),
		os.Getenv("SFDC_ENVIRONMENT"),
	)
	if err != nil {
		panic(err)
	} else {
		fmt.Println("Bingo! Connected to Salesforce successfully!!")
	}

	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/m/:handle", middleware, func(r render.Render, params martini.Params, res http.ResponseWriter) {
		member, status := fetchMemberByHandle(forceApi, params["handle"])
		r.JSON(status, member)
	})

	m.Post("/slack/whois", middleware, binding.Bind(SlackWhois{}), func(slack SlackWhois) string {
		if slack.Token != os.Getenv("SLACK_TOKEN") {
			return "Slack not authorized. Bad Slack token."
		} else {
			data, status := fetchMemberByHandle(forceApi, slack.Text)
			if status == 200 {
				member := data.(map[string]interface{})
				return ("'" + member["handle"].(string) + "' is " + member["firstname"].(string) + " " + member["lastname"].(string) + " (" + member["email"].(string) + ") from " + member["country"].(string) + ". Current status is " + member["status"].(string) + " and their last login was " + member["lastLogin"].(string) + ".")
			} else if status == 404 {
				return "No member found with handle '" + slack.Text + "'."
			} else {
				return "Bummer. Service returned an error."
			}
		}
	})

	m.Run()
}

func fetchMemberByHandle(forceApi *force.ForceApi, handle string) (interface{}, int) {

	list := &ContactQueryResponse{}
	err := forceApi.Query("select id, name, firstname, lastname, email, mailingcountry, topcoder_handle__c, topcoder_last_login__c, topcoder_member_status__c, topcoder_user_id__c from contact where topcoder_handle__c = '"+handle+"' limit 1", list)
	if err != nil {
		panic(err)
	} else {

		// found member in salesforce
		if len(list.Records) == 1 {

			id, _ := strconv.ParseInt(list.Records[0].Topcoder_User_Id__c, 0, 64)
			member := map[string]interface{}{
				"id":        id,
				"firstname": list.Records[0].Firstname,
				"lastname":  list.Records[0].Lastname,
				"name":      list.Records[0].Name,
				"handle":    list.Records[0].Topcoder_Handle__c,
				"email":     list.Records[0].Email,
				"country":   list.Records[0].MailingCountry,
				"status":    list.Records[0].Topcoder_Member_Status__c,
				"lastLogin": list.Records[0].Topcoder_Last_Login__c,
			}
			return member, 200

			// not found in sfdc, try topcoder
		} else {

			// call the topcoder api
			resp, err := http.Get(os.Getenv("TC_ENDPOINT") + "/" + handle + "?apiKey=" + os.Getenv("TC_API_KEY"))
			if err != nil {
				panic(err)
			}
			// if 200 then we are good and send the email and handle back
			if resp.StatusCode == 200 {
				defer resp.Body.Close()
				body, _ := ioutil.ReadAll(resp.Body)
				byt := []byte(string(body))
				var dat map[string]interface{}
				if err := json.Unmarshal(byt, &dat); err != nil {
					panic(err)
				}
				return map[string]interface{}{
					"handle": handle,
					"email":  dat["email"],
				}, 200
			} else {
				return map[string]interface{}{}, resp.StatusCode
			}

		}
	}

}
