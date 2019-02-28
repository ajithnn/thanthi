package render

import (
	"fmt"
	"strings"

	"github.com/ajithnn/thanthi/app"
	"github.com/ajithnn/thanthi/logger"
	"github.com/jroimartin/gocui"
)

const (
	SIDE   = 0
	HEADER = 1
	MAIN   = 2
)

type Render struct {
	Handler     *gocui.Gui
	MailHandler *app.Mailer
	Views       []*gocui.View
	Params      *app.ComposeParams
	ViewButtons map[string][]string
	ButtonIndex int
}

func NewRenderer(mailer *app.Mailer) (*Render, error) {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		logger.NewLogger().Fatalf("NewRenderer#NewGui: %v", err)
		return &Render{}, err
	}
	return &Render{g, mailer, make([]*gocui.View, 0), &app.ComposeParams{}, make(map[string][]string), 0}, nil
}

func (r *Render) setParams(mode, to, bcc, cc, sub, body string) {
	r.Params = &app.ComposeParams{
		mode,
		to,
		bcc,
		cc,
		sub,
		body,
		"",
	}
}

func (r *Render) Show() error {
	r.Handler.Cursor = true
	r.Handler.SetManagerFunc(r.layout)
	if err := r.keybindings(); err != nil {
		logger.NewLogger().Fatalf("Render#Show: Key binding failed %v", err)
		return err
	}

	if err := r.Handler.MainLoop(); err != nil && err != gocui.ErrQuit {
		logger.NewLogger().Fatalf("Render#Show: Main loop failed %v", err)
		return err
	}
	return nil
}

func (r *Render) Close() {
	r.Handler.Close()
}

func (r *Render) loadMail(g *gocui.Gui, v *gocui.View) error {

	r.Views[MAIN].SetCursor(0, 0)

	g.Update(func(g *gocui.Gui) error {
		_, cy := v.Cursor()
		r.Views[MAIN].Clear()
		r.renderMailView(cy)
		if _, err := g.SetCurrentView("main"); err != nil {
			logger.NewLogger().Fatalf("Render#LoadMail: SetCurrentView Failed failed %v", err)
			return err
		}
		return nil
	})
	return nil
}

func (r *Render) nextPage(g *gocui.Gui) error {

	r.MailHandler.ListMail("next")
	r.renderHeader(g, "Messages")

	r.Views[SIDE].Clear()
	r.Views[MAIN].Clear()
	r.renderSideView()
	r.renderMailView(0)

	if _, err := g.SetCurrentView("side"); err != nil {
		logger.NewLogger().Fatalf("Render#NextPage: SetCurrentView Failed failed %v", err)
		return err
	}

	return nil
}

func (r *Render) prevPage(g *gocui.Gui) error {

	r.MailHandler.ListMail("prev")
	r.renderHeader(g, "Messages")

	r.Views[SIDE].Clear()
	r.Views[MAIN].Clear()
	r.renderSideView()
	r.renderMailView(0)

	if _, err := g.SetCurrentView("side"); err != nil {
		logger.NewLogger().Fatalf("Render#PrevPage: SetCurrentView Failed failed %v", err)
		return err
	}
	return nil
}

func (r *Render) initPage(g *gocui.Gui) error {
	r.MailHandler.ListMail("init")
	r.renderHeader(g, "Messages")

	r.Views[SIDE].Clear()
	r.Views[MAIN].Clear()
	r.renderSideView()
	r.renderMailView(0)

	if _, err := g.SetCurrentView("side"); err != nil {
		logger.NewLogger().Fatalf("Render#InitPage: SetCurrentView Failed failed %v", err)
		return err
	}
	return nil
}

func (r *Render) reloadPage(g *gocui.Gui) error {
	r.MailHandler.ListMail("reload")
	r.renderHeader(g, "Messages")

	r.Views[SIDE].Clear()
	r.Views[MAIN].Clear()
	r.renderSideView()
	r.renderMailView(0)

	if _, err := g.SetCurrentView("side"); err != nil {
		logger.NewLogger().Fatalf("Render#ReloadPage: SetCurrentView Failed failed %v", err)
		return err
	}
	return nil
}

func (r *Render) sendMail(g *gocui.Gui, v *gocui.View) error {
	var replyID string
	lines := v.BufferLines()
	for index, line := range lines {
		switch index {
		case 0:
			r.Params.To = line[strings.Index(line, ":")+1:]
		case 1:
			r.Params.Cc = line[strings.Index(line, ":")+1:]
		case 2:
			r.Params.Bcc = line[strings.Index(line, ":")+1:]
		case 3:
			r.Params.Subject = line[strings.Index(line, ":")+1:]
		case 4:
		default:
			r.Params.Body += line + "\n"
		}
	}
	_, cy := r.Views[SIDE].Cursor()
	curThread := r.MailHandler.Threads[cy]
	r.Params.ThreadID = curThread.ID
	for ind, msgs := range curThread.Messages {
		replyID += msgs.MessageID
		if ind != len(curThread.Messages)-1 {
			replyID += " "
		}
	}
	err := r.MailHandler.ComposeAndSend(r.Params, replyID)
	if err != nil {
		logger.NewLogger().Fatalf("Render#SendMail: Send Failed %v", err)
	}
	g.Update(r.renderCompose)
	return nil
}

