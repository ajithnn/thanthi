package render

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/ajithnn/thanthi/app"
	"github.com/jroimartin/gocui"
)

type Render struct {
	Handler     *gocui.Gui
	MailHandler *app.Mailer
}

func NewRenderer(mailer *app.Mailer) (*Render, error) {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return &Render{}, err
	}
	return &Render{g, mailer}, nil
}

func (r *Render) Show() error {
	r.Handler.Cursor = true
	r.Handler.SetManagerFunc(r.layout)
	if err := keybindings(r.Handler); err != nil {
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

func getLine(g *gocui.Gui, v *gocui.View) error {
	var l string
	var err error

	_, cy := v.Cursor()
	if l, err = v.Line(cy); err != nil {
		l = ""
	}

	maxX, maxY := g.Size()
	if v, err := g.SetView("msg", maxX/2-30, maxY/2, maxX/2+30, maxY/2+2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprintln(v, l)
		if _, err := g.SetCurrentView("msg"); err != nil {
			return err
		}
	}
	return nil
}

func delMsg(g *gocui.Gui, v *gocui.View) error {
	if err := g.DeleteView("msg"); err != nil {
		return err
	}
	if _, err := g.SetCurrentView("side"); err != nil {
		return err
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func keybindings(g *gocui.Gui) error {
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
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyEnter, gocui.ModNone, getLine); err != nil {
		return err
	}
	if err := g.SetKeybinding("msg", gocui.KeyEnter, gocui.ModNone, delMsg); err != nil {
		return err
	}

	if err := g.SetKeybinding("main", gocui.KeyCtrlS, gocui.ModNone, saveMain); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyCtrlW, gocui.ModNone, saveVisualMain); err != nil {
		return err
	}
	return nil
}

func saveMain(g *gocui.Gui, v *gocui.View) error {
	f, err := ioutil.TempFile("", "gocui_demo_")
	if err != nil {
		return err
	}
	defer f.Close()

	p := make([]byte, 5)
	v.Rewind()
	for {
		n, err := v.Read(p)
		if n > 0 {
			if _, err := f.Write(p[:n]); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func saveVisualMain(g *gocui.Gui, v *gocui.View) error {
	f, err := ioutil.TempFile("", "gocui_demo_")
	if err != nil {
		return err
	}
	defer f.Close()

	vb := v.ViewBuffer()
	if _, err := io.Copy(f, strings.NewReader(vb)); err != nil {
		return err
	}
	return nil
}

func (r *Render) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	contents := make([]string, 0)
	if v, err := g.SetView("side", -1, -1, maxX/3-10, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		for _, content := range r.MailHandler.Messages {
			fmt.Fprintln(v, strings.Split(content, ":")[0])
			contents = append(contents, content)
		}
	}
	if v, err := g.SetView("main", maxX/3-10, -1, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprintf(v, "%s", []byte(contents[0]))
		v.Editable = true
		v.Wrap = true
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}
	}
	return nil
}
