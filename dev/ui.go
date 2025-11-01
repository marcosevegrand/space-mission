package main

import (
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	var menu *tview.List

	// Server config forms
	missionForm := tview.NewForm().
		AddInputField("Mission Address", "", 20, nil, nil).
		AddInputField("Timeout (ms)", "", 20, nil, nil).
		AddButton("Save", func() {}).
		AddButton("Back", func() { app.SetRoot(menu, true) })

	telemetryForm := tview.NewForm().
		AddInputField("Telemetry Address", "", 20, nil, nil).
		AddInputField("Timeout (ms)", "", 20, nil, nil).
		AddButton("Save", func() {}).
		AddButton("Back", func() { app.SetRoot(menu, true) })

	// Telemetry view
	telemetryView := tview.NewTextView().
		SetText("Incoming telemetry stream...\n[stream output would appear here]\n").
		SetScrollable(true).
		SetChangedFunc(func() { app.Draw() })

	telemetryBox := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(telemetryView, 0, 1, false).
		AddItem(tview.NewButton("Back").SetSelectedFunc(func() { app.SetRoot(menu, true) }), 3, 1, true)

	// Add mission form
	addMissionForm := tview.NewForm().
		AddInputField("Mission Name", "", 20, nil, nil).
		AddInputField("Mission Data", "", 20, nil, nil).
		AddButton("Send", func() {}).
		AddButton("Back", func() { app.SetRoot(menu, true) })

	// Main menu
	menu = tview.NewList().
		AddItem("Configure Mission Link Server", "", '1', func() { app.SetRoot(missionForm, true) }).
		AddItem("Configure Telemetry Stream Server", "", '2', func() { app.SetRoot(telemetryForm, true) }).
		AddItem("View Telemetry Stream", "", '3', func() { app.SetRoot(telemetryBox, true) }).
		AddItem("Add Mission (send to rover)", "", '4', func() { app.SetRoot(addMissionForm, true) }).
		AddItem("Exit", "", 'q', func() { app.Stop() })

	if err := app.SetRoot(menu, true).Run(); err != nil {
		panic(err)
	}
}