func (r *Render) markReadWrapper(g *gocui.Gui) error {
	return r.markRead(g, r.Views[MAIN])
}

func (r *Render) markRead(g *gocui.Gui, v *gocui.View) error {
	_, cy := r.Views[SIDE].Cursor()
	_ = r.MailHandler.MarkAsRead(r.MailHandler.Threads[cy])
	g.Update(r.reloadPage)
	return nil
}

func (r *Render) scrollDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		v.Autoscroll = false
		ox, oy := v.Origin()
		if err := v.SetOrigin(ox, oy+1); err != nil {
			return nil
		}
	}
	return nil
}

func (r *Render) scrollUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		v.Autoscroll = false
		ox, oy := v.Origin()
		if err := v.SetOrigin(ox, oy-1); err != nil {
			return nil
		}
	}
	return nil
}

func (r *Render) moveToMainView(g *gocui.Gui, v *gocui.View) error {
	for _, button := range r.ViewButtons[v.Name()] {
		buttonView, _ := g.View(button)
		buttonView.Highlight = true
		buttonView.SelFgColor = gocui.ColorDefault
	}
	_, err := g.SetCurrentView("main")
	if err != nil {
		logger.NewLogger().Fatalf("Render#moveToMainView: SetCurrentView Failed failed %v", err)
		return err
	}
	return nil
}

func (r *Render) moveToSideView(g *gocui.Gui, v *gocui.View) error {
	for _, button := range r.ViewButtons[v.Name()] {
		buttonView, _ := g.View(button)
		buttonView.Highlight = true
		buttonView.SelFgColor = gocui.ColorDefault
	}
	_, err := g.SetCurrentView("side")
	if err != nil {
		logger.NewLogger().Fatalf("Render#moveToSideView: SetCurrentView Failed failed %v", err)
		return err
	}
	return nil
}

func (r *Render) moveToSideActionView(g *gocui.Gui, v *gocui.View) error {
	return r.moveToActionView("side-action", g, v)
}

func (r *Render) moveToMainActionView(g *gocui.Gui, v *gocui.View) error {
	return r.moveToActionView("mail-action", g, v)
}

func (r *Render) moveToActionView(viewname string, g *gocui.Gui, v *gocui.View) error {
	view, err := g.SetCurrentView(viewname)
	if err != nil {
		logger.NewLogger().Fatalf("Render#moveToActionView: SetCurrentView Failed %v", err)
		return err
	}
	r.ButtonIndex = -1
	return r.selectButton(g, view)
}

func (r *Render) selectButton(g *gocui.Gui, view *gocui.View) error {
	if r.ButtonIndex >= 0 {
		buttonName := r.ViewButtons[view.Name()][r.ButtonIndex]
		buttonView, _ := g.View(buttonName)
		buttonView.Highlight = true
		buttonView.SelFgColor = gocui.ColorDefault
	}
	r.ButtonIndex = (r.ButtonIndex + 1) % len(r.ViewButtons[view.Name()])
	buttonName := r.ViewButtons[view.Name()][r.ButtonIndex]
	buttonView, err := g.View(buttonName)
	if err != nil {
		logger.NewLogger().Fatalf("Render#selectButton: ButtonView Failed %v", err)
		return err
	}
	buttonView.Highlight = true
	buttonView.SelFgColor = gocui.ColorRed
	return nil
}

func (r *Render) mailSendWrapper(g *gocui.Gui) error {
	return r.mailSender(g, r.Views[MAIN])
}

func (r *Render) mailSender(g *gocui.Gui, v *gocui.View) error {
	_, cy := r.Views[SIDE].Cursor()
	thread := r.MailHandler.Threads[cy]
	msg := thread.Messages[len(thread.Messages)-1]
	r.setParams("reply", msg.From, msg.BCC, "", thread.Subject, "")
	g.Update(r.renderCompose)
	return nil
}

