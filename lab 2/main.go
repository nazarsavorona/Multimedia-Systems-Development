package main

import (
	"fmt"
	vlc "github.com/adrg/libvlc-go/v3"
	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"log"
	"math"
	"os"
	"time"
)

const (
	appID        = "com.github.media-player"
	defaultTitle = "Media Player"
)

func builderGetObject(builder *gtk.Builder, name string) glib.IObject {
	obj, _ := builder.GetObject(name)
	return obj
}

func assertErr(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func assertConv(ok bool) {
	if !ok {
		log.Panic("invalid widget conversion")
	}
}

func playerReleaseMedia(player *vlc.Player) {
	player.Stop()
	if media, _ := player.Media(); media != nil {
		media.Release()
	}
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%02d:%02d:%02d", h, m, int(d.Seconds())%60)
}

func main() {
	// Initialize libVLC module
	err := vlc.Init("--quiet", "--no-xlib")
	assertErr(err)

	// Create a new player
	player, err := vlc.NewPlayer()
	assertErr(err)

	// Create new GTK application
	app, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	assertErr(err)

	var scale *gtk.Scale

	app.Connect("activate", func() {
		// Load application layout
		builder, err := gtk.BuilderNewFromFile("layout.glade")
		assertErr(err)

		// Get application window
		appWin, ok := builderGetObject(builder, "appWindow").(*gtk.ApplicationWindow)
		assertConv(ok)

		// Get play button
		playButton, ok := builderGetObject(builder, "playButton").(*gtk.Button)
		assertConv(ok)

		scale, ok = builderGetObject(builder, "slider").(*gtk.Scale)
		assertConv(ok)

		scale.SetRange(0, 1)

		timeLabel, ok := builderGetObject(builder, "timeLabel").(*gtk.Label)
		assertConv(ok)

		volume, ok := builderGetObject(builder, "volume").(*gtk.Adjustment)
		assertConv(ok)

		// stream dialog
		streamDialog, ok := builderGetObject(builder, "streamDialog").(*gtk.Dialog)
		assertConv(ok)

		streamEntry, ok := builderGetObject(builder, "streamEntry").(*gtk.Entry)
		assertConv(ok)

		go func() {
			for true {
				for player.IsPlaying() {
					position, _ := player.MediaPosition()

					scale.SetValue(float64(position))

					time.Sleep(10 * time.Millisecond)
				}
			}
		}()

		// Add builder signal handlers
		signals := map[string]interface{}{
			"onRealizePlayerArea": func(playerArea *gtk.DrawingArea) {
				// Set window for the player
				playerWindow, err := playerArea.GetWindow()
				assertErr(err)
				err = setPlayerWindow(player, playerWindow)
				assertErr(err)
			},
			"onDrawPlayerArea": func(playerArea *gtk.DrawingArea, cr *cairo.Context) {
				cr.SetSourceRGB(0, 0, 0)
				cr.Paint()
			},
			"onActivateOpenFile": func() {
				fileDialog, err := gtk.FileChooserDialogNewWith2Buttons(
					"Choose file...",
					appWin, gtk.FILE_CHOOSER_ACTION_OPEN,
					"Cancel", gtk.RESPONSE_DELETE_EVENT,
					"Open", gtk.RESPONSE_ACCEPT)
				assertErr(err)
				defer fileDialog.Destroy()

				fileFilter, err := gtk.FileFilterNew()
				assertErr(err)
				fileFilter.SetName("Media files")
				fileFilter.AddPattern("*.mp4")
				fileFilter.AddPattern("*.avi")
				fileFilter.AddPattern("*.mpg")
				fileFilter.AddPattern("*.wmv")

				fileFilter.AddPattern("*.mp3")
				fileFilter.AddPattern("*.wav")
				fileFilter.AddPattern("*.flac")
				fileFilter.AddPattern("*.ogg")
				fileFilter.AddPattern("*.wav")
				fileDialog.AddFilter(fileFilter)

				if result := fileDialog.Run(); result == gtk.RESPONSE_ACCEPT {
					// Release current media, if any
					playerReleaseMedia(player)

					// Get selected filename
					filename := fileDialog.GetFilename()

					// Load media and start playback
					if _, err := player.LoadMediaFromPath(filename); err != nil {
						log.Printf("Cannot load selected media: %s\n", err)
						return
					}

					player.Play()
					playButton.SetLabel("gtk-media-pause")
					appWin.SetTitle(filename)
				}
			},
			"onActivateQuit": func() {
				app.Quit()
			},
			"onClickPlayButton": func(playButton *gtk.Button) {
				if media, _ := player.Media(); media == nil {
					return
				}

				if player.IsPlaying() {
					player.SetPause(true)
					playButton.SetLabel("gtk-media-play")
				} else {
					player.Play()
					playButton.SetLabel("gtk-media-pause")
				}
			},
			"onClickStopButton": func(stopButton *gtk.Button) {
				player.Stop()
				playButton.SetLabel("gtk-media-play")
				scale.SetValue(0)
				appWin.SetTitle(defaultTitle)
			},
			"onClickBackwardButton": func() {
				position, _ := player.MediaPosition()

				player.SetMediaPosition(float32(math.Max(float64(position-0.1), 0)))
			},
			"onClickForwardButton": func() {
				position, _ := player.MediaPosition()

				player.SetMediaPosition(float32(math.Min(float64(position+0.1), 1)))
			},
			"onChangeSlider": func(scale *gtk.Scale) {
				player.SetMediaPosition(float32(scale.GetValue()))
			},
			"onSliderValueChange": func() {
				mediaTime, _ := player.MediaTime()

				t := time.Duration(mediaTime) * time.Millisecond
				timeLabel.SetLabel(fmtDuration(t))
			},
			"onSliderPress": func() {
				player.SetPause(true)
			},
			"onSliderRelease": func() {
				player.SetPause(false)
			},
			"onVolumeChanged": func() {
				player.SetVolume(int(volume.GetValue()))
			},
			"onActivateStreamDialog": func() {
				streamDialog.Show()
				streamEntry.GrabFocus()
			},
			"onPlayStreamButtonClicked": func() {
				url, _ := streamEntry.GetText()
				streamEntry.SetText("")
				streamDialog.Hide()
				appWin.SetTitle(url)

				if _, err := player.LoadMediaFromURL(url); err != nil {
					log.Printf("Cannot load selected media: %s\n", err)
					return
				}

				player.Play()
				playButton.SetLabel("gtk-media-pause")
			},
			"onCloseStreamClicked": func() {
				streamDialog.Hide()
			},
		}
		builder.ConnectSignals(signals)

		appWin.ShowAll()
		app.AddWindow(appWin)
	})

	// Cleanup on exit
	app.Connect("shutdown", func() {
		playerReleaseMedia(player)
		player.Release()
		vlc.Release()
	})

	// Launch the application
	os.Exit(app.Run(os.Args))
}
