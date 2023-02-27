package chat

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/johnthethird/thresher/network"
	"github.com/johnthethird/thresher/user"
	"github.com/johnthethird/thresher/utils"
	"github.com/johnthethird/thresher/version"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var	inputWidth = 70

// A structure that represents the ChatRoom UI
type UI struct {
	*ChatRoom
	*TermApp

	net network.Network

	MsgInputs chan string
	CmdInputs chan UICommand
}

// A structure that represents the tview application
type TermApp struct {
	TerminalApp *tview.Application
	pages *tview.Pages
	participantBox *tview.TextView
	keyBox *tview.TextView
	messageBox *tview.TextView
	helpBox *tview.TextView
	inputBox *tview.InputField
}

// A structure that represents a UI command (i.e. /somecommand)
type UICommand struct {
	cmdtype string
	cmdargs  []string
}

// Create a new tview application
func NewTerminalApp(blockchain string, roomname string, nick string, cmdchan chan UICommand, msgchan chan string) *TermApp {
	app := tview.NewApplication()
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Clear()
		return false
	})

	titleleftbox := tview.NewTextView().
		SetText(fmt.Sprintf("STT Wallet  (v%s)", version.Version)).
		SetTextColor(tcell.ColorWhite).
		SetTextAlign(tview.AlignLeft)

	titlerightbox := tview.NewTextView().
		SetText(blockchain).
		SetTextColor(tcell.ColorWhite).
		SetTextAlign(tview.AlignRight)

	titlebox := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(titleleftbox, 0, 1, false).
		AddItem(titlerightbox, 0,1,false)

	titlebox.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen)

	messagebox := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	messagebox.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle(fmt.Sprintf("ChatRoom-%s", roomname)).
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite)

	participantbox := tview.NewTextView().SetDynamicColors(true)
	participantbox.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Online Participants").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite)

	keybox := tview.NewTextView().SetDynamicColors(true)
	keybox.
		SetBorder(true).
		SetBorderColor(tcell.ColorYellow).
		SetTitle("Available Wallets").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite)

	helpbox := tview.NewTextView().SetDynamicColors(true)
	fmt.Fprintf(helpbox, "  [grey]Available Commands:[-] [yellow]F2[-] [grey]Generate new mpc wallet[-]  [yellow]F3[-]  [grey]Send Transaction[-]")

	input := tview.NewInputField().
		SetLabel(nick + " > ").
		SetLabelColor(tcell.ColorGreen).
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack)

	input.SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Input").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite).
		SetBorderPadding(0, 0, 1, 0)

	// Define functionality when the input recieves a done signal (enter/tab)
	input.SetDoneFunc(func(key tcell.Key) {
		// Check if trigger was caused by a Return(Enter) press.
		if key != tcell.KeyEnter {
			return
		}

		// Read the input text
		line := input.GetText()

		// Check if there is any input text. No point printing empty messages
		if len(line) == 0 {
			return
		}

		// Check for command inputs
		if strings.HasPrefix(line, "/") {
			// Split the command
			cmdparts := strings.Split(line, " ")

			// Add a nil arg if there is no argument
			if len(cmdparts) == 1 {
				cmdparts = append(cmdparts, "")
			}

			// Send as a command
			cmdchan <- UICommand{cmdtype: cmdparts[0], cmdargs: cmdparts[1:]}

		} else {
			// Send as a chat message
			msgchan <- line
		}

		// Reset the input field
		input.SetText("")
	})

	topflex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(keybox, 0, 1, false).
		AddItem(participantbox, 0, 1, false)

	middleflex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topflex, 0, 1, false).
		AddItem(messagebox, 0, 1, false)

	// Create a flexbox to fit all the widgets
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titlebox, 3, 1, false).
		AddItem(middleflex, 0, 1, false).
		AddItem(input, 3, 1, true).
		AddItem(helpbox, 1, 1, false)

	pages := tview.NewPages().AddAndSwitchToPage("main", flex, true)

	// Set the flex as the app root
	app.SetRoot(pages, true)

	return &TermApp{
		TerminalApp:    app,
		pages:          pages,
		participantBox: participantbox,
		keyBox:         keybox,
		messageBox:     messagebox,
		helpBox:        helpbox,
		inputBox:       input,
	}
}

