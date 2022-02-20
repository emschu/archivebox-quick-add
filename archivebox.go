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
	"strconv"
	"strings"
)

func isUrl(inputStr string) bool {
	parse, err := url.Parse(inputStr)
	if err != nil {
		return false
	}
	if parse.Scheme == "http" || parse.Scheme == "https" {
		return true
	}
	log.Printf("Invalid url: %s\n", inputStr)
	return false
}

func doArchiveBoxLogin() {
	pw := application.Preferences().StringWithFallback(PREFERENCE_PASSWORD, "")
	username := application.Preferences().StringWithFallback(PREFERENCE_USERNAME, "")

	buffer := bytes.NewBuffer([]byte(fmt.Sprintf("csrfmiddlewaretoken=%s&username=%s&password=%s&next=%2F",
		csrfMiddlewareToken, url.QueryEscape(username), url.QueryEscape(pw))))

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
		sessionCookie = nil
		return
	}

	for _, i := range do.Cookies() {
		if i.Name == "sessionid" {
			sessionCookie = i
			log.Printf("Session id is set successfully")
			IsConnected = true
		}
	}
}

func doArchiveBoxLogout() {
	if sessionCookie == nil || csrfToken == nil {
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
	request, err := http.NewRequest("GET", fmt.Sprintf("%s%s", archiveBoxURL, apiPath), nil)
	if err != nil {
		panic(err)
	}

	if csrfToken != nil {
		request.AddCookie(csrfToken)
	}
	if sessionCookie != nil {
		request.AddCookie(sessionCookie)
	}
	return request, err
}

func buildPostRequest(apiPath string, requestData *bytes.Buffer) (*http.Request, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s%s", archiveBoxURL, apiPath), requestData)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate, private")
	request.Header.Set("Host", archiveBoxURL)
	request.Header.Set("Origin", archiveBoxURL)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Content-Length", "0")
	if csrfToken != nil {
		request.AddCookie(csrfToken)
	}
	if sessionCookie != nil {
		request.AddCookie(sessionCookie)
	}

	return request, nil
}

