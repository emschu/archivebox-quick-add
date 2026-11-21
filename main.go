// archivebox-quick-add
// 2026 emschu[aet]mailbox.org
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
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// default http client
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// adding a URL may take a while on the server side
var addURLHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

var fyneApplication fyne.App
var window fyne.Window
var windowSize = fyne.Size{Width: 600, Height: 200}

// widgets
var inputEntryWidget *URLInputField
var addToArchiveBtn *widget.Button
var infoLabel *widget.Label

var appConfig applicationConfiguration
var appSessionState sessionState

var isDebug = false

// connectionConfigMu guards the connection settings that are read by request
// goroutines and written by the settings dialog.
var connectionConfigMu sync.RWMutex

// currentConnectionConfig returns a consistent snapshot of the connection settings.
func currentConnectionConfig() (string, string) {
	connectionConfigMu.RLock()
	defer connectionConfigMu.RUnlock()
	return appConfig.InstanceURL, appConfig.APIKey
}

// updateConnectionConfig replaces the connection settings shared with goroutines.
func updateConnectionConfig(instanceURL, apiKey string) {
	connectionConfigMu.Lock()
	defer connectionConfigMu.Unlock()
	appConfig.InstanceURL = instanceURL
	appConfig.APIKey = apiKey
}

// all app-wide vars besides the archivebox session state belong here
type applicationConfiguration struct {
	AppID           string
	AppName         string
	AppVersion      string
	AppLinkToGitHub string
	InstanceURL     string
	APIKey          string
	Localizer       *i18n.Localizer
}

// stores UI-related state shared between the UI thread and background goroutines
type sessionState struct {
	IsSubmissionBlocked atomicBool
	IsCloseBlocked      atomicBool
}

const (
	preferenceInstanceURL   = "InstanceURL"   // string
	preferenceAPIKey        = "APIKey"        // string
	preferenceCheckAdd      = "CheckAdd"      // bool
	preferenceCloseAfterAdd = "CloseAfterAdd" // bool
	preferenceFirstRun      = "FirstRun"      // bool
)

func main() {
	appConfig = applicationConfiguration{
		AppID:           "org.archivebox.go-quick-add",
		AppName:         "ArchiveBoxQuickAdd",
		AppVersion:      "1.14",
		AppLinkToGitHub: "https://github.com/emschu/archivebox-quick-add",
	}

	appSessionState = sessionState{}
	appSessionState.IsSubmissionBlocked = *newAtomicBool(false)
	appSessionState.IsCloseBlocked = *newAtomicBool(false)
	appConfig.initI18n()

	fyneApplication = app.NewWithID(appConfig.AppID)
	fyneApplication.SetIcon(resourceIconPng)

	isFirstRun := fyneApplication.Preferences().BoolWithFallback(preferenceFirstRun, true)
	if isFirstRun {
		// initial preference setup
		appConfig.doInitialPreferenceSetup()
	}

	// load archive box instance url and api key
	updateConnectionConfig(
		fyneApplication.Preferences().StringWithFallback(preferenceInstanceURL, "http://127.0.0.1:8000"),
		fyneApplication.Preferences().StringWithFallback(preferenceAPIKey, ""),
	)

	drv, ok := fyne.CurrentApp().Driver().(desktop.Driver)
	if ok {
		window = drv.CreateSplashWindow()
	} else {
		window = fyneApplication.NewWindow(appConfig.AppName)
	}

	window.CenterOnScreen()
	window.SetFixedSize(false)
	window.SetFullScreen(false)
	window.SetMaster()
	window.SetPadded(false)
	window.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			appConfig.safeClose()
		}
		if k.Name == fyne.KeyReturn {
			archiveURL(inputEntryWidget.Text)
		}
	})

	logoTextItem := canvas.NewText(appConfig.AppName, color.White)
	logoTextItem.Alignment = fyne.TextAlignCenter
	logoTextItem.TextSize = 25
	logoTextItem.TextStyle = fyne.TextStyle{
		Bold:      true,
		Italic:    false,
		Monospace: false,
	}

	instanceInfoLabel := widget.NewLabel(t("ArchiveBoxInstanceURL"))
	logoTextItem.Alignment = fyne.TextAlignCenter
	logoTextItem.TextSize = 18

	infoLabel = widget.NewLabel("") // empty at startup
	infoLabel.Wrapping = fyne.TextWrapBreak
	infoLabel.TextStyle = fyne.TextStyle{
		Bold:      true,
		Italic:    false,
		Monospace: false,
	}

	instanceURL, _ := currentConnectionConfig()
	parsedURL, err := url.Parse(instanceURL)
	var instanceLink fyne.Widget
	if err != nil {
		log.Printf("No valid url to archivebox instance\n")
		instanceLink = widget.NewLabel("")
	} else {
		instanceLink = widget.NewHyperlink(instanceURL, parsedURL)
	}

	inputEntryWidget = newURLInputField()

	go setupArchiveBoxConnection()

	addToArchiveBtn = widget.NewButtonWithIcon(t("AddToArchive"), theme.ContentAddIcon(), func() {})
	cancelBtn := widget.NewButtonWithIcon(t("Close"), theme.CancelIcon(), func() {
		appConfig.safeClose()
	})
	clipBoardBtn := widget.NewButtonWithIcon(t("PasteClipboard"), theme.ContentPasteIcon(), func() {
		pasteClipboard()
	})
	inputEntryWidget.OnSubmitted = func(s string) {
		archiveURL(s)
	}
	addToArchiveBtn.OnTapped = func() {
		archiveURL(inputEntryWidget.Text)
	}

	settingsBtn := widget.NewButtonWithIcon(t("Settings"), theme.SettingsIcon(), func() {
		showSettingsDialog()
	})

	infoBtn := widget.NewButtonWithIcon(t("Info"), theme.InfoIcon(), func() {
		appSessionState.IsSubmissionBlocked.setTrue()
		appSessionState.IsCloseBlocked.setTrue()
		informationD := dialog.NewInformation(t("Information"),
			fmt.Sprintf("%s\n%d - %s: %s\n%s: %s\n\n%s\n\n%s",
				appConfig.AppName, time.Now().Year(), t("Version"), appConfig.AppVersion,
				t("License"), "GNU Affero General Public License v3", appConfig.AppLinkToGitHub, t("InfoIndependence")), window)
		informationD.SetOnClosed(func() {
			appSessionState.IsSubmissionBlocked.setFalse()
			appSessionState.IsCloseBlocked.setFalse()
		})
		informationD.Show()
	})

	window.SetContent(container.NewVBox(
		container.NewHBox(
			layout.NewSpacer(),
			logoTextItem,
			layout.NewSpacer(),
			settingsBtn,
			infoBtn,
		),
		container.NewHBox(
			instanceInfoLabel,
			instanceLink,
		),
		infoLabel,
		inputEntryWidget,
		addToArchiveBtn,
		clipBoardBtn,
		cancelBtn,
	))
	window.Resize(windowSize)

	// called on startup
	go func() {
		time.Sleep(300 * time.Millisecond)
		fyne.DoAndWait(func() {
			window.Canvas().Focus(inputEntryWidget)
			pasteClipboard()
		})
	}()
	window.ShowAndRun()
}