// Create a new Chatroom UI
func NewUI(cr *ChatRoom, net network.Network) *UI {
	// Initialize the command and message input channels
	cmdchan := make(chan UICommand)
	msgchan := make(chan string)

	blockchain := fmt.Sprintf("%s [%s]", cr.cfg.Blockchain, cr.cfg.Network)
	app := NewTerminalApp(blockchain, cr.cfg.Project, cr.cfg.Me.Nick, cmdchan, msgchan)

	ui := &UI{
		ChatRoom:  cr,
		TermApp:   app,
		net:       net,
		MsgInputs: msgchan,
		CmdInputs: cmdchan,
 	}

	return ui
}

func (ui *UI) Run() error {
	ui.TerminalApp.SetInputCapture(ui.globalKeyboardIntercept)
	ui.fetchWalletBalances()
	go ui.startEventHandler()
	defer ui.Close()
	return ui.TerminalApp.Run()
}

func (ui *UI) Close() {
	ui.pscancel()
}

func (ui *UI) generateKeyForm() {
	participants := ui.ParticipantList()

	if len(participants) == 0 {
		ui.message("No participants are online and available", "OK", "main", nil)
		return
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle("Generate MPC Wallet")
	form.SetTitleAlign(tview.AlignLeft)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			ui.pages.RemovePage("form").ShowPage("main")
		}
		return event
	})

	form.AddInputField("Wallet Name", "", inputWidth, nil, nil)
	form.AddInputField("Maximum amount of parties corrupted", "", inputWidth, nil, nil)

	form.AddCheckbox(ui.cfg.Me.Nick, true, nil)
	for _, p := range participants {
		form.AddCheckbox(p.Nick, false, nil)
	}

	form.AddButton("Generate", func() {
		name := form.GetFormItemByLabel("Wallet Name").(*tview.InputField).GetText()

		thresholdstr := form.GetFormItemByLabel("Maximum amount of parties corrupted").(*tview.InputField).GetText()
		var threshold int
		fmt.Sscan(thresholdstr, &threshold)

		// Always include ourselves
		signers := []user.User{ui.cfg.Me.User}
		for _, p := range participants {
			cb := form.GetFormItemByLabel(p.Nick).(*tview.Checkbox)
			if cb.IsChecked() {
				signers = append(signers, p.User)
			}
		}

		if (threshold <= 0) || (threshold > len(signers) - 1) {
			ui.message("Threshold must be less than total signers", "OK", "main", nil)
			return
		}
		
		go ui.generateKey(name, threshold, signers)
		ui.pages.RemovePage("form").ShowPage("main")
	})

	form.AddButton("Cancel", func() {
		ui.pages.RemovePage("form").ShowPage("main")
	})

	ui.pages.AddAndSwitchToPage("form", ui.modal(form, 80, 29), true).ShowPage("main")
}

func (ui *UI) generateKey(keyname string, threshold int, signers []user.User) {
	othernicks := []string{}
	for _, s := range signers {
		if s.Nick != ui.cfg.Me.Nick {
			othernicks = append(othernicks, s.Nick)
		}
	}
	ui.MsgInputs <- fmt.Sprintf("%s is proposing %v-of-%v wallet with other signers %s", ui.cfg.Me.Nick, threshold+1, len(signers), strings.Join(othernicks, ","))

	ui.OutboundChat <- chatmessage{
		Type:       messageTypeStartKeygen,
		SenderName: ui.cfg.Me.Nick,
		StartKeygen: startkeygencmd{
			Name: keyname,
			Threshold: threshold,
			Signers: signers,
		},
	}

	ui.runProtocolKeygen(keyname, threshold, signers)
}

