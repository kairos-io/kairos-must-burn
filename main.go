package main

import (
	"context"
	_ "embed"
	"os"
	"runtime"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

//go:embed Resources/kairos-must-burn.png
var logoData []byte
var lastVersionList []string
var isoPath string // Make isoPath package-level

func main() {
	f, err := os.CreateTemp("", "logo.png")
	if err != nil {
		panic("Failed to create temporary logo file: " + err.Error())
	}
	defer os.Remove(f.Name()) // Clean up temporary file
	if _, err := f.Write(logoData); err != nil {
		panic("Failed to write logo data to temporary file: " + err.Error())
	}
	f.Close() // Close the file to ensure it's written

	app := gtk.NewApplication("com.kairos.isoburn", gio.ApplicationHandlesOpen|gio.ApplicationDefaultFlags)

	app.ConnectActivate(func() {
		win := gtk.NewApplicationWindow(app)
		win.SetTitle("Kairos Must Burn")

		// Check for elevated permissions immediately
		hasPermissions, err := CheckElevatedPermissions()
		if !hasPermissions {
			// Create permission error dialog
			dialog := gtk.NewDialog()
			dialog.SetTitle("Elevated Permissions Required")
			dialog.SetTransientFor(&win.Window)
			dialog.SetModal(true)

			contentArea := dialog.ContentArea()
			box := gtk.NewBox(gtk.OrientationVertical, 10)
			box.SetMarginTop(20)
			box.SetMarginBottom(20)
			box.SetMarginStart(20)
			box.SetMarginEnd(20)

			icon := gtk.NewImageFromIconName("dialog-warning")
			icon.SetPixelSize(48)
			icon.SetMarginBottom(10)
			box.Append(icon)

			msgLabel := gtk.NewLabel("This application requires elevated permissions to write to USB devices.")
			msgLabel.SetHAlign(gtk.AlignCenter)
			msgLabel.SetWrap(true)
			msgLabel.SetMarginBottom(10)
			box.Append(msgLabel)

			var errorMsg string
			if err != nil {
				errorMsg = "Error: " + err.Error()
			} else {
				if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
					errorMsg = "Please run this application with sudo."
				} else if runtime.GOOS == "windows" {
					errorMsg = "Please right-click and select 'Run as administrator'."
				} else {
					errorMsg = "Please run this application with elevated privileges."
				}
			}

			errorLabel := gtk.NewLabel(errorMsg)
			errorLabel.SetHAlign(gtk.AlignCenter)
			errorLabel.SetWrap(true)
			box.Append(errorLabel)

			contentArea.Append(box)

			// Create a button box with proper margins and styling
			buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
			buttonBox.SetMarginTop(20)
			buttonBox.SetMarginBottom(20)
			buttonBox.SetMarginStart(20)
			buttonBox.SetMarginEnd(20)
			buttonBox.SetHAlign(gtk.AlignCenter)

			// Create exit button with styling
			exitBtn := gtk.NewButton()
			exitBtn.SetLabel("Exit")
			exitBtn.SetCSSClasses([]string{"suggested-action"}) // Add styling to make button more prominent
			exitBtn.SetMarginTop(10)
			exitBtn.SetMarginBottom(10)
			exitBtn.SetMarginStart(40)
			exitBtn.SetMarginEnd(40)

			buttonBox.Append(exitBtn)
			contentArea.Append(buttonBox)

			// Show dialog
			dialog.SetVisible(true)
			dialog.Present()
			dialog.Focus()

			// Connect exit button click
			exitBtn.ConnectClicked(func() {
				app.Quit()
			})

			// Also handle dialog close
			dialog.Connect("close-request", func() bool {
				app.Quit()
				return false
			})

			return
		}

		burnBtn := gtk.NewButtonWithLabel("🔥 Burn!")
		burnBtn.SetSensitive(false)

		var drive string

		isoBtn := gtk.NewButtonWithLabel("💿 Select ISO")
		isoBtn.ConnectClicked(func() {
			dialog := gtk.NewFileDialog()
			dialog.SetTitle("Select ISO File")
			dialog.SetModal(true)

			// More reliable way to get home directory when running with elevated permissions
			homeDir, err := getHomeDirectory()
			if err == nil && homeDir != "" {
				// Create a file for the home directory
				gfile := gio.NewFileForPath(homeDir)
				if gfile != nil {
					dialog.SetInitialFolder(gfile)
				}
			}

			// Create and apply filter for ISO files
			filter := gtk.NewFileFilter()
			filter.SetName("ISO files")
			filter.AddPattern("*.iso")
			filter.AddMIMEType("application/x-iso9660-image")
			dialog.SetDefaultFilter(filter)

			dialog.Open(context.Background(), &win.Window, func(res gio.AsyncResulter) {
				file, err := dialog.OpenFinish(res)
				if err == nil && file != nil {
					isoPath = file.Path()
					glib.IdleAdd(func() {
						isoBtn.SetLabel("ISO: " + file.Path())
						if drive != "" {
							burnBtn.SetSensitive(true)
						} else {
							burnBtn.SetSensitive(false)
						}
					})
				}
			})
		})

		drives := ListUSBDrives()
		model := gtk.NewStringList(drives)
		driveDropdown := gtk.NewDropDown(model, nil)
		driveDropdown.SetSelected(0)

		refreshBtn := gtk.NewButtonWithLabel("⟳")
		refreshBtn.SetTooltipText("Refresh USB drives list")
		refreshBtn.SetHAlign(gtk.AlignStart)
		refreshBtn.SetVAlign(gtk.AlignCenter)
		refreshBtn.SetSizeRequest(40, 32)
		refreshBtn.ConnectClicked(func() {
			newDrives := ListUSBDrives()
			model := gtk.NewStringList(newDrives)
			driveDropdown.SetModel(model)
			driveDropdown.SetSelected(0)
			drives = newDrives
			burnBtn.SetSensitive(false)
		})

		driveBox := gtk.NewBox(gtk.OrientationHorizontal, 5)
		driveDropdown.SetHExpand(true) // Make dropdown expand to fill available space
		driveBox.Append(driveDropdown)
		driveBox.Append(refreshBtn)

		driveDropdown.Connect("notify::selected", func() {
			index := int(driveDropdown.Selected())
			if index > 0 && index < len(drives) {
				drive = drives[index]
				if isoPath != "" {
					burnBtn.SetSensitive(true)
				} else {
					burnBtn.SetSensitive(false)
				}
			} else {
				burnBtn.SetSensitive(false)
			}
		})

		// Add image at the top and make it bigger
		logo := gtk.NewImageFromFile(f.Name())
		logo.SetPixelSize(256) // Make the image bigger
		layout := gtk.NewBox(gtk.OrientationVertical, 10)
		layout.SetMarginTop(20)
		layout.SetMarginBottom(20)
		layout.SetMarginStart(20)
		layout.SetMarginEnd(20)
		layout.Append(logo)
		layout.Append(isoBtn)

		// Pass a callback to getDownloadWindow to set isoPath and update isoBtn label
		layout.Append(getDownloadWindow(func(newPath string) {
			glib.IdleAdd(func() {
				isoPath = newPath
				isoBtn.SetLabel("ISO: " + newPath)
				if drive != "" {
					burnBtn.SetSensitive(true)
				} else {
					burnBtn.SetSensitive(false)
				}
			})
		}))

		layout.Append(driveBox)
		layout.Append(burnBtn)

		win.SetChild(layout)
		win.SetVisible(true)
		win.SetDefaultSize(800, 600)
		win.Present() // Bring window to the foreground and give it focus

		// Function to start the burning process
		startBurning := func() {
			content := gtk.NewBox(gtk.OrientationVertical, 20)
			content.SetMarginTop(30)
			content.SetMarginBottom(30)
			content.SetMarginStart(30)
			content.SetMarginEnd(30)
			content.SetHAlign(gtk.AlignCenter)
			content.SetVAlign(gtk.AlignCenter)

			// Add image above progress bar and make it bigger
			logo = gtk.NewImageFromFile(f.Name())
			logo.SetPixelSize(256) // Make the image bigger
			content.Append(logo)

			progress := gtk.NewProgressBar()
			progress.SetVExpand(true)
			progress.SetHExpand(true)
			progress.SetMarginBottom(10)

			status := gtk.NewLabel("Burning...")
			status.SetMarginBottom(10)
			status.SetHAlign(gtk.AlignCenter)

			exitBtn := gtk.NewButtonWithLabel("Exit")
			exitBtn.SetSensitive(false)
			exitBtn.SetHAlign(gtk.AlignCenter)

			content.Append(progress)
			content.Append(status)
			content.Append(exitBtn)

			win.SetChild(content)

			go func() {
				Burn(isoPath, drive, progress, status, exitBtn)
			}()

			exitBtn.ConnectClicked(func() {
				win.Close()
			})
		}

		burnBtn.ConnectClicked(func() {
			// Check if any partitions of the selected device are mounted
			if drive != "" && !strings.HasPrefix(drive, "Select") && !strings.HasPrefix(drive, "No USB") {
				devPath := strings.Fields(drive)[0] // e.g. /dev/sdb
				mounted, err := IsDeviceMounted(devPath)
				if err == nil && len(mounted) > 0 {
					// Create a custom dialog using available widgets
					dialog := gtk.NewDialog()
					dialog.SetTitle("Unmount Partitions")
					dialog.SetTransientFor(&win.Window)
					dialog.SetModal(true)
					//dialog.SetDefaultWidth(400)

					contentArea := dialog.ContentArea()
					box := gtk.NewBox(gtk.OrientationVertical, 10)
					box.SetMarginTop(20)
					box.SetMarginBottom(20)
					box.SetMarginStart(20)
					box.SetMarginEnd(20)

					// Add message
					msg := "Some partitions are mounted:\n" + strings.Join(mounted, "\n")
					msgLabel := gtk.NewLabel(msg)
					msgLabel.SetHAlign(gtk.AlignStart)
					msgLabel.SetWrap(true)
					box.Append(msgLabel)

					// Add question
					questionLabel := gtk.NewLabel("Do you want to unmount them?")
					questionLabel.SetHAlign(gtk.AlignStart)
					questionLabel.SetMarginTop(10)
					box.Append(questionLabel)

					contentArea.Append(box)

					// Create a button box with proper margins and styling
					buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 10)
					buttonBox.SetMarginTop(20)
					buttonBox.SetMarginBottom(20)
					buttonBox.SetMarginStart(20)
					buttonBox.SetMarginEnd(20)
					buttonBox.SetHAlign(gtk.AlignCenter)

					// Create Cancel button
					cancelBtn := gtk.NewButton()
					cancelBtn.SetLabel("Cancel")
					cancelBtn.SetMarginTop(10)
					cancelBtn.SetMarginBottom(10)
					cancelBtn.SetMarginStart(20)
					cancelBtn.SetMarginEnd(20)

					// Create Unmount button with styling
					unmountBtn := gtk.NewButton()
					unmountBtn.SetLabel("Unmount")
					unmountBtn.SetCSSClasses([]string{"suggested-action"})
					unmountBtn.SetMarginTop(10)
					unmountBtn.SetMarginBottom(10)
					unmountBtn.SetMarginStart(20)
					unmountBtn.SetMarginEnd(20)

					buttonBox.Append(cancelBtn)
					buttonBox.Append(unmountBtn)
					contentArea.Append(buttonBox)

					// Show dialog
					dialog.Show()

					// Connect cancel button click
					cancelBtn.ConnectClicked(func() {
						dialog.Destroy()
					})

					// Connect unmount button click
					unmountBtn.ConnectClicked(func() {
						dialog.Destroy()

						// Try to unmount
						err := UnmountDevice(mounted)
						if err != nil {
							errDialog(win.Window, err)
						} else {
							// Continue with burn after successful unmount
							startBurning()
						}
					})
					return
				}
			}

			// Start burning directly if no unmounting is needed
			startBurning()
		})

	})

	app.Run(os.Args)
}

