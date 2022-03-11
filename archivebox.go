//
// archivebox-quick-add
// 2022 emschu[aet]mailbox.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License along with this program.
// If not, see <https://www.gnu.org/licenses/>.
package main

import (
	"bytes"
	"fmt"
	"fyne.io/fyne/v2"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func isURL(inputStr string) bool {
	parse, err := url.Parse(inputStr)
	if err != nil {
		return false
	}
	if parse.Scheme == "http" || parse.Scheme == "https" {
		return true
	}
	if isDebug {
		log.Printf("Invalid url: %s\n", inputStr)
	}
	return false
}

func doArchiveBoxLogin() {
	pw := fyneApplication.Preferences().StringWithFallback(preferencePassword, "")
	username := fyneApplication.Preferences().StringWithFallback(preferenceUsername, "")

	paramStr := fmt.Sprintf("csrfmiddlewaretoken=%s&username=%s&password=%s&next=%%2F",
		appSessionState.CsrfMiddlewareToken, url.QueryEscape(username), url.QueryEscape(pw))
	buffer := bytes.NewBuffer([]byte(paramStr))

	request, err := buildPostRequest("/admin/login/?next=", buffer)

	transport := http.Transport{}
	do, err := transport.RoundTrip(request)
	if err != nil {
		panic(err)
	}
	defer do.Body.Close()

	if do.StatusCode != 302 {
		// something went wrong
		log.Printf("Problem with login! Got status %d\n", do.StatusCode)
		appSessionState.SessionCookie = nil
		return
	}

	for _, i := range do.Cookies() {
		if i.Name == "sessionid" {
			appSessionState.SessionCookie = i
			log.Printf("Session id is set successfully")
			appSessionState.IsConnected = true
			break
		}
	}
}

func doArchiveBoxLogout() {
	if appSessionState.SessionCookie == nil || appSessionState.CsrfToken == nil {
		// there was no login
		return
	}

	request, err := buildGetRequest("/admin/logout")
	if err != nil {
		log.Printf("Error creating logout request\n")
		return
	}

	get, err := httpClient.Do(request)
	if err != nil {
		log.Printf("Logout request failed!:%v\n", err)
		return
	}
	if get.StatusCode == 200 {
		log.Printf("Logout\n")
	} else {
		log.Printf("Logout not successful\n")
	}
	defer get.Body.Close()
}

func buildGetRequest(apiPath string) (*http.Request, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s%s", appConfig.InstanceURL, apiPath), nil)
	if err != nil {
		panic(err)
	}

	if appSessionState.CsrfToken != nil {
		request.AddCookie(appSessionState.CsrfToken)
	}
	if appSessionState.SessionCookie != nil {
		request.AddCookie(appSessionState.SessionCookie)
	}
	return request, err
}

func buildPostRequest(apiPath string, requestData *bytes.Buffer) (*http.Request, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s%s", appConfig.InstanceURL, apiPath), requestData)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate, private")
	request.Header.Set("Host", appConfig.InstanceURL)
	request.Header.Set("Origin", appConfig.InstanceURL)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Content-Length", "0")
	if appSessionState.CsrfToken != nil {
		request.AddCookie(appSessionState.CsrfToken)
	}
	if appSessionState.SessionCookie != nil {
		request.AddCookie(appSessionState.SessionCookie)
	}

	return request, nil
}

func setupArchiveBoxConnection() {
	if len(strings.TrimSpace(appConfig.InstanceURL)) == 0 {
		appSessionState.ConnectionErr = fmt.Errorf("invalid empty url to archivebox")
		appConfig.disconnect()
		return
	}
	if !isURL(appConfig.InstanceURL) {
		appSessionState.ConnectionErr = fmt.Errorf("url does not start with 'http[s]://'")
		appConfig.disconnect()
		return
	}

	adminResp, err := httpClient.Get(fmt.Sprintf("%s/admin/login", appConfig.InstanceURL))
	if err != nil {
		appSessionState.ConnectionErr = err
		appConfig.disconnect()
		return
	}
	defer adminResp.Body.Close()

	for _, c := range adminResp.Cookies() {
		if c.Name == "csrftoken" {
			if len(c.Value) > 0 {
				appSessionState.CsrfToken = c
			}
			if isDebug {
				log.Printf("csrf token: %v\n", c.Value)
			}
		}
	}
	pattern := regexp.MustCompile("name=\"csrfmiddlewaretoken\" value=\"(.*?)\"")
	all, err := ioutil.ReadAll(adminResp.Body)
	if err != nil {
		csrfErrMsg := "Cannot find csrfmiddlewaretoken!"
		log.Println(csrfErrMsg)
		appSessionState.ConnectionErr = fmt.Errorf(csrfErrMsg)
		appConfig.disconnect()
		return
	}
	if len(strings.TrimSpace(string(all))) > 0 {
		match := pattern.FindStringSubmatch(string(all))
		if len(match) > 1 && len(match[1]) > 0 {
			appSessionState.CsrfMiddlewareToken = strings.TrimSpace(match[1])
		} else {
			log.Printf("Problem finding csrfmiddlewaretoken!\n")
		}
	}
	if isDebug {
		log.Printf("csrfmiddlewaretoken: %s\n", appSessionState.CsrfMiddlewareToken)
	}

	if appSessionState.SessionCookie == nil {
		if len(appSessionState.CsrfMiddlewareToken) > 0 && appSessionState.CsrfToken != nil {
			doArchiveBoxLogin()
		} else {
			log.Printf("Cannot start login")
		}
	}
}