func (ui *UI) signMsgForm() {
	participants := ui.ParticipantList()

	if len(participants) == 0 {
		ui.message("No participants are online and available", "OK", "main", nil)
		return
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle("Sign Text Message")
	form.SetTitleAlign(tview.AlignLeft)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			ui.pages.RemovePage("form").ShowPage("main")
		}
		return event
	})

	form.AddInputField("Wallet Name", "", inputWidth, nil, nil)
	form.AddInputField("Message", "", inputWidth, nil, nil)

	form.AddCheckbox(ui.cfg.Me.Nick, true, nil)
	for _, p := range participants {
		form.AddCheckbox(p.Nick, false, nil)
	}

	form.AddButton("Sign", func() {
		keyname := form.GetFormItemByLabel("Wallet Name").(*tview.InputField).GetText()
		message := form.GetFormItemByLabel("Message").(*tview.InputField).GetText()
		msk := ui.cfg.FindWallet(keyname)
		// Always include ourselves
		signers := []user.User{ui.cfg.Me.User}
		for _, p := range participants {
			cb := form.GetFormItemByLabel(p.Nick).(*tview.Checkbox)
			if cb.IsChecked() {
				signers = append(signers, p.User)
			}
		}

		if (len(signers) <= msk.Threshold) {
			ui.message(fmt.Sprintf("Wallet threshold requires at least %v signers", msk.Threshold+1), "OK", "main", nil)
			return
		}
		
		go ui.signMsg(keyname, message, signers)

		ui.pages.RemovePage("form").ShowPage("main")
	})

	form.AddButton("Cancel", func() {
		ui.pages.RemovePage("form").ShowPage("main")
	})

	ui.pages.AddAndSwitchToPage("form", ui.modal(form, 80, 29), true).ShowPage("main")
}

func (ui *UI) signMsg(keyname string, message string, signers []user.User) {
	othernicks := []string{}
	for _, s := range signers {
		if s.Nick != ui.cfg.Me.Nick {
			othernicks = append(othernicks, s.Nick)
		}
	}
	ui.MsgInputs <- fmt.Sprintf("%s wants %s to sign a message with wallet %s: '%s' ", ui.cfg.Me.Nick, strings.Join(othernicks, ","), keyname, message)

	ui.OutboundChat <- chatmessage{
		Type:       messageTypeStartSign,
		SenderName: ui.cfg.Me.Nick,
		StartSign: startsigncmd{
			Name: keyname,
			Message: message,
			Signers: signers,
		},
	}

	msghash := utils.DigestAvaMsg(message)
	avasig := ui.runProtocolSign(keyname, msghash, signers)
	avasigcb58, _ := formatting.EncodeWithChecksum(formatting.CB58, avasig)
	ui.MsgInputs <- "Message sucessfully signed!"
	ui.MsgInputs <- fmt.Sprintf("Sig(cb58): %s", avasigcb58)
}

