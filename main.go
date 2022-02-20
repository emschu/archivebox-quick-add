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
var inputUrlEntryWidget *URLInputField
var addToArchiveBtn *widget.Button
var infoLabel *widget.Label
var IsConnected bool = false

var window fyne.Window

var csrfToken *http.Cookie
var sessionCookie *http.Cookie
var csrfMiddlewareToken string
var connectionErr error = nil // store latest error
var appName = "ArchiveBox Quick-Add"
var appVersion = "1.0"
var appLinkToGitHub = "https://github.com/emschu/archivebox-quick-add"
var isDebug = false
var localizer *i18n.Localizer

const (
	APP_ID                     = "org.archivebox.go-quick-add"
	PREFERENCE_INSTANCE_URL    = "InstanceURL"   // string
	PREFERENCE_USERNAME        = "Username"      // string
	PREFERENCE_PASSWORD        = "Password"      // string
	PREFERENCE_BORDERLESS      = "Borderless"    // bool
	PREFERENCE_CHECK_ADD       = "CheckAdd"      // bool
	PREFERENCE_CLOSE_AFTER_ADD = "CloseAfterAdd" // bool
	PREFERENCE_FIRST_RUN       = "FirstRun"      // bool
)

func main() {
	application = app.NewWithID(APP_ID)
	path, err := fyne.LoadResourceFromPath("./Icon.png")
	if err != nil {
		log.Printf("Error loading app's Icon.png!\n")
	}
	application.SetIcon(path)

	initI18n()

	isFirstRun := application.Preferences().BoolWithFallback(PREFERENCE_FIRST_RUN, true)
	if isFirstRun {
		// initial preference setup
		doInitialPreferenceSetup()
	}

	archiveBoxURL = application.Preferences().StringWithFallback(PREFERENCE_INSTANCE_URL, "http://127.0.0.1:8000")
	isSplashScreen := application.Preferences().BoolWithFallback(PREFERENCE_BORDERLESS, true)
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
			saveURL(inputUrlEntryWidget.Text)
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

	inputUrlEntryWidget = NewURLInputField()

	go setupArchiveBoxConnection()

	addToArchiveBtn = widget.NewButtonWithIcon(t("AddToArchive"), theme.ContentAddIcon(), func() {})
	cancelBtn := widget.NewButtonWithIcon(t("Close"), theme.CancelIcon(), func() {
		safeClose()
	})
	clipBoardBtn := widget.NewButtonWithIcon(t("PasteClipboard"), theme.ContentPasteIcon(), func() {
		pasteClipboard()
	})
	inputUrlEntryWidget.OnSubmitted = func(s string) {
		saveURL(s)
	}
	addToArchiveBtn.OnTapped = func() {
		saveURL(inputUrlEntryWidget.Text)
	}

	settingsBtn := widget.NewButtonWithIcon(t("Settings"), theme.SettingsIcon(), func() {
		showSettingsDialog()
	})

	infoBtn := widget.NewButtonWithIcon(t("Info"), theme.InfoIcon(), func() {
		dialog.NewInformation(t("Information"),
			fmt.Sprintf("%s\n%d - %s: %s\n%s: %s\n\n%s\n\n%s",
				appName, time.Now().Year(), appVersion, t("Version"),
				"GNU Affero General Public License v3", t("License"), appLinkToGitHub, t("InfoIndependence")), window).Show()
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
		inputUrlEntryWidget,
		addToArchiveBtn,
		clipBoardBtn,
		cancelBtn,
	))
	window.Resize(windowSize)

	// called on startup
	go func() {
		time.Sleep(300 * time.Millisecond)
		window.Canvas().Focus(inputUrlEntryWidget)

		pasteClipboard()
	}()
	window.ShowAndRun()
}

func doInitialPreferenceSetup() {
	application.Preferences().SetString(PREFERENCE_INSTANCE_URL, "http://127.0.0.1:8000")
	application.Preferences().SetString(PREFERENCE_USERNAME, "")
	application.Preferences().SetString(PREFERENCE_PASSWORD, "")
	application.Preferences().SetBool(PREFERENCE_BORDERLESS, true)
	application.Preferences().SetBool(PREFERENCE_CLOSE_AFTER_ADD, true)
	application.Preferences().SetBool(PREFERENCE_CHECK_ADD, false)

	application.Preferences().SetBool(PREFERENCE_FIRST_RUN, false)
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
	if len(strings.TrimSpace(inputUrlEntryWidget.Text)) > 5 {
		dialog.ShowConfirm(t("Cancel"), t("DoYouReallyWantToClose"), func(decision bool) {
			if decision { // = yes
				application.Quit()
			}
		}, window)
	} else {
		// close immediately if input is empty
		application.Quit()
	}
}

func connect() {
	setupArchiveBoxConnection()
}

func disconnect() {
	IsConnected = false
	log.Printf("Warn: No connection could be established!\n")
	infoLabel.Text = t("NoConnectionPossible")
	if connectionErr != nil {
		infoLabel.Text += " " + connectionErr.Error()
	}
	infoLabel.Refresh()
}