func isURLAlreadyArchived(urlToCheck string) bool {
	urlToCheck = strings.TrimSpace(urlToCheck)
	if !appSessionState.IsConnected {
		return false
	}
	// validate url at first
	if len(urlToCheck) < 5 || !isURL(urlToCheck) {
		return false
	}

	snapshotSearchPath := fmt.Sprintf("/admin/core/snapshot/?q=%s", url.QueryEscape(urlToCheck))

	request, err := buildGetRequest(snapshotSearchPath)
	if err != nil {
		log.Printf("Problem creating request\n")
		return false
	}

	// use a higher another client timeout value to wait for search results
	do, err := archiveSearchHTTPClient.Do(request)
	if err != nil {
		log.Printf("Problem fetching snapshot search page: %v\n", err)
		return false
	}
	defer do.Body.Close()

	content, err := ioutil.ReadAll(do.Body)
	if err != nil {
		log.Printf("Problem reading response body:%s\n", err.Error())
		return false
	}

	// check if result size is zero
	pattern := regexp.MustCompile("<span class=\"small quiet\">0 results \\(<a href=\"?\">[0-9]+ total</a>\\)</span>")
	if pattern.Match(content) {
		return false
	}
	return true
}

func sendURLToArchiveBox(urlToSave string) (bool, error) {
	setupArchiveBoxConnection()
	urlToSave = strings.TrimSpace(urlToSave)
	if !appSessionState.IsConnected {
		return false, fmt.Errorf(t("NoConnectionToInstance"))
	}

	// validate url at first
	if len(urlToSave) < 5 {
		return false, fmt.Errorf(t("URLTooShort"))
	}
	if !isURL(urlToSave) {
		return false, fmt.Errorf(t("InvalidURL"))
	}

	buffer := bytes.NewBuffer([]byte(fmt.Sprintf("csrfmiddlewaretoken=%s&url=%s&parser=auto&tag=&depth=0",
		appSessionState.CsrfMiddlewareToken, url.QueryEscape(urlToSave))))
	request, err := buildPostRequest("/add/", buffer)
	if err != nil {
		panic(err)
	}
	transport := http.Transport{}

	// the endpoint sometimes needs a lot of time (~60 secs) until there is a response
	// therefore we 'cancel' the post request after 2 secs and ArchiveBox should have the request
	transport.ResponseHeaderTimeout = 2 * time.Second // this should be enough time to wait for the request being processed by ArchiveBox
	do, err := transport.RoundTrip(request)
	if err != nil {
		if strings.HasSuffix(err.Error(), "timeout awaiting response headers") {
			// this is expected
			return true, nil
		}
		msg := tWithArgs("ProblemCallingArchiveBox", struct {
			URL string
		}{URL: appConfig.InstanceURL})
		log.Printf("%s\n", msg)
		return false, fmt.Errorf(msg)
	}
	defer do.Body.Close()
	log.Printf("entry add response status: %v\n", do.StatusCode)
	return true, nil
}

// should be non-blocking to be safe for ui, handles validation of input
func archiveURL(urlInput string) {
	setupArchiveBoxConnection()
	urlInput = strings.TrimSpace(urlInput)
	if appSessionState.IsSubmissionBlocked.isSet() {
		if isDebug {
			log.Printf("Blocked submission of URL '%s'\n", urlInput)
		}
		return
	}
	infoLabel.Text = ""
	log.Printf("Started URL archiving for url '%s'\n", urlInput)
	go func() {
		inputEntryWidget.Disable()
		addToArchiveBtn.Disable()
		hasWorked, err := sendURLToArchiveBox(urlInput)
		if hasWorked {
			// all went fine!
			closeAppPref := fyneApplication.Preferences().BoolWithFallback(preferenceCloseAfterAdd, false)
			checkAfterAddPref := fyneApplication.Preferences().BoolWithFallback(preferenceCheckAdd, false)
			if checkAfterAddPref {
				if isURLAlreadyArchived(urlInput) {
					var urlString string
					urlString, err = url.QueryUnescape(urlInput)
					if err != nil {
						urlString = urlInput
					}
					fyneApplication.SendNotification(&fyne.Notification{
						Title: tWithArgs("NotificationTitle", struct {
							APP_NAME string
						}{APP_NAME: appConfig.AppName}),
						Content: tWithArgs("URLHasBeenAdded", struct {
							URL string
						}{URL: urlString}),
					})
					if closeAppPref {
						fyneApplication.Quit()
					}
					inputEntryWidget.SetText("")
				} else {
					fyneApplication.SendNotification(&fyne.Notification{
						Title: tWithArgs("NotificationTitle", struct {
							APP_NAME string
						}{APP_NAME: appConfig.AppName}),
						Content: t("URLAddingCouldNotBeChecked"),
					})
				}
			} else {
				fyneApplication.SendNotification(&fyne.Notification{
					Title: tWithArgs("NotificationTitle", struct {
						APP_NAME string
					}{APP_NAME: appConfig.AppName}),
					Content: tWithArgs("URLHasBeenSent", struct {
						URL string
					}{URL: urlInput}),
				})
				if closeAppPref {
					fyneApplication.Quit()
				}
				inputEntryWidget.SetText("")
			}
		}
		if err != nil {
			infoLabel.Text = tWithArgs("ProblemAddingURL", struct {
				ERROR string
			}{ERROR: err.Error()})
		}
		infoLabel.Refresh()
		inputEntryWidget.Enable()
		inputEntryWidget.Refresh()
		addToArchiveBtn.Enable()
		addToArchiveBtn.Refresh()
		window.Resize(windowSize)
	}()
	infoLabel.Refresh()
	log.Println("Finished URL archiving process")
}
