// Command yoshi-menubar is a small macOS status-bar helper for Yoshi
// Pi-hole. It runs in the user's GUI session (unlike the DNS daemon, which
// runs as a root LaunchDaemon with no GUI access) and shows whether blocking
// is currently active, with quick controls mirroring the dashboard's bedtime
// mode.
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/getlantern/systray"

	"yoshi-pihole/internal/trayicon"
)

const (
	apiBase    = "http://127.0.0.1:8080"
	pollPeriod = 5 * time.Second
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTemplateIcon(trayicon.Active(), trayicon.Active())
	systray.SetTooltip("Yoshi Pi-hole")

	mOpen := systray.AddMenuItem("Apri Dashboard", "Apri la dashboard di Yoshi Pi-hole")
	systray.AddSeparator()
	mDisable30s := systray.AddMenuItem("Disabilita 30 secondi", "")
	mDisable5m := systray.AddMenuItem("Disabilita 5 minuti", "")
	mDisable30m := systray.AddMenuItem("Disabilita 30 minuti", "")
	mEnable := systray.AddMenuItem("Riabilita blocco", "")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Nascondi icona", "Chiude solo questa icona: il blocco DNS resta attivo")

	go pollStatusLoop()

	for {
		select {
		case <-mOpen.ClickedCh:
			_ = exec.Command("open", apiBase).Start()
		case <-mDisable30s.ClickedCh:
			setBlocking(false, 30)
		case <-mDisable5m.ClickedCh:
			setBlocking(false, 300)
		case <-mDisable30m.ClickedCh:
			setBlocking(false, 1800)
		case <-mEnable.ClickedCh:
			setBlocking(true, 0)
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func onExit() {}

func pollStatusLoop() {
	for {
		refreshIcon()
		time.Sleep(pollPeriod)
	}
}

func refreshIcon() {
	resp, err := http.Get(apiBase + "/api/dns/blocking")
	if err != nil {
		systray.SetTooltip("Yoshi Pi-hole (servizio non raggiungibile)")
		return
	}
	defer resp.Body.Close()

	var data struct {
		Blocking bool `json:"blocking"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}

	if data.Blocking {
		systray.SetTemplateIcon(trayicon.Active(), trayicon.Active())
		systray.SetTooltip("Yoshi Pi-hole — blocco attivo")
	} else {
		systray.SetTemplateIcon(trayicon.Paused(), trayicon.Paused())
		systray.SetTooltip("Yoshi Pi-hole — blocco disattivato")
	}
}

func setBlocking(blocking bool, timerSeconds int) {
	body, _ := json.Marshal(map[string]any{"blocking": blocking, "timer": timerSeconds})
	resp, err := http.Post(apiBase+"/api/dns/blocking", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("yoshi-menubar: impossibile contattare il servizio: %v", err)
		return
	}
	resp.Body.Close()
	refreshIcon()
}
