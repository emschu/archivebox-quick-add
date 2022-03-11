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
	"encoding/json"
	"fmt"
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
	"image/color"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var httpClient = &http.Client{
	Timeout: time.Second * 10,
}

// preferences
var archiveBoxURL string

var application fyne.App
var windowSize = fyne.Size{600, 200}

// widgets
var inputEntryWidget *URLInputField
var addToArchiveBtn *widget.Button
var infoLabel *widget.Label
var isConnected = false
var isSubmissionBlocked = newAtomicBool(false)
var isCloseBlocked = newAtomicBool(false)

var window fyne.Window

var csrfToken *http.Cookie
var sessionCookie *http.Cookie
var csrfMiddlewareToken string
var connectionErr error // store latest error
var appName = "ArchiveBox Quick-Add"
var appVersion = "1.3"
var appLinkToGitHub = "https://github.com/emschu/archivebox-quick-add"
var isDebug = false
var localizer *i18n.Localizer

const (
	appID                   = "org.archivebox.go-quick-add"
	preferenceInstanceURL   = "InstanceURL"   // string
	preferenceUsername      = "Username"      // string
	preferencePassword      = "Password"      // string
	preferenceBorderless    = "Borderless"    // bool
	preferenceCheckAdd      = "CheckAdd"      // bool
	preferenceCloseAfterAdd = "CloseAfterAdd" // bool
	preferenceFirstRun      = "FirstRun"      // bool
)

func main() {
	application = app.NewWithID(appID)
	application.SetIcon(resourceIconPng)

	initI18n()

	isFirstRun := application.Preferences().BoolWithFallback(preferenceFirstRun, true)
	if isFirstRun {
		// initial preference setup
		doInitialPreferenceSetup()
	}

	archiveBoxURL = application.Preferences().StringWithFallback(preferenceInstanceURL, "http://127.0.0.1:8000")
	isSplashScreen := application.Preferences().BoolWithFallback(preferenceBorderless, true)
	drv, ok := fyne.CurrentApp().Driver().(desktop.Driver)
	if ok && isSplashScreen {
		window = drv.CreateSplashWindow()
	} else {
		window = application.NewWindow(appName)
	}

	window.CenterOnScreen()
	window.SetFixedSize(false)
	window.SetFullScreen(false)
	window.SetMaster()
	window.SetPadded(true)
	window.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			safeClose()
		}
		if k.Name == fyne.KeyReturn {
			archiveURL(inputEntryWidget.Text)
		}
	})

	logoTextItem := canvas.NewText(appName, color.White)
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

	defer func() {
		doArchiveBoxLogout()
	}()

	parsedURL, err := url.Parse(archiveBoxURL)
	var instanceLink fyne.Widget
	if err != nil {
		log.Printf("No valid url to archivebox instance\n")
		instanceLink = widget.NewLabel("")
	} else {
		instanceLink = widget.NewHyperlink(archiveBoxURL, parsedURL)
	}

	inputEntryWidget = newURLInputField()

	go setupArchiveBoxConnection()

	addToArchiveBtn = widget.NewButtonWithIcon(t("AddToArchive"), theme.ContentAddIcon(), func() {})
	cancelBtn := widget.NewButtonWithIcon(t("Close"), theme.CancelIcon(), func() {
		safeClose()
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
		isSubmissionBlocked.setTrue()
		isCloseBlocked.setTrue()
		informationD := dialog.NewInformation(t("Information"),
			fmt.Sprintf("%s\n%d - %s: %s\n%s: %s\n\n%s\n\n%s",
				appName, time.Now().Year(), t("Version"), appVersion,
				t("License"), "GNU Affero General Public License v3", appLinkToGitHub, t("InfoIndependence")), window)
		informationD.SetOnClosed(func() {
			isSubmissionBlocked.setFalse()
			isCloseBlocked.setFalse()
		})
		informationD.Show()
	})

	window.SetContent(container.NewVBox(
		container.New(layout.NewHBoxLayout(),
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
		window.Canvas().Focus(inputEntryWidget)

		pasteClipboard()
	}()
	window.ShowAndRun()
}

func doInitialPreferenceSetup() {
	application.Preferences().SetString(preferenceInstanceURL, "http://127.0.0.1:8000")
	application.Preferences().SetString(preferenceUsername, "")
	application.Preferences().SetString(preferencePassword, "")
	application.Preferences().SetBool(preferenceBorderless, true)
	application.Preferences().SetBool(preferenceCloseAfterAdd, true)
	application.Preferences().SetBool(preferenceCheckAdd, true)

	application.Preferences().SetBool(preferenceFirstRun, false)
}

func initI18n() {
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
	localizer = i18n.NewLocalizer(bundle, selectedLang.String())
}

// central method to translate strings, supports up to two format args
func tWithArgs(s string, args interface{}) string {
	localizeMessage, err := localizer.Localize(&i18n.LocalizeConfig{
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
	localizeMessage, err := localizer.LocalizeMessage(message)
	if err != nil {
		log.Printf("Cannot get message '%s'\n", err.Error())
		return s
	}
	if len(localizeMessage) == 0 {
		return s
	}
	return localizeMessage
}

func safeClose() {
	if isCloseBlocked.isSet() {
		return
	}
	if len(strings.TrimSpace(inputEntryWidget.Text)) > 5 {
		// lock submission
		isCloseBlocked.setTrue()
		isSubmissionBlocked.setTrue()
		confirmD := dialog.NewConfirm(t("Cancel"), t("DoYouReallyWantToClose"), func(decision bool) {
			if decision { // = yes
				application.Quit()
			}
		}, window)
		confirmD.SetOnClosed(func() {
			isSubmissionBlocked.setFalse()
			isCloseBlocked.setFalse()
		})
		confirmD.Show()
	} else {
		// close immediately if input is empty
		application.Quit()
	}
}

func disconnect() {
	isConnected = false
	log.Printf("Warn: No connection could be established!\n")
	infoLabel.Text = t("NoConnectionPossible")
	if connectionErr != nil {
		infoLabel.Text += " " + connectionErr.Error()
	}
	infoLabel.Refresh()
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