func setupArchiveBoxConnection() {
	if len(strings.TrimSpace(archiveBoxURL)) == 0 {
		connectionErr = fmt.Errorf("invalid empty url to archivebox")
		disconnect()
		return
	}
	if !isUrl(archiveBoxURL) {
		connectionErr = fmt.Errorf("url does not start with 'http[s]://'")
		disconnect()
		return
	}

	adminResp, err := httpClient.Get(fmt.Sprintf("%s/admin/login", archiveBoxURL))
	if err != nil {
		connectionErr = err
		disconnect()
		return
	}
	defer adminResp.Body.Close()

	for _, c := range adminResp.Cookies() {
		if c.Name == "csrftoken" {
			if len(c.Value) > 0 {
				csrfToken = c
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
		connectionErr = fmt.Errorf(csrfErrMsg)
		disconnect()
		return
	}
	if len(strings.TrimSpace(string(all))) > 0 {
		match := pattern.FindStringSubmatch(string(all))
		if len(match) > 1 && len(match[1]) > 0 {
			csrfMiddlewareToken = strings.TrimSpace(match[1])
		} else {
			log.Printf("Problem finding csrfmiddlewaretoken!\n")
		}
	}
	if isDebug {
		log.Printf("csrfmiddlewaretoken: %s\n", csrfMiddlewareToken)
	}

	if sessionCookie == nil {
		if len(csrfMiddlewareToken) > 0 && csrfToken != nil {
			doArchiveBoxLogin()
		} else {
			log.Printf("Cannot start login")
		}
	}
}

func isUrlAdded(urlToCheck string) bool {
	snapshotSearchPath := fmt.Sprintf("/admin/core/snapshot/?q=%s", url.QueryEscape(urlToCheck))

	request, err := buildGetRequest(snapshotSearchPath)
	if err != nil {
		log.Printf("Problem creating request\n")
		return false
	}

	do, err := httpClient.Do(request)
	if err != nil {
		log.Printf("Problem fetching snapshot search page\n")
		return false
	}
	defer do.Body.Close()

	content, err := ioutil.ReadAll(do.Body)
	if err != nil {
		log.Printf("Problem reading response body:%s\n", err.Error())
		return false
	}

	pattern := regexp.MustCompile("name=\"q\" value=\"" + urlToCheck + "\"")
	if pattern.Match(content) {
		return true
	}
	return false
}

func sendURLToArchiveBox(urlToSave string) (bool, error) {
	connect()
	urlToSave = strings.TrimSpace(urlToSave)
	if !IsConnected {
		return false, fmt.Errorf(t("NoConnectionToInstance"))
	}

	// validate url at first
	if len(urlToSave) < 5 {
		return false, fmt.Errorf(t("URLTooShort"))
	}
	if !isUrl(urlToSave) {
		return false, fmt.Errorf(t("InvalidURL"))
	}

	buffer := bytes.NewBuffer([]byte(fmt.Sprintf("csrfmiddlewaretoken=%s&url=%s&parser=auto&tag=&depth=0",
		csrfMiddlewareToken, url.QueryEscape(urlToSave))))
	request, err := buildPostRequest("/add/", buffer)
	if err != nil {
		panic(err)
	}
	transport := http.Transport{}
	do, err := transport.RoundTrip(request)
	if err != nil {
		msg := tWithArgs("ProblemCallingArchiveBox", struct {
			URL string
		}{URL: archiveBoxURL})
		log.Printf("%s\n", msg)
		return false, fmt.Errorf(msg)
	}
	defer do.Body.Close()
	log.Printf("entry add response status: %v\n", do.Status)

	// there is only 200...
	if do.StatusCode != 200 {
		// something went wrong
		log.Printf("Problem with add! Got status %d\n", do.StatusCode)
		return false, fmt.Errorf(tWithArgs("UnexpectedStatusCode", struct {
			Code string
		}{Code: strconv.Itoa(do.StatusCode)}))
	}
	return true, nil
}

// should be non-blocking to be safe for ui, handles validation of input
func saveURL(urlInput string) {
	infoLabel.Text = ""
	log.Printf("Started add event for url '%s'\n", urlInput)
	go func() {
		inputUrlEntryWidget.Disable()
		addToArchiveBtn.Disable()
		hasWorked, err := sendURLToArchiveBox(urlInput)
		if hasWorked {
			// all went fine!
			closeAppPref := application.Preferences().BoolWithFallback(PREFERENCE_CLOSE_AFTER_ADD, false)
			checkAfterAddPref := application.Preferences().BoolWithFallback(PREFERENCE_CHECK_ADD, false)
			if checkAfterAddPref {
				if isUrlAdded(urlInput) {
					application.SendNotification(&fyne.Notification{
						Title: tWithArgs("NotificationTitle", struct {
							APP_NAME string
						}{APP_NAME: appName}),
						Content: t("URLHasBeenAddedToArchivebox"),
					})
					if closeAppPref {
						application.Quit()
					}
				} else {
					application.SendNotification(&fyne.Notification{
						Title: tWithArgs("NotificationTitle", struct {
							APP_NAME string
						}{APP_NAME: appName}),
						Content: t("URLAddingCouldNotBeChecked"),
					})
				}
			} else {
				application.SendNotification(&fyne.Notification{
					Title: tWithArgs("NotificationTitle", struct {
						APP_NAME string
					}{APP_NAME: appName}),
					Content: t("URLHasBeenSentToArchivebox"),
				})
				if closeAppPref {
					application.Quit()
				}
			}
			return
		}
		if err != nil {
			infoLabel.Text = tWithArgs("ProblemAddingURL", struct {
				ERROR string
			}{ERROR: err.Error()})
		} else {
			infoLabel.Text = t("UnknownProblemAddingURL")
		}
		infoLabel.Refresh()
		inputUrlEntryWidget.Enable()
		addToArchiveBtn.Enable()
		window.Resize(windowSize)
	}()
	infoLabel.Refresh()
	log.Println("Finished add event")
}