func (*applicationConfiguration) doInitialPreferenceSetup() {
	fyneApplication.Preferences().SetString(preferenceInstanceURL, "http://127.0.0.1:5797")
	fyneApplication.Preferences().SetString(preferenceAPIKey, "")
	fyneApplication.Preferences().SetBool(preferenceCloseAfterAdd, true)
	fyneApplication.Preferences().SetBool(preferenceCheckAdd, true)

	fyneApplication.Preferences().SetBool(preferenceFirstRun, false)
}

func (*applicationConfiguration) initI18n() {
	var selectedLang = language.English
	var langResource = resourceEnJson

	// TODO extend language detection mechanism
	for _, s := range os.Environ() {
		envVar := strings.ToLower(s)
		if strings.HasPrefix(envVar, "lang") && strings.Contains(envVar, "de_de") {
			selectedLang = language.German
			langResource = resourceDeJson
		}
	}
	bundle := i18n.NewBundle(selectedLang)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustParseMessageFileBytes(langResource.StaticContent, langResource.StaticName)
	appConfig.Localizer = i18n.NewLocalizer(bundle, selectedLang.String())
}

// central method to translate strings, supports up to two format args
func tWithArgs(s string, args interface{}) string {
	localizeMessage, err := appConfig.Localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    s,
		TemplateData: args,
	})
	if err != nil {
		log.Printf("Warning! '%s'\n", err.Error())
		return s
	}
	if len(localizeMessage) == 0 {
		return s
	}
	return localizeMessage
}

func t(s string) string {
	message := &i18n.Message{
		ID: s,
	}
	localizeMessage, err := appConfig.Localizer.LocalizeMessage(message)
	if err != nil {
		log.Printf("Cannot get message '%s'\n", err.Error())
		return s
	}
	if len(localizeMessage) == 0 {
		return s
	}
	return localizeMessage
}

func (*applicationConfiguration) safeClose() {
	if appSessionState.IsCloseBlocked.isSet() {
		return
	}
	if len(strings.TrimSpace(inputEntryWidget.Text)) > 5 {
		// lock submission
		appSessionState.IsCloseBlocked.setTrue()
		appSessionState.IsSubmissionBlocked.setTrue()
		confirmD := dialog.NewConfirm(t("Cancel"), t("DoYouReallyWantToClose"), func(decision bool) {
			if decision { // = yes
				fyneApplication.Quit()
			}
		}, window)
		confirmD.SetOnClosed(func() {
			appSessionState.IsSubmissionBlocked.setFalse()
			appSessionState.IsCloseBlocked.setFalse()
		})
		confirmD.Show()
	} else {
		// close immediately if input is empty
		fyneApplication.Quit()
	}
}

func (*applicationConfiguration) disconnect(err error) {
	log.Printf("Warn: No connection could be established!\n")
	msg := t("NoConnectionPossible")
	if err != nil {
		msg += " " + err.Error()
	}
	fyne.Do(func() {
		infoLabel.Text = msg
		infoLabel.Refresh()
	})
}

type atomicBool int32

func (b *atomicBool) isSet() bool { return atomic.LoadInt32((*int32)(b)) != 0 }
func (b *atomicBool) setTrue()    { atomic.StoreInt32((*int32)(b), 1) }
func (b *atomicBool) setFalse()   { atomic.StoreInt32((*int32)(b), 0) }

func newAtomicBool(startVal bool) *atomicBool {
	var ab atomicBool
	if startVal {
		ab.setTrue()
	} else {
		ab.setFalse()
	}
	return &ab
}
