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
	"log"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/cmd/fyne_settings/settings"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var isAppearanceWindowOpen = false

// pasteClipboard reads the clipboard and pre-fills the URL input field.
// It must be called on the main Fyne thread (wrap calls from goroutines in fyne.DoAndWait).
func pasteClipboard() {
	clipboard := window.Clipboard()
	if clipboard != nil {
		currentClipboard := clipboard.Content()
		if len(currentClipboard) > 0 && isURL(currentClipboard) {
			inputEntryWidget.SetText(strings.TrimSpace(currentClipboard))
		}
	} else {
		log.Printf("Could not access clipboard!\n")
	}
}

func showSettingsDialog() {
	var items []*widget.FormItem
	instanceURLEntry := widget.NewEntry()
	instanceURLEntry.Text = fyneApplication.Preferences().StringWithFallback(preferenceInstanceURL, "http://127.0.0.1:8000")
	instanceURLEntry.Validator = validation.NewRegexp("^https?://[^\\s]+$", "invalid URL")
	items = append(items, widget.NewFormItem(t("ArchiveBoxURL"), instanceURLEntry))

	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.ActionItem = nil
	currentAPIKey := fyneApplication.Preferences().StringWithFallback(preferenceAPIKey, "")
	apiKeyValidator := validation.NewRegexp("^\\s*\\S{8,}\\s*$", "too short")
	if len(currentAPIKey) > 0 {
		apiKeyEntry.SetPlaceHolder(t("AlreadySet"))
		apiKeyEntry.OnChanged = func(s string) {
			if len(strings.TrimSpace(s)) > 0 {
				apiKeyEntry.Validator = apiKeyValidator
			} else {
				apiKeyEntry.Validator = nil
			}
		}
	} else {
		apiKeyEntry.Validator = apiKeyValidator
	}
	items = append(items, widget.NewFormItem(t("APIKey"), apiKeyEntry))
	items = append(items, widget.NewFormItem("", widget.NewLabel(t("APIKeyHint"))))

	borderlessCheckbox := widget.NewCheck("", func(b bool) {})
	isBorderless := fyneApplication.Preferences().BoolWithFallback(preferenceBorderless, true)
	borderlessCheckbox.Checked = isBorderless
	items = append(items, widget.NewFormItem(t("BorderlessWindow"), borderlessCheckbox))

	linkAddCheckCheckbox := widget.NewCheck("", func(b bool) {})
	isAddChecked := fyneApplication.Preferences().BoolWithFallback(preferenceCheckAdd, false)
	linkAddCheckCheckbox.Checked = isAddChecked
	items = append(items, widget.NewFormItem(t("CheckIfURLWasAdded"), linkAddCheckCheckbox))

	closeAfterAddCheckbox := widget.NewCheck("", func(b bool) {})
	isCloseAfterAdd := fyneApplication.Preferences().BoolWithFallback(preferenceCloseAfterAdd, false)
	closeAfterAddCheckbox.Checked = isCloseAfterAdd
	items = append(items, widget.NewFormItem(t("CloseAppAfterArchiving"), closeAfterAddCheckbox))

	newSettings := settings.NewSettings()
	appearanceBtn := widget.NewButtonWithIcon("", newSettings.AppearanceIcon(), func() {
		// open fine settings
		showFyneSettingsWindow()
	})
	items = append(items, widget.NewFormItem(t("Appearance"), appearanceBtn))

	appSessionState.IsCloseBlocked.setTrue()
	appSessionState.IsSubmissionBlocked.setTrue()
	settingsDialog := dialog.NewForm(t("Settings"), t("Apply"), t("Cancel"), items, func(b bool) {
		if b {
			log.Printf("Updating preferences! \n")
			fyneApplication.Preferences().SetString(preferenceInstanceURL, strings.TrimSpace(instanceURLEntry.Text))
			inputAPIKey := strings.TrimSpace(apiKeyEntry.Text)
			if len(inputAPIKey) > 0 {
				fyneApplication.Preferences().SetString(preferenceAPIKey, inputAPIKey)
			}
			fyneApplication.Preferences().SetBool(preferenceBorderless, borderlessCheckbox.Checked)
			fyneApplication.Preferences().SetBool(preferenceCheckAdd, linkAddCheckCheckbox.Checked)
			fyneApplication.Preferences().SetBool(preferenceCloseAfterAdd, closeAfterAddCheckbox.Checked)

			// use the new values without requiring a restart
			appConfig.InstanceURL = fyneApplication.Preferences().String(preferenceInstanceURL)
			appConfig.APIKey = fyneApplication.Preferences().String(preferenceAPIKey)
			go setupArchiveBoxConnection()
		}
	}, window)

	window.Resize(fyne.Size{
		Width:  750,
		Height: 500,
	})
	settingsDialog.Resize(fyne.Size{
		Width:  450,
		Height: 300,
	})
	settingsDialog.SetOnClosed(func() {
		// restore original main window settings
		appSessionState.IsCloseBlocked.setFalse()
		appSessionState.IsSubmissionBlocked.setFalse()
		window.Resize(windowSize)
	})
	settingsDialog.Show()
}

func showFyneSettingsWindow() {
	if isAppearanceWindowOpen {
		return
	}
	isAppearanceWindowOpen = true
	fyneSettings := settings.NewSettings()

	settingsWindow := fyneApplication.NewWindow(t("AppearanceSettings"))
	settingsWindow.SetOnClosed(func() {
		isAppearanceWindowOpen = false
	})

	appearance := fyneSettings.LoadAppearanceScreen(settingsWindow)
	tabs := container.NewAppTabs(
		&container.TabItem{Text: t("Appearance"), Icon: fyneSettings.AppearanceIcon(), Content: appearance})
	tabs.SetTabLocation(container.TabLocationLeading)

	settingsWindow.SetContent(tabs)
	settingsWindow.CenterOnScreen()
	settingsWindow.Resize(fyne.NewSize(500, 300))
	settingsWindow.Show()
}

// URLInputField wrapper for the input field to have control over custom shortcuts
type URLInputField struct {
	*widget.Entry
}

func newURLInputField() *URLInputField {
	entry := &URLInputField{&widget.Entry{}}
	entry.ExtendBaseWidget(entry)
	entry.SetPlaceHolder(t("EnterURL"))
	entry.MultiLine = true
	entry.Wrapping = fyne.TextWrapBreak
	return entry
}

// TypedShortcut override
func (s *URLInputField) TypedShortcut(shortcut fyne.Shortcut) {
	log.Printf("%v\n", shortcut.ShortcutName())
	if shortcut.ShortcutName() == "CustomDesktop:Control+Return" {
		archiveURL(inputEntryWidget.Text)
	} else {
		s.Entry.TypedShortcut(shortcut)
	}
}
