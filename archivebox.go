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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
)

const apiBasePath = "/api/v1/"

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

// buildAPIRequest creates a request against the ArchiveBox REST API and
// authenticates it with the configured API key.
func buildAPIRequest(method, apiPath string, requestBody []byte) (*http.Request, error) {
	instanceURL := strings.TrimRight(strings.TrimSpace(appConfig.InstanceURL), "/")
	requestURL := instanceURL + apiBasePath + strings.TrimLeft(apiPath, "/")

	var reader io.Reader
	if requestBody != nil {
		reader = bytes.NewReader(requestBody)
	}
	request, err := http.NewRequest(method, requestURL, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(appConfig.APIKey))
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

// checkArchiveBoxConnection verifies the configured API key using the
// /auth/check_api_token endpoint. It is free of UI calls so it can be tested
// without a running Fyne application.
func checkArchiveBoxConnection() error {
	appSessionState.IsConnected = false

	if len(strings.TrimSpace(appConfig.InstanceURL)) == 0 {
		return fmt.Errorf("invalid empty url to archivebox")
	}
	if !isURL(appConfig.InstanceURL) {
		return fmt.Errorf("url does not start with 'http[s]://'")
	}
	if len(strings.TrimSpace(appConfig.APIKey)) == 0 {
		return fmt.Errorf("%s", t("InvalidAPIKey"))
	}

	credentials, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: strings.TrimSpace(appConfig.APIKey)})
	if err != nil {
		return err
	}
	request, err := buildAPIRequest(http.MethodPost, "auth/check_api_token", credentials)
	if err != nil {
		return err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", tWithArgs("UnexpectedStatusCode", struct {
			Code int
		}{Code: response.StatusCode}))
	}

	var checkResult struct {
		Success  bool   `json:"success"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(response.Body).Decode(&checkResult); err != nil {
		return fmt.Errorf("invalid API response: %w", err)
	}
	if !checkResult.Success {
		return fmt.Errorf("%s", t("InvalidAPIKey"))
	}

	appSessionState.IsConnected = true
	if isDebug {
		log.Printf("Authenticated as '%s'\n", checkResult.Username)
	}
	return nil
}

// setupArchiveBoxConnection is the UI-facing wrapper around the connection check.
func setupArchiveBoxConnection() {
	if err := checkArchiveBoxConnection(); err != nil {
		appSessionState.ConnectionErr = err
		appConfig.disconnect()
		return
	}
	appSessionState.ConnectionErr = nil
}

// cliCommandResponse mirrors the response of the ArchiveBox CLI API endpoints.
type cliCommandResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors"`
	Result  any      `json:"result"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
}

// addURLToArchiveBox submits a URL through POST /api/v1/cli/add.
func addURLToArchiveBox(urlToSave string) (bool, error) {
	urlToSave = strings.TrimSpace(urlToSave)
	if len(urlToSave) < 5 {
		return false, fmt.Errorf("%s", t("URLTooShort"))
	}
	if !isURL(urlToSave) {
		return false, fmt.Errorf("%s", t("InvalidURL"))
	}

	payload, err := json.Marshal(struct {
		URLs   []string `json:"urls"`
		Parser string   `json:"parser"`
		Tag    string   `json:"tag"`
		Depth  int      `json:"depth"`
	}{
		URLs:   []string{urlToSave},
		Parser: "auto",
		Tag:    "",
		Depth:  0,
	})
	if err != nil {
		return false, err
	}
	request, err := buildAPIRequest(http.MethodPost, "cli/add", payload)
	if err != nil {
		return false, err
	}

	response, err := addURLHTTPClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("%s", tWithArgs("ProblemCallingArchiveBox", struct {
			URL string
		}{URL: appConfig.InstanceURL}))
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s", tWithArgs("UnexpectedStatusCode", struct {
			Code int
		}{Code: response.StatusCode}))
	}

	var result cliCommandResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("invalid API response: %w", err)
	}
	if !result.Success {
		if len(result.Errors) > 0 {
			return false, fmt.Errorf("%s", strings.Join(result.Errors, "; "))
		}
		return false, fmt.Errorf("%s", t("UnknownProblemAddingURL"))
	}
	if isDebug {
		log.Printf("entry add response: %s\n", result.Stdout)
	}
	return true, nil
}

func sendURLToArchiveBox(urlToSave string) (bool, error) {
	if err := checkArchiveBoxConnection(); err != nil {
		appSessionState.ConnectionErr = err
		appConfig.disconnect()
		return false, fmt.Errorf("%s", t("NoConnectionToInstance"))
	}
	return addURLToArchiveBox(urlToSave)
}

// snapshotURLCount returns the number of snapshots matching an exact URL.
func snapshotURLCount(urlToCheck string) (int, error) {
	apiPath := fmt.Sprintf("core/snapshots?url=%s&limit=1", url.QueryEscape(urlToCheck))
	request, err := buildAPIRequest(http.MethodGet, apiPath, nil)
	if err != nil {
		return 0, err
	}
	response, err := archiveSearchHTTPClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s", tWithArgs("UnexpectedStatusCode", struct {
			Code int
		}{Code: response.StatusCode}))
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("invalid API response: %w", err)
	}
	return result.Count, nil
}

func isURLAlreadyArchived(urlToCheck string) bool {
	urlToCheck = strings.TrimSpace(urlToCheck)
	if !appSessionState.IsConnected {
		return false
	}
	if len(urlToCheck) < 5 || !isURL(urlToCheck) {
		return false
	}
	// ArchiveBox normalizes trailing slashes away on some URLs, so check both forms.
	candidates := []string{urlToCheck}
	if strings.HasSuffix(urlToCheck, "/") {
		candidates = append(candidates, strings.TrimRight(urlToCheck, "/"))
	} else {
		candidates = append(candidates, urlToCheck+"/")
	}
	for _, candidate := range candidates {
		count, err := snapshotURLCount(candidate)
		if err != nil {
			log.Printf("Problem fetching snapshots: %v\n", err)
			return false
		}
		if count > 0 {
			return true
		}
	}
	return false
}

// should be non-blocking to be safe for ui, handles validation of input
func archiveURL(urlInput string) {
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
		fyne.Do(func() {
			inputEntryWidget.Disable()
			addToArchiveBtn.Disable()
		})
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
					fyne.Do(func() {
						if closeAppPref {
							fyneApplication.Quit()
						}
						inputEntryWidget.SetText("")
					})
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
				fyne.Do(func() {
					inputEntryWidget.SetText("")
				})
			}
		}
		if err != nil {
			errMsg := tWithArgs("ProblemAddingURL", struct {
				ERROR string
			}{ERROR: err.Error()})
			fyne.Do(func() {
				infoLabel.Text = errMsg
			})
		}
		fyne.Do(func() {
			infoLabel.Refresh()
			inputEntryWidget.Enable()
			inputEntryWidget.Refresh()
			addToArchiveBtn.Enable()
			addToArchiveBtn.Refresh()
			window.Resize(windowSize)
		})
	}()
	infoLabel.Refresh()
	log.Println("Finished URL archiving process")
}
