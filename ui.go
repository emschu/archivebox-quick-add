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
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/cmd/fyne_settings/settings"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"log"
	"strings"
)

var isAppearanceWindowOpen = false

func pasteClipboard() {
	currentClipboard := window.Clipboard().Content()
	if len(currentClipboard) > 0 && isURL(currentClipboard) {
		inputEntryWidget.SetText(strings.TrimSpace(currentClipboard))
	}
}

func showSettingsDialog() {
	var items []*widget.FormItem
	instanceURLEntry := widget.NewEntry()
	instanceURLEntry.Text = application.Preferences().StringWithFallback(preferenceInstanceURL, "http://127.0.0.1:8000")
	instanceURLEntry.Validator = validation.NewRegexp("^http[s]?://.*:[0-9]{1,5}$", "invalid URL")
	items = append(items, widget.NewFormItem(t("ArchiveBoxURL"), instanceURLEntry))

	userNameEntry := widget.NewEntry()
	userNameEntry.Text = application.Preferences().StringWithFallback(preferenceUsername, "")
	userNameEntry.Validator = validation.NewRegexp("^\\s*\\S{2,}\\s*$", "too short")
	items = append(items, widget.NewFormItem(t("Username"), userNameEntry))

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.ActionItem = nil
	currentPw := application.Preferences().StringWithFallback(preferencePassword, "")
	if len(currentPw) > 0 {
		passwordEntry.SetPlaceHolder(t("AlreadySet"))
		passwordEntry.OnChanged = func(s string) {
			if len(strings.TrimSpace(s)) > 0 && passwordEntry.Validator == nil {
				passwordEntry.Validator = validation.NewRegexp("^\\s*\\S{2,}\\s*$", "too short")
			} else {
				passwordEntry = nil
			}
		}
	} else {
		passwordEntry.Validator = validation.NewRegexp("^\\s*\\S{2,}\\s*$", "too short")
	}
	items = append(items, widget.NewFormItem(t("Password"), passwordEntry))

	borderlessCheckbox := widget.NewCheck("", func(b bool) {})
	isBorderless := application.Preferences().BoolWithFallback(preferenceBorderless, true)
	borderlessCheckbox.Checked = isBorderless
	items = append(items, widget.NewFormItem(t("BorderlessWindow"), borderlessCheckbox))

	linkAddCheckCheckbox := widget.NewCheck("", func(b bool) {})
	isAddChecked := application.Preferences().BoolWithFallback(preferenceCheckAdd, false)
	linkAddCheckCheckbox.Checked = isAddChecked
	items = append(items, widget.NewFormItem(t("CheckIfURLWasAdded"), linkAddCheckCheckbox))

	closeAfterAddCheckbox := widget.NewCheck("", func(b bool) {})
	isCloseAfterAdd := application.Preferences().BoolWithFallback(preferenceCloseAfterAdd, false)
	closeAfterAddCheckbox.Checked = isCloseAfterAdd
	items = append(items, widget.NewFormItem(t("CloseAppAfterAddingAURL"), closeAfterAddCheckbox))

	newSettings := settings.NewSettings()
	appearanceBtn := widget.NewButtonWithIcon("", newSettings.AppearanceIcon(), func() {
		// open fine settings
		showFyneSettingsWindow()
	})
	items = append(items, widget.NewFormItem(t("Appearance"), appearanceBtn))

	settingsDialog := dialog.NewForm(t("Settings"), t("Apply"), t("Cancel"), items, func(b bool) {
		if b {
			log.Printf("Updating preferences! \n")
			application.Preferences().SetString(preferenceInstanceURL, strings.TrimSpace(instanceURLEntry.Text))
			application.Preferences().SetString(preferenceUsername, strings.TrimSpace(userNameEntry.Text))
			inputPw := strings.TrimSpace(passwordEntry.Text)
			if len(inputPw) > 0 {
				application.Preferences().SetString(preferencePassword, inputPw)
			}
			application.Preferences().SetBool(preferenceBorderless, borderlessCheckbox.Checked)
			application.Preferences().SetBool(preferenceCheckAdd, linkAddCheckCheckbox.Checked)
			application.Preferences().SetBool(preferenceCloseAfterAdd, closeAfterAddCheckbox.Checked)
		}
	}, window)

	window.Resize(fyne.Size{
		Width:  750,
		Height: 500,
	})
	settingsDialog.Resize(fyne.Size{
		Width:  500,
		Height: 250,
	})
	settingsDialog.SetOnClosed(func() {
		// restore original main window settings
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

	settingsWindow := application.NewWindow(t("AppearanceSettings"))
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
		saveURL(inputEntryWidget.Text)
	} else {
		s.Entry.TypedShortcut(shortcut)
	}
}
