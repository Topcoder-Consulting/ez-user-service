package main

import (
	"encoding/json"
	"fmt"
	"github.com/codegangsta/martini"
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
	//fmt.Println("Running middleware...", qs["apiKey"][0], os.Getenv("API_KEY"))
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
	Topcoder_Handle__c        string `force:",omitempty"`
	Topcoder_Last_Login__c    string `force:",omitempty"`
	Topcoder_Member_Status__c string `force:",omitempty"`
	Topcoder_User_Id__c       string `force:",omitempty"`
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
		fmt.Println("Bingo! Success!!")
	}

	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/m/:handle", middleware, func(r render.Render, params martini.Params, res http.ResponseWriter) {

		list := &ContactQueryResponse{}
		err = forceApi.Query("select id, name, firstname, lastname, email, topcoder_handle__c, topcoder_last_login__c, topcoder_member_status__c, topcoder_user_id__c from contact where topcoder_handle__c = '"+params["handle"]+"' limit 1", list)
		if err != nil {
			panic(err)
		} else {

			if len(list.Records) == 1 {
				id, _ := strconv.ParseInt(list.Records[0].Topcoder_User_Id__c, 0, 64)
				r.JSON(200, map[string]interface{}{
					"id":        id,
					"firstname": list.Records[0].Firstname,
					"lastname":  list.Records[0].Lastname,
					"name":      list.Records[0].Name,
					"handle":    list.Records[0].Topcoder_Handle__c,
					"email":     list.Records[0].Email,
					"status":    list.Records[0].Topcoder_Member_Status__c,
					"lastLogin": list.Records[0].Topcoder_Last_Login__c,
				})
			} else {
				// call the topcoder api
				resp, err := http.Get(os.Getenv("TC_ENDPOINT") + "/" + params["handle"] + "?apiKey=" + os.Getenv("TC_API_KEY"))
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
					r.JSON(200, map[string]interface{}{
						"handle": params["handle"],
						"email":  dat["email"],
					})
				} else {
					r.JSON(resp.StatusCode, map[string]interface{}{})
				}

			}
		}

	})

	m.Run()
}