func (r *Render) handleButtonPress(g *gocui.Gui, view *gocui.View) error {
	buttonName := r.ViewButtons[view.Name()][r.ButtonIndex]
	//buttonView, _ := g.View(buttonName)
	switch buttonName {
	case "Next":
		g.Update(r.nextPage)
	case "Prev":
		g.Update(r.prevPage)
	case "Reply":
		g.Update(r.mailSendWrapper)
	case "MarkAsRead":
		g.Update(r.markReadWrapper)
	}

	for _, button := range r.ViewButtons[view.Name()] {
		buttonView, _ := g.View(button)
		buttonView.Highlight = true
		buttonView.SelFgColor = gocui.ColorDefault
	}

	return nil
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() == "side" {
		_, err := g.SetCurrentView("main")
		if err != nil {
			logger.NewLogger().Fatalf("Render#nextView: SetCurrentView Failed %v", err)
		}
		return err
	}
	_, err := g.SetCurrentView("side")
	if err != nil {
		logger.NewLogger().Fatalf("Render#nextView: SetCurrentView Failed %v", err)
	}
	return err
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (r *Render) keybindings() error {
	g := r.Handler

	// Side View Bindings

	if err := g.SetKeybinding("side", gocui.KeyCtrlSpace, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyEnter, gocui.ModNone, r.loadMail); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyPgup, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		r.renderHeader(g, "Loading....")
		g.Update(r.prevPage)
		return nil
	}); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyPgdn, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		r.renderHeader(g, "Loading....")
		g.Update(r.nextPage)
		return nil
	}); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyCtrlA, gocui.ModNone, r.moveToSideActionView); err != nil {
		return err
	}

	// Main View Bindings

	if err := g.SetKeybinding("main", gocui.KeyCtrlSpace, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowDown, gocui.ModNone, r.scrollDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowUp, gocui.ModNone, r.scrollUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyCtrlA, gocui.ModNone, r.moveToMainActionView); err != nil {
		return err
	}

	// All View Bindings

	if err := g.SetKeybinding("", gocui.KeyCtrlR, gocui.ModNone, r.markRead); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		g.Update(r.initPage)
		return nil
	}); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlH, gocui.ModNone, r.renderKeyBind); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlN, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		r.setParams("new", "", "", "", "", "")
		g.Update(r.renderCompose)
		return nil
	}); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlB, gocui.ModNone, r.mailSender); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}

	// Compose View Bindings

	if err := g.SetKeybinding("compose", gocui.KeyCtrlS, gocui.ModNone, r.sendMail); err != nil {
		return err
	}

	// action view bindings

	if err := g.SetKeybinding("mail-action", gocui.KeyCtrlA, gocui.ModNone, r.moveToMainView); err != nil {
		return err
	}
	if err := g.SetKeybinding("mail-action", gocui.KeyTab, gocui.ModNone, r.selectButton); err != nil {
		return err
	}
	if err := g.SetKeybinding("mail-action", gocui.KeyEnter, gocui.ModNone, r.handleButtonPress); err != nil {
		return err
	}

	if err := g.SetKeybinding("side-action", gocui.KeyCtrlA, gocui.ModNone, r.moveToSideView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side-action", gocui.KeyTab, gocui.ModNone, r.selectButton); err != nil {
		return err
	}
	if err := g.SetKeybinding("side-action", gocui.KeyEnter, gocui.ModNone, r.handleButtonPress); err != nil {
		return err
	}

	return nil
}

func (r *Render) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("side", -1, 1, maxX/3-10, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		r.Views = append(r.Views, v)
		r.renderSideView()
	}

	if v, err := g.SetView("side-top", -1, -1, maxX/3-10, 1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprintf(v, "\t\t\t\t\t\t\t\t%s", []byte("Subject"))
	}

	if v, err := g.SetView("mail-top", maxX/3-10, -1, maxX, 1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		r.Views = append(r.Views, v)
		r.renderHeader(g, "Messages")
	}

	if _, err := g.SetView("mail-action", maxX/3-10, maxY-4, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		r.renderButtons([]string{"Reply", "MarkAsRead"}, "mail-action", maxX/3-10, maxY-4, maxX, maxY, g)
	}

	if _, err := g.SetView("side-action", -1, maxY-4, maxX/3-10, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		r.renderButtons([]string{"Next", "Prev"}, "side-action", -1, maxY-4, maxX/3-10, maxY, g)
	}

	if v, err := g.SetView("main", maxX/3-10, 1, maxX, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		r.Views = append(r.Views, v)
		r.renderMailView(0) // Set index 0 for First Mail
		r.renderKeyBind(g, v)
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}
	}

	return nil
}

func (r *Render) renderCompose(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	_, err := g.View("compose")
	if err != nil {
		if view, err := g.SetView("compose", 0, 0, maxX, maxY); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			view.SetCursor(0, 0)
			fmt.Fprintf(view, "%s%s\n", "TO(comma-separated):", r.Params.To)
			fmt.Fprintf(view, "%s%s\n", "CC(comma-separated):", r.Params.Cc)
			fmt.Fprintf(view, "%s%s\n", "BCC(comma-separated):", r.Params.Bcc)
			fmt.Fprintf(view, "%s%s\n", "Subject:", r.Params.Subject)
			fmt.Fprintf(view, "%s%s\n", "Body(below):", r.Params.Body)
			view.Editable = true
			view.Wrap = true
			g.SetViewOnTop("compose")
			g.SetCurrentView("compose")
		}
		return nil
	}
	err = g.DeleteView("compose")
	if err != nil {
		return err
	}
	g.Update(func(g *gocui.Gui) error {
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}
		return nil
	})
	return nil
}