// Show the Send a TX form UI
func (ui *UI) sendTxForm() {
	participants := ui.ParticipantList()

	if len(participants) == 0 {
		ui.message("No participants are online and available", "OK", "main", nil)
		return
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle("Sign and Send Transaction")
	form.SetTitleAlign(tview.AlignLeft)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			ui.pages.RemovePage("form").ShowPage("main")
		}
		return event
	})

	form.AddInputField("Wallet Name", "", inputWidth, nil, nil)
	form.AddInputField("Dest Addr", "", inputWidth, nil, nil)
	form.AddInputField("Amount", "", inputWidth, nil, nil)
	form.AddInputField("Memo", "", inputWidth, nil, nil)

	form.AddCheckbox(ui.cfg.Me.Nick, true, nil)
	for _, p := range participants {
		form.AddCheckbox(p.Nick, false, nil)
	}

	form.AddButton("Sign and Send", func() {
		walletname  := form.GetFormItemByLabel("Wallet Name").(*tview.InputField).GetText()
		destaddr := form.GetFormItemByLabel("Dest Addr").(*tview.InputField).GetText()
		amount   := form.GetFormItemByLabel("Amount").(*tview.InputField).GetText()
		memo     := form.GetFormItemByLabel("Memo").(*tview.InputField).GetText()

		w := ui.cfg.FindWallet(walletname)
		// Always include ourselves
		signers := []user.User{ui.cfg.Me.User}
		for _, p := range participants {
			cb := form.GetFormItemByLabel(p.Nick).(*tview.Checkbox)
			if cb.IsChecked() {
				signers = append(signers, p.User)
			}
		}

		if (len(signers) <= w.Threshold) {
			ui.message(fmt.Sprintf("Wallet threshold requires at least %v signers", w.Threshold+1), "OK", "main", nil)
			return
		}

		othernicks := []string{}
		for _, s := range signers {
			if s.Nick != ui.cfg.Me.Nick {
				othernicks = append(othernicks, s.Nick)
			}
		}
		
		var amt float64
		_, err := fmt.Sscan(amount, &amt)
		if err != nil {
			ui.message(fmt.Sprintf("Error scanning amount %s into float64: %v", amount, err), "OK", "main", nil)
			return
		}

		ui.MsgInputs <- fmt.Sprintf("%s wants %s to send %f AVAX from wallet %s to destination address %s", ui.cfg.Me.Nick, strings.Join(othernicks, ","), amt, walletname, destaddr)

		amt64 := uint64(amt * float64(units.Avax))

		go ui.sendTx(walletname, destaddr, amt64, memo, signers)

		ui.pages.RemovePage("form").ShowPage("main")
	})

	form.AddButton("Cancel", func() {
		ui.pages.RemovePage("form").ShowPage("main")
	})

	ui.pages.AddAndSwitchToPage("form", ui.modal(form, 80, 29), true).ShowPage("main")
}

// Construct a Tx, and run the SendTx multi-party protocol. 
// Also notifies other signers to contruct the Tx and start the protocol.
// TODO support other assetids besides AVAX duh
func (ui *UI) sendTx(walletname string, destaddr string, amount uint64, memo string, signers []user.User) {
	othernicks := []string{}
	for _, s := range signers {
		if s.Nick != ui.cfg.Me.Nick {
			othernicks = append(othernicks, s.Nick)
		}
	}

	// TODO is division the best way to do this?
	amtDisplay := float64(amount) / float64(units.Avax)

	ui.MsgInputs <- fmt.Sprintf("%s wants %s to send %f AVAX from wallet %s to destination address %s", ui.cfg.Me.Nick, strings.Join(othernicks, ","), amtDisplay, walletname, destaddr)

	ui.OutboundChat <- chatmessage{
		Type:       messageTypeStartSendTx,
		SenderName: ui.cfg.Me.Nick,
		StartSendTx: startsendtxcmd{
			Name: walletname,
			Amount: amount,
			DestAddr: destaddr,
			Memo: memo,
			Signers: signers,
		},
	}
	ui.runProtocolSendTx(walletname, destaddr, amount, memo, signers)
}

// Popup modal message UI
func (ui *UI) message(message, doneLabel, page string, doneFunc func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{doneLabel}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("modal").ShowPage(page)

			if buttonLabel == doneLabel && doneFunc != nil {
				doneFunc()
			}
		})

	ui.pages.AddAndSwitchToPage("modal", ui.modal(modal, 80, 29), true).ShowPage("main")
}

// Popup modal confirmation UI
func (ui *UI) confirm(message, doneLabel, page string, doneFunc func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{doneLabel, "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("modal").ShowPage(page)
			if buttonLabel == doneLabel && doneFunc != nil {
				doneFunc()
			}
		})

	ui.pages.AddAndSwitchToPage("modal", ui.modal(modal, 80, 29), true).ShowPage("main")
}