func errDialog(win gtk.Window, err error) {
	// Show error dialog
	errD := gtk.NewDialog()
	errD.SetTitle("Error")
	errD.SetTransientFor(&win)
	errD.SetModal(true)

	contentArea := errD.ContentArea()
	box := gtk.NewBox(gtk.OrientationVertical, 10)
	box.SetMarginTop(20)
	box.SetMarginBottom(20)
	box.SetMarginStart(20)
	box.SetMarginEnd(20)

	// Add warning icon
	icon := gtk.NewImageFromIconName("dialog-error")
	icon.SetPixelSize(48)
	icon.SetMarginBottom(10)
	icon.SetHAlign(gtk.AlignCenter)
	box.Append(icon)

	// Error message
	errMsgLabel := gtk.NewLabel("Failed to unmount: " + err.Error())
	errMsgLabel.SetHAlign(gtk.AlignCenter)
	errMsgLabel.SetWrap(true)
	box.Append(errMsgLabel)

	contentArea.Append(box)

	// Create a button box with proper margins and styling
	buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	buttonBox.SetMarginTop(20)
	buttonBox.SetMarginBottom(20)
	buttonBox.SetMarginStart(20)
	buttonBox.SetMarginEnd(20)
	buttonBox.SetHAlign(gtk.AlignCenter)

	// Create close button with styling
	closeBtn := gtk.NewButton()
	closeBtn.SetLabel("Close")
	closeBtn.SetCSSClasses([]string{"suggested-action"})
	closeBtn.SetMarginTop(10)
	closeBtn.SetMarginBottom(10)
	closeBtn.SetMarginStart(40)
	closeBtn.SetMarginEnd(40)

	buttonBox.Append(closeBtn)
	contentArea.Append(buttonBox)

	// Show dialog
	errD.Show()

	// Connect close button click
	closeBtn.ConnectClicked(func() {
		errD.Destroy()
	})
}