func (r *Render) renderKeyBind(g *gocui.Gui, _ *gocui.View) error {
	maxX, maxY := g.Size()
	_, err := g.View("top")
	if err != nil {
		if v, err := g.SetView("top", maxX/2-10, maxY/2-10, maxX/2+30, maxY/2+10); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			fmt.Fprintf(v, "%s\n", "---- Key Bindings ----")
			fmt.Fprintf(v, "%s\n\n", "")
			fmt.Fprintf(v, "%s\n", "---- From Anywhere ----")
			fmt.Fprintf(v, "%s\n", "Toggle help    - CTRL+H")
			fmt.Fprintf(v, "%s\n", "Change View     - CTRL+Space")
			fmt.Fprintf(v, "%s\n", "Load Mail      - CTRL+L")
			fmt.Fprintf(v, "%s\n", "Compose Mail      - CTRL+N")
			fmt.Fprintf(v, "%s\n\n", "Mark as Read   - CTRL+R")
			fmt.Fprintf(v, "%s\n\n", "Reply   - CTRL+B")
			fmt.Fprintf(v, "%s\n", "---- From Side View ----")
			fmt.Fprintf(v, "%s\n", "Next Page       - Pg Dn")
			fmt.Fprintf(v, "%s\n\n", "Prev Page      - Pg Up")
			fmt.Fprintf(v, "%s\n", "---- From Mail View ----")
			fmt.Fprintf(v, "%s\n", "Scroll Down    - Arrow Down")
			fmt.Fprintf(v, "%s\n", "Scroll Up      - Arrow Up")
			g.SetViewOnTop("top")
		}
		return nil
	}
	err = g.DeleteView("top")
	if err != nil {
		return err
	}
	g.Update(func(g *gocui.Gui) error {
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}
		return nil
	})
	return nil
}

func (r *Render) renderHeader(g *gocui.Gui, headerMsg string) error {
	r.Views[HEADER].Clear()
	r.Views[HEADER].Highlight = true
	fmt.Fprintf(r.Views[HEADER], "\t\t\t\t\t\t\t\t\t\t\t\t\t\t%s", headerMsg)
	if _, err := g.SetCurrentView("mail-top"); err != nil {
		return err
	}
	return nil
}

func (r *Render) renderMailView(index int) {
	if len(r.MailHandler.Threads) == 0 {
		fmt.Fprintln(r.Views[MAIN], "------------------")
		return
	}
	fmt.Fprintf(r.Views[MAIN], "Subject: %s\n", r.MailHandler.Threads[index].Subject)
	fmt.Fprintln(r.Views[MAIN], "========================================================================================================")
	for _, msg := range r.MailHandler.Threads[index].Messages {
		fmt.Fprintf(r.Views[MAIN], "%s:%s\n", "From", msg.From)
		fmt.Fprintf(r.Views[MAIN], "%s:%s\n", "CC", msg.CC)
		fmt.Fprintf(r.Views[MAIN], "%s:%s\n\n", "BCC", msg.BCC)
		fmt.Fprintf(r.Views[MAIN], "%s\n", msg.Body)
		fmt.Fprintf(r.Views[MAIN], "%s\n", []byte("-------------------------------------------------------------------------------------------------"))
	}
	r.Views[MAIN].Wrap = true
}

func (r *Render) renderSideView() {
	r.Views[SIDE].Highlight = true
	r.Views[SIDE].SelBgColor = gocui.ColorWhite
	r.Views[SIDE].SelFgColor = gocui.ColorRed
	if len(r.MailHandler.Threads) == 0 {
		fmt.Fprintln(r.Views[SIDE], "-----All Read----")
		return
	}
	for _, thread := range r.MailHandler.Threads {
		fmt.Fprintf(r.Views[SIDE], "%s\n", thread.Subject)
	}
}

func (r *Render) renderButtons(buttons []string, parentName string, minX, minY, maxX, maxY int, g *gocui.Gui) error {
	curMinX := minX + 1
	curMinY := minY + 1
	for _, buttonValue := range buttons {
		if bv, err := g.SetView(buttonValue, curMinX, curMinY, curMinX+len(buttonValue)+1, curMinY+2); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			fmt.Fprintf(bv, "%s", buttonValue)
			r.ViewButtons[parentName] = append(r.ViewButtons[parentName], buttonValue)
		}
		curMinX += len(buttonValue) + 3
		if curMinX >= maxX {
			break
		}
	}
	return nil
}