// This is the main UI loop. Listen to several channels and process messages
func (ui *UI) startEventHandler() {
	refreshticker := time.NewTicker(time.Second)
	defer refreshticker.Stop()

	fetchticker := time.NewTicker(time.Second*30)
	defer fetchticker.Stop()

	for {
		select {

		case msg := <-ui.MsgInputs:
			log.Printf("MsgInputs: %+v", msg)
			ui.OutboundChat <- chatmessage{Type: messageTypeChatMessage, SenderName: ui.cfg.Me.Nick, UserMessage: msg}
			ui.displaySelfMessage(msg)

		case cmd := <-ui.CmdInputs:
			log.Printf("CmdInputs: %+v", cmd)
			go ui.handleCommand(cmd)

		case msg := <-ui.InboundChat:
			ui.displayChatMessage(msg)

		case msg := <-ui.InboundProtocolStart:
			switch msg.Type {
			case messageTypeStartKeygen:
				othernicks := []string{}
				for _, s := range msg.StartKeygen.Signers {
					if s.Nick != ui.cfg.Me.Nick {
						othernicks = append(othernicks, s.Nick)
					}
				}
				confirmMsg := fmt.Sprintf("%s wants to generate a %v-of-%v wallet with other signers %s", msg.SenderName, msg.StartKeygen.Threshold+1, len(msg.StartKeygen.Signers), strings.Join(othernicks, ","))
				ui.confirm(confirmMsg, "Generate!", "main", func() {
					go ui.runProtocolKeygen(msg.StartKeygen.Name, msg.StartKeygen.Threshold, msg.StartKeygen.Signers)
				})
			case messageTypeStartSign:
				othernicks := []string{}
				for _, s := range msg.StartSign.Signers {
					if s.Nick != ui.cfg.Me.Nick {
						othernicks = append(othernicks, s.Nick)
					}
				}
				confirmMsg := fmt.Sprintf("%s wants %s to sign a text message with wallet %s: %s", msg.SenderName, strings.Join(othernicks, ","), msg.StartSign.Name, msg.StartSign.Message)
				ui.confirm(confirmMsg, "Sign!", "main", func() {
					hash := utils.DigestAvaMsg(msg.StartSign.Message)
					go ui.runProtocolSign(msg.StartSign.Name, hash, msg.StartSign.Signers)
				})
			case messageTypeStartSendTx:
				othernicks := []string{}
				for _, s := range msg.StartSendTx.Signers {
					if s.Nick != ui.cfg.Me.Nick {
						othernicks = append(othernicks, s.Nick)
					}
				}
				
				// TODO is division the best way to do this?
				amtDisplay := float64(msg.StartSendTx.Amount) / float64(units.Avax)
				confirmMsg := fmt.Sprintf("%s wants %s to send %v AVAX to address %s", msg.SenderName, strings.Join(othernicks, ","), amtDisplay, msg.StartSendTx.DestAddr)
				if msg.StartSendTx.Memo != "" {
					confirmMsg = fmt.Sprintf("%s with memo %s", confirmMsg, msg.StartSendTx.Memo)
				}
				log.Println(confirmMsg)
				ui.confirm(confirmMsg, "Sign!", "main", func() {

					w := ui.cfg.FindWallet(msg.StartSendTx.Name)
					err := w.FetchUTXOs()
					if err != nil {
						ui.MsgInputs <- fmt.Sprintf("Error fetching wallet balance %v", err)
					}

					_, _, b, err := formatting.ParseAddress(msg.StartSendTx.DestAddr)
					if err != nil {
						ui.MsgInputs <- fmt.Sprintf("Error parsing dest addr %s: %v", msg.StartSendTx.DestAddr, err)
					}
					destid, err := ids.ToShortID(b)
					if err != nil {
						ui.MsgInputs <- fmt.Sprintf("Error parsing dest addr to id %s: %v", msg.StartSendTx.DestAddr, err)
					}
					
					tx, err := w.CreateTx(w.Config.AssetID, msg.StartSendTx.Amount, destid, msg.StartSendTx.Memo)
					if err != nil {
						ui.MsgInputs <- fmt.Sprintf("Error CreateTx %v", err)
					}
					unsignedBytes, err := w.GetUnsignedBytes(&tx.UnsignedTx)
					if err != nil {
						ui.MsgInputs <- fmt.Sprintf("Error GetUnsignedBytes %v", err)
					}
					msgHash := hashing.ComputeHash256(unsignedBytes)
					
					go ui.runProtocolSign(msg.StartSendTx.Name, msgHash, msg.StartSendTx.Signers)
				})
			}
		case log := <-ui.Logs:
			ui.handleLogMessage(log)

		case <-refreshticker.C:
			ui.syncWallets()
			ui.syncParticipants()
			// TODO this causes flashing on windows
			ui.TerminalApp.Draw()

		case <-fetchticker.C:
			ui.fetchWalletBalances()

		case <-ui.psctx.Done():
			return
		}
	}
}

