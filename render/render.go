package render

import (
	"fmt"
	"strings"

	"github.com/ajithnn/thanthi/app"
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
	TopView     string
}

func NewRenderer(mailer *app.Mailer) (*Render, error) {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return &Render{}, err
	}
	return &Render{g, mailer, make([]*gocui.View, 0), ""}, nil
}

func (r *Render) Show() error {
	r.Handler.Cursor = true
	r.Handler.SetManagerFunc(r.layout)
	if err := r.keybindings(); err != nil {
		return err
	}

	if err := r.Handler.MainLoop(); err != nil && err != gocui.ErrQuit {
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
		return err
	}
	return nil
}

func (r *Render) sendMail(g *gocui.Gui, v *gocui.View) error {
	var to, cc, bcc, sub, body string
	lines := v.BufferLines()
	for index, line := range lines {
		switch index {
		case 1:
			to = line[strings.Index(line, ":")+1:]
		case 2:
			cc = line[strings.Index(line, ":")+1:]
		case 3:
			bcc = line[strings.Index(line, ":")+1:]
		case 4:
			sub = line[strings.Index(line, ":")+1:]
		case 5:
		case 0:
		default:
			body += line + "\n"
		}
	}
	err := r.MailHandler.SendMail(to, sub, cc, bcc, body)
	if err != nil {
		panic(err)
	}
	g.Update(r.renderCompose)
	return nil
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

func nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() == "side" {
		_, err := g.SetCurrentView("main")
		return err
	}
	_, err := g.SetCurrentView("side")
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
	if err := g.SetKeybinding("side", gocui.KeyCtrlSpace, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyCtrlSpace, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowDown, gocui.ModNone, r.scrollDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowUp, gocui.ModNone, r.scrollUp); err != nil {
		return err
	}
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
		g.Update(r.renderCompose)
		return nil
	}); err != nil {
		return err
	}
	if err := g.SetKeybinding("compose", gocui.KeyCtrlS, gocui.ModNone, r.sendMail); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
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
	return nil
}

func (r *Render) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("side", -1, 1, maxX/3-10, maxY); err != nil {
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

	if v, err := g.SetView("main", maxX/3-10, 1, maxX, maxY); err != nil {
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
			fmt.Fprintf(view, "%s\n", "--------------------Send - Ctrl+S--------------------")
			fmt.Fprintf(view, "%s\n", "TO(comma-separated):")
			fmt.Fprintf(view, "%s\n", "CC(comma-separated):")
			fmt.Fprintf(view, "%s\n", "BCC(comma-separated):")
			fmt.Fprintf(view, "%s\n", "Subject:")
			fmt.Fprintf(view, "%s\n", "Body(below):")
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
		if v, err := g.SetView("top", maxX/2-10, maxY/2-10, maxX/2+20, maxY/2+10); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			fmt.Fprintf(v, "%s\n", "---- Key Bindings ----")
			fmt.Fprintf(v, "%s\n\n", "")
			fmt.Fprintf(v, "%s\n", "---- From Anywhere ----")
			fmt.Fprintf(v, "%s\n", "Toggle help    - CTRL+H")
			fmt.Fprintf(v, "%s\n", "Change Vie     - CTRL+Space")
			fmt.Fprintf(v, "%s\n", "Load Mail      - CTRL+L")
			fmt.Fprintf(v, "%s\n", "Toggle Compose Mail      - CTRL+N")
			fmt.Fprintf(v, "%s\n\n", "Mark as Read   - CTRL+R")
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
	for _, msg := range r.MailHandler.Threads[index].Messages {
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
