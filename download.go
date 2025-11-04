package main

import (
	"context"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	isoFileExtension = ".iso"
)

var filteredAssets []ReleaseAsset // Store filtered assets for dropdowns
var saveFilePath string           // Store the path to save the downloaded file

// filterStrings filters a list of strings based on a search query.
// If the query is a valid regex, it uses regex matching; otherwise, it performs a case-insensitive substring search.
func filterStrings(items []string, searchQuery string) []string {
	if searchQuery == "" {
		return items
	}

	var filtered []string
	searchLower := strings.ToLower(searchQuery)

	// Try to compile as regex
	re, err := regexp.Compile(searchLower)
	useRegex := err == nil

	for _, item := range items {
		itemLower := strings.ToLower(item)
		var matches bool

		if useRegex {
			matches = re.MatchString(itemLower)
		} else {
			// Fall back to simple substring search if regex compilation failed
			matches = strings.Contains(itemLower, searchLower)
		}

		if matches {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// isISOFile checks if a filename has the ISO extension
func isISOFile(filename string) bool {
	return strings.HasSuffix(filename, isoFileExtension)
}

func getDownloadWindow(onDownloaded func(string)) *gtk.Button {
	downloadBtn := gtk.NewButtonWithLabel("Download ISOs")
	downloadBtn.ConnectClicked(func() {
		// Open a new window for ISO downloads
		downloadWin := gtk.NewWindow()
		downloadWin.SetTitle("Download ISO")
		downloadWin.SetDefaultSize(800, 600)

		// Create a vertical box for content
		vbox := gtk.NewBox(gtk.OrientationVertical, 10)
		vbox.SetMarginTop(20)
		vbox.SetMarginBottom(20)
		vbox.SetMarginStart(20)
		vbox.SetMarginEnd(20)
		vbox.SetHAlign(gtk.AlignFill)
		vbox.SetVAlign(gtk.AlignFill)
		vbox.SetHExpand(true)
		vbox.SetVExpand(true)
		vbox.SetSizeRequest(-1, -1) // Let vbox expand to fill window

		// Add a loading label and spinner
		loadingLabel := gtk.NewLabel("Loading data...")
		loadingLabel.SetHAlign(gtk.AlignCenter)
		spinner := gtk.NewSpinner()
		spinner.SetHAlign(gtk.AlignCenter)
		spinner.Start()
		vbox.Append(spinner)
		vbox.Append(loadingLabel)

		// Dropdowns for selection
		versionLabel := gtk.NewLabel("Versions:")
		versionLabel.SetHAlign(gtk.AlignStart)
		vbox.Append(versionLabel)

		versionBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
		versionBox.SetHAlign(gtk.AlignFill)
		versionBox.SetVAlign(gtk.AlignCenter)
		versionBox.SetMarginTop(4)
		versionBox.SetMarginBottom(4)
		versionBox.SetMarginStart(0)
		versionBox.SetMarginEnd(0)

		versionSearchEntry := gtk.NewSearchEntry()
		versionSearchEntry.SetPlaceholderText("Search versions... (regex supported)")
		versionSearchEntry.SetHExpand(true)
		versionBox.Append(versionSearchEntry)

		versionDropdown := gtk.NewDropDown(gtk.NewStringList([]string{""}), nil)
		versionDropdown.SetHExpand(true)
		versionDropdown.SetSensitive(false)
		versionBox.Append(versionDropdown)

		vbox.Append(versionBox)

		assetLabel := gtk.NewLabel("Assets:")
		assetLabel.SetHAlign(gtk.AlignStart)
		vbox.Append(assetLabel)

		assetBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
		assetBox.SetHAlign(gtk.AlignFill)
		assetBox.SetVAlign(gtk.AlignCenter)
		assetBox.SetMarginTop(4)
		assetBox.SetMarginBottom(4)
		assetBox.SetMarginStart(0)
		assetBox.SetMarginEnd(0)

		assetSearchEntry := gtk.NewSearchEntry()
		assetSearchEntry.SetPlaceholderText("Search assets... (regex supported)")
		assetSearchEntry.SetHExpand(true)
		assetBox.Append(assetSearchEntry)

		assetDropdown := gtk.NewDropDown(gtk.NewStringList([]string{""}), nil)
		assetDropdown.SetHExpand(true)
		assetDropdown.SetSensitive(false)
		assetBox.Append(assetDropdown)
		vbox.Append(assetBox)

		// Move Download button to bottom, outside assetBox
		buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 10)
		buttonBox.SetHAlign(gtk.AlignEnd)
		buttonBox.SetMarginTop(20)
		assetDownloadBtn := gtk.NewButtonWithLabel("Download")
		assetDownloadBtn.SetHExpand(false)
		assetDownloadBtn.SetVExpand(false)
		assetDownloadBtn.SetSizeRequest(-1, -1) // Default size
		assetDownloadBtn.SetMarginTop(0)
		assetDownloadBtn.SetMarginBottom(0)
		assetDownloadBtn.SetSensitive(true)
		buttonBox.Append(assetDownloadBtn)
		vbox.Append(buttonBox)

		// Move refreshCacheBtn to the bottom of the vbox
		refreshCacheBtn := gtk.NewButtonWithLabel("Refresh Releases")
		refreshCacheBtn.SetHAlign(gtk.AlignCenter)
		refreshCacheBtn.SetMarginTop(10)
		refreshCacheBtn.SetVExpand(false)

		assetDownloadBtn.ConnectClicked(func() {
			selectedIdx := assetDropdown.Selected()
			if selectedIdx < 0 || int(selectedIdx) >= len(filteredAssets) {
				return
			}
			selectedAsset := filteredAssets[selectedIdx]

			fmt.Printf("Downloading asset: %s (ID: %d) for version: %s\n", selectedAsset.Name, selectedAsset.ID, selectedAsset.Version)
			// Here you would implement the actual download logic using selectedAsset.ID
			// Open a file dialog to choose save location
			fileDialog := gtk.NewFileDialog()
			fileDialog.SetTitle("Save ISO File")
			fileDialog.SetAcceptLabel("Save")
			fileDialog.SetModal(true)
			homeDir, err := getHomeDirectory()
			if err == nil && homeDir != "" {
				fileDialog.SetInitialFile(gio.NewFileForPath(filepath.Join(homeDir, selectedAsset.Name)))
			} else {
				fileDialog.SetInitialFile(gio.NewFileForPath(selectedAsset.Name))
			}

			fileDialog.Save(context.Background(), downloadWin, func(res gio.AsyncResulter) {
				file, err := fileDialog.SaveFinish(res)
				if err != nil {
					return
				}
				if file == nil {
					return
				}

				// Remove all children from vbox (Gtk4 doesn't have ForEach, use Remove on each child)
				for {
					child := vbox.FirstChild()
					if child == nil {
						break
					}
					vbox.Remove(child)
				}

				// Create a new box for download progress
				progressBox := gtk.NewBox(gtk.OrientationVertical, 10)
				progressBox.SetHAlign(gtk.AlignCenter)
				progressBox.SetVAlign(gtk.AlignCenter)
				progressBox.SetMarginTop(40)
				progressBox.SetMarginBottom(40)
				progressBox.SetMarginStart(40)
				progressBox.SetMarginEnd(40)

				spinnerDownload := gtk.NewSpinner()
				spinnerDownload.SetHAlign(gtk.AlignCenter)
				spinnerDownload.SetVAlign(gtk.AlignCenter)
				spinnerDownload.Start()
				progressBox.Append(spinnerDownload)

				progress := gtk.NewProgressBar()
				progress.SetVExpand(true)
				progress.SetHExpand(true)
				progress.SetMarginBottom(10)

				progress.SetShowText(true) // Set a fixed height for the progress bar
				progressBox.Append(progress)

				assetDownloadText := fmt.Sprintf("Downloading asset %s", selectedAsset.Name)
				downloadLabel := gtk.NewLabel(assetDownloadText)
				downloadLabel.SetHAlign(gtk.AlignCenter)
				progressBox.Append(downloadLabel)

				vbox.Append(progressBox)

				// Run download in a goroutine so the dialog closes immediately
				go func() {
					resp, err := http.Get(selectedAsset.URL)
					if err != nil {
						glib.IdleAdd(func() {
							spinnerDownload.Stop()
							downloadLabel.SetText("Failed to download asset: " + err.Error())
							progressBox.Append(goBackButton(downloadWin))
						})
						return
					}

					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						glib.IdleAdd(func() {
							spinnerDownload.Stop()
							downloadLabel.SetText("Failed to download asset: " + resp.Status)
							progressBox.Append(goBackButton(downloadWin))
						})
						return
					}

					// Check for redirects
					redirect := resp.Header.Get("Location")
					if redirect != "" && redirect != selectedAsset.URL {
						glib.IdleAdd(func() {
							spinnerDownload.Stop()
							downloadLabel.SetText("Redirected to another URL: " + redirect)
							progressBox.Append(goBackButton(downloadWin))
						})
						return
					}

					// get the size
					contentLength := resp.ContentLength
					// Read the response body
					read := resp.Body

					if err != nil {
						glib.IdleAdd(func() {
							spinnerDownload.Stop()
							downloadLabel.SetText("Failed to download asset")
							progress.SetText("Error: " + err.Error())
							progressBox.Append(goBackButton(downloadWin))
						})
						return
					}
					defer read.Close()
					if redirect != "" {
						glib.IdleAdd(func() {
							spinnerDownload.Stop()
							downloadLabel.SetText("Redirected to another URL")
							progress.SetText("Redirect: " + redirect)
							progressBox.Append(goBackButton(downloadWin))
						})
						return
					}

					// now write the file
					fileWriter, err := os.Create(file.Path())
					if err != nil {
						glib.IdleAdd(func() {
							spinnerDownload.Stop()
							downloadLabel.SetText("Failed to create file")
							progress.SetText("Error: " + err.Error())
							progressBox.Append(goBackButton(downloadWin))
						})
						return
					}
					defer fileWriter.Close()

					// copy from read to fileWriter with progress
					progress.SetFraction(0)
					progress.SetText("Downloading...")
					buf := make([]byte, 32*1024) // 32KB buffer
					totalBytes := int64(0)
					for {
						n, err := read.Read(buf)
						if n > 0 {
							// Write to file
							if _, err := fileWriter.Write(buf[:n]); err != nil {
								glib.IdleAdd(func() {
									spinnerDownload.Stop()
									downloadLabel.SetText("Failed to write file")
									progress.SetText("Error: " + err.Error())
									progressBox.Append(goBackButton(downloadWin))
								})
								return
							}
							totalBytes += int64(n)
							glib.IdleAdd(func() {
								progress.SetFraction(float64(totalBytes) / float64(contentLength))
								// set the downloaded size in Mb
								totalBytesMb := totalBytes / (1024 * 1024)
								// Set the final image size in Mb
								contentLengthMb := contentLength / (1024 * 1024)
								// Update the text with the current download size and total size
								progress.SetText(fmt.Sprintf("Downloading... %d/%d MB", totalBytesMb, contentLengthMb))
							})
						}
						if err != nil {
							if err == io.EOF {
								break // Download complete
							}
							glib.IdleAdd(func() {
								spinnerDownload.Stop()
								downloadLabel.SetText("Failed to read data")
								progress.SetText("Error: " + err.Error())
								progressBox.Append(goBackButton(downloadWin))
							})
							return
						}
					}

					glib.IdleAdd(func() {
						spinnerDownload.Stop()
						downloadLabel.SetText("Download complete!")
						progress.SetText("Done!")

						// Show the full path of the ISO file
						filePathLabel := gtk.NewLabel(fmt.Sprintf("ISO saved to: %s", file.Path()))
						filePathLabel.SetHAlign(gtk.AlignCenter)
						filePathLabel.SetMarginTop(20)
						progressBox.Append(filePathLabel)
						progressBox.Append(goBackButton(downloadWin))

						// Call the callback to set the ISO path in the main window
						onDownloaded(file.Path())
					})
				}()
			})
		})

		// Helper to update dropdowns after fetching assets
		var releaseAssets []ReleaseAsset // Store assets for dropdown logic
		var lastAssetList []string       // Keep last asset list for filtering

		// Helper function to update filteredAssets based on displayed asset names
		updateFilteredAssets := func(displayedNames []string, allAssets []ReleaseAsset) {
			filteredAssets = nil
			nameSet := make(map[string]bool)
			for _, name := range displayedNames {
				nameSet[name] = true
			}
			for _, asset := range allAssets {
				if nameSet[asset.Name] {
					filteredAssets = append(filteredAssets, asset)
				}
			}
		}

		// Update lastVersionList after fetching versions
		updateReleaseDropdowns := func(assets []ReleaseAsset, err error) {
			spinner.Stop()
			if err != nil || len(assets) == 0 {
				loadingLabel.SetText("Failed to load releases or no assets found.")
				return
			}
			loadingLabel.SetText("")
			releaseAssets = assets // Save for later use
			versionSet := make(map[string]struct{})
			for _, a := range assets {
				versionSet[a.Version] = struct{}{}
			}
			// Sort versions by semver descending
			var semverVersions []*semver.Version
			versionMap := make(map[string]*semver.Version)
			for v := range versionSet {
				ver, err := semver.NewVersion(v)
				if err == nil {
					semverVersions = append(semverVersions, ver)
					versionMap[v] = ver
				}
			}
			sort.Sort(sort.Reverse(semver.Collection(semverVersions)))
			versions := make([]string, 0, len(semverVersions))
			for _, ver := range semverVersions {
				versions = append(versions, ver.Original())
			}
			lastVersionList = versions // Save for filtering
			versionDropdown.SetModel(gtk.NewStringList(versions))
			versionDropdown.SetSensitive(true)
			if len(versions) > 0 {
				versionDropdown.SetSelected(0)
				latestVersion := versions[0]
				filteredAssets = nil
				var assetList []string
				for _, a := range assets {
					if a.Version == latestVersion && isISOFile(a.Name) {
						assetList = append(assetList, a.Name)
						filteredAssets = append(filteredAssets, a)
					}
				}
				assetDropdown.SetModel(gtk.NewStringList(assetList))
				assetDropdown.SetSensitive(true)
				// Set the number in the label
				versionLabel.SetText(fmt.Sprintf("Versions (%d):", len(versions)))
			}
		}

		// Update asset dropdown when version changes
		versionDropdown.Connect("notify::selected", func() {
			selectedObj := versionDropdown.Model().Item(versionDropdown.Selected())
			selectedStr, ok := selectedObj.Cast().(*gtk.StringObject)
			if !ok {
				return
			}
			selectedVersion := selectedStr.String()

			// Get all assets for the selected version
			var versionAssets []ReleaseAsset
			var assetList []string
			for _, a := range releaseAssets {
				if a.Version == selectedVersion && isISOFile(a.Name) {
					assetList = append(assetList, a.Name)
					versionAssets = append(versionAssets, a)
				}
			}
			lastAssetList = assetList

			// Apply search filter if any
			search := assetSearchEntry.Text()
			filtered := filterStrings(assetList, search)

			// Update filteredAssets to match the displayed filtered list
			updateFilteredAssets(filtered, versionAssets)

			if len(filtered) == 0 {
				assetDropdown.SetModel(gtk.NewStringList([]string{"No assets available"}))
				assetDropdown.SetSensitive(false)
			} else {
				assetDropdown.SetModel(gtk.NewStringList(filtered))
				assetDropdown.SetSensitive(true)
			}
			// Update asset label with count
			assetLabel.SetText(fmt.Sprintf("Assets (%d):", len(filtered)))
		})

		// Filter asset dropdown on search entry change
		assetSearchEntry.Connect("search-changed", func() {
			search := assetSearchEntry.Text()
			filtered := filterStrings(lastAssetList, search)

			// Get all assets for currently selected version to update filteredAssets
			selectedObj := versionDropdown.Model().Item(versionDropdown.Selected())
			if selectedObj != nil {
				selectedStr, ok := selectedObj.Cast().(*gtk.StringObject)
				if ok {
					selectedVersion := selectedStr.String()
					var versionAssets []ReleaseAsset
					for _, a := range releaseAssets {
						if a.Version == selectedVersion && isISOFile(a.Name) {
							versionAssets = append(versionAssets, a)
						}
					}
					updateFilteredAssets(filtered, versionAssets)
				}
			}

			if len(filtered) == 0 {
				assetDropdown.SetModel(gtk.NewStringList([]string{"No assets available"}))
				assetDropdown.SetSensitive(false)
			} else {
				assetDropdown.SetModel(gtk.NewStringList(filtered))
				assetDropdown.SetSensitive(true)
			}
			// Update asset label with count
			assetLabel.SetText(fmt.Sprintf("Assets (%d):", len(filtered)))
		})

		// Filter version dropdown on search entry change
		// connect to search-changed with adds a delay otherwise the trigger is instant
		versionSearchEntry.Connect("search-changed", func() {
			search := versionSearchEntry.Text()
			filtered := filterStrings(lastVersionList, search)

			if len(filtered) == 0 {
				versionDropdown.SetModel(gtk.NewStringList([]string{"No versions found"}))
				versionDropdown.SetSensitive(false)
			} else {
				versionDropdown.SetModel(gtk.NewStringList(filtered))
				versionDropdown.SetSensitive(true)
				versionDropdown.SetSelected(0)
			}
		})

		// Remove previous vbox.Append(refreshCacheBtn)
		// Instead, add a spacer and then append the button
		spacer := gtk.NewBox(gtk.OrientationVertical, 0)
		spacer.SetVExpand(true)
		vbox.Append(spacer)
		vbox.Append(refreshCacheBtn)

		// Helper to set button sensitivity during fetch
		setRefreshBtnActive := func(active bool) {
			refreshCacheBtn.SetSensitive(active)
		}

		refreshCacheBtn.ConnectClicked(func() {
			setRefreshBtnActive(false)
			spinner.Start()
			loadingLabel.SetText("Refreshing releases...")
			versionDropdown.SetSensitive(false)
			assetDropdown.SetSensitive(false)
			go func() {
				ctx := context.Background()
				cacheFile := filepath.Join(os.TempDir(), "kairos_releases_cache.json")
				_ = os.Remove(cacheFile)
				assets, err := GetCachedReleaseAssets(ctx, "kairos-io", "kairos")
				glib.IdleAdd(func() {
					updateReleaseDropdowns(assets, err)
					setRefreshBtnActive(true)
				})
			}()
		})

		// Start fetching release assets in background
		setRefreshBtnActive(false)
		go func() {
			ctx := context.Background()
			assets, err := GetCachedReleaseAssets(ctx, "kairos-io", "kairos")
			glib.IdleAdd(func() {
				updateReleaseDropdowns(assets, err)
				setRefreshBtnActive(true)
			})
		}()
		downloadWin.SetChild(vbox)
		downloadWin.SetVisible(true)
	})

	return downloadBtn
}

func goBackButton(window *gtk.Window) *gtk.Button {
	goBackBtn := gtk.NewButtonWithLabel("Go Back")
	goBackBtn.SetHAlign(gtk.AlignCenter)
	goBackBtn.SetMarginTop(20)

	goBackBtn.ConnectClicked(func() {
		window.Close()
	})
	return goBackBtn
}