func (ui *UI) handleCommand(cmd UICommand) {
	switch cmd.cmdtype {
	case "/quit":
		ui.TerminalApp.Stop()
		return

	case "/keygen":
		ui.generateKeyForm()

	case "/signmsg":
		ui.signMsgForm()

	case "/sendtx":
		ui.sendTxForm()
		
	// Unsupported command
	default:
		ui.Logs <- chatlog{level: logLevelInfo, msg: fmt.Sprintf("unsupported command - %s", cmd.cmdtype)}
	}
}

func (ui *UI) displayChatMessage(msg chatmessage) {
	prompt := fmt.Sprintf("[blue]<%s>:[-]", msg.SenderName)
	fmt.Fprintf(ui.messageBox, "%s %s\n", prompt, msg.UserMessage)
}

func (ui *UI) displaySelfMessage(msg string) {
	prompt := fmt.Sprintf("[green]<%s>:[-]", ui.cfg.Me.Nick)
	fmt.Fprintf(ui.messageBox, "%s %s\n", prompt, msg)
}

func (ui *UI) handleLogMessage(cl chatlog) {
	// Write to UI
	fmt.Fprintf(ui.messageBox, "[grey]%s[-]\n", cl.msg)
	log.Printf("%s: %s", cl.level, cl.msg)
}

func (ui *UI) syncParticipants() {
	participants := ui.ParticipantList()

	// Clear() is not a threadsafe call
	ui.participantBox.Lock()
	ui.participantBox.Clear()
	ui.participantBox.Unlock()

	for _, p := range participants {
		fmt.Fprintf(ui.participantBox, "[blue]<%s>[-]\n[grey]%s[-]\n\n", p.Nick, p.Address)
	}
}

func (ui *UI) syncWallets() {
	// Clear() is not a threadsafe call
	ui.keyBox.Lock()
	ui.keyBox.Clear()
	ui.keyBox.Unlock()

	for _, n := range ui.cfg.SortedWalletNames() {
		w := ui.cfg.FindWallet(n)
		m := w.Threshold + 1
		n := len(w.Others) + 1
		signers := fmt.Sprint(strings.Join(w.AllPartyNicks(), ","))
		bal := w.BalanceForDisplay(w.Config.AssetID)
		fmt.Fprintf(
			ui.keyBox, 
			"[blue]<%s>[-]\n[yellow]%s[-]\n[white]Balance:[-] [green]%s[-] [white]AVAX[-]\n[grey]Signers: %s (%d of %d)\n", 
			w.Name, w.Address, bal, signers, m, n)
	}
}

func (ui *UI) fetchWalletBalances() {
	for _, n := range ui.cfg.SortedWalletNames() {
		w := ui.cfg.FindWallet(n)
		go w.FetchUTXOs()
	}
}

func (ui *UI) modal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewGrid().
		SetColumns(0, width, 0).
		SetRows(0, height, 0).
		AddItem(p, 1, 1, 1, 1, 0, 0, true)
}

func (ui *UI) globalKeyboardIntercept(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF2:
			ui.generateKeyForm()
		case tcell.KeyF3:
			ui.sendTxForm()
		case tcell.KeyF4:
			ui.signMsgForm()
		}
		// if event.Key() == tcell.KeyPgUp {
		// 	row, column := messagebox.GetScrollOffset()
		// 	messagebox.ScrollTo(row-10, column)
		// }

		return event
}

// func floatingModal(p tview.Primitive, width, height int) tview.Primitive {
// 	return tview.NewFlex().
// 		AddItem(nil, 0, 1, false).
// 		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
// 			AddItem(nil, 0, 1, false).
// 			AddItem(p, height, 1, false).
// 			AddItem(nil, 0, 1, false), width, 1, false).
// 		AddItem(nil, 0, 1, false)
// }
