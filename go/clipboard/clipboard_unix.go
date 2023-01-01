package clipboard

import (
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"go.f110.dev/xerrors"
)

type Clipboard struct {
	x   *xgb.Conn
	win xproto.Window

	text string

	clipboardAtom     xproto.Atom
	stringAtom        xproto.Atom
	utf8StringAtom    xproto.Atom
	textAtom          xproto.Atom
	textPlainAtom     xproto.Atom
	textPlainUtf8Atom xproto.Atom

	targetAtom  xproto.Atom
	atomAtom    xproto.Atom
	targetAtoms []xproto.Atom
}

func New() (*Clipboard, error) {
	return new()
}

func (c *Clipboard) Set(v string) error {
	ssoc := xproto.SetSelectionOwnerChecked(c.x, c.win, c.clipboardAtom, xproto.TimeCurrentTime)
	if err := ssoc.Check(); err != nil {
		return xerrors.WithMessage(err, "setting clipboard")
	}
	c.text = v

	return nil
}

func new() (*Clipboard, error) {
	xServer := os.Getenv("DISPLAY")
	if xServer == "" {
		return nil, xerrors.New("could not identify xserver")
	}
	x, err := xgb.NewConnDisplay(xServer)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	win, err := xproto.NewWindowId(x)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	setup := xproto.Setup(x)
	s := setup.DefaultScreen(x)
	err = xproto.CreateWindowChecked(x, s.RootDepth, win, s.Root, 100, 100, 1, 1, 0, xproto.WindowClassInputOutput, s.RootVisual, 0, []uint32{}).Check()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	clipboardAtom := internAtom(x, "CLIPBOARD")
	stringAtom := internAtom(x, "STRING")
	utf8StringAtom := internAtom(x, "UTF8_STRING")
	textAtom := internAtom(x, "TEXT")
	textPlainAtom := internAtom(x, "text/plain")
	textPlainUtf8Atom := internAtom(x, "text/plain;charset=utf-8")
	targetsAtom := internAtom(x, "TARGETS")
	atomAtom := internAtom(x, "ATOM")

	c := &Clipboard{
		x:                 x,
		win:               win,
		clipboardAtom:     clipboardAtom,
		stringAtom:        stringAtom,
		utf8StringAtom:    utf8StringAtom,
		textAtom:          textAtom,
		textPlainAtom:     textPlainAtom,
		textPlainUtf8Atom: textPlainUtf8Atom,
		targetAtom:        targetsAtom,
		atomAtom:          atomAtom,
		targetAtoms:       []xproto.Atom{targetsAtom, stringAtom, textAtom, textPlainAtom, utf8StringAtom, textPlainUtf8Atom},
	}
	go c.eventLoop()
	return c, nil
}

func (c *Clipboard) eventLoop() {
	for {
		event, err := c.x.WaitForEvent()
		if err != nil {
			log.Print(err.Error())
			continue
		}

		switch e := event.(type) {
		case xproto.SelectionRequestEvent:
			switch e.Target {
			case c.utf8StringAtom, c.textPlainUtf8Atom, c.textAtom, c.textPlainAtom, c.stringAtom:
				cpc := xproto.ChangePropertyChecked(
					c.x,
					xproto.PropModeReplace,
					e.Requestor,
					e.Property,
					e.Target,
					8,
					uint32(len(c.text)),
					[]byte(c.text),
				)
				if cpc.Check() == nil {
					c.sendSelectionNotify(e)
				} else {
					fmt.Fprintln(os.Stderr, err)
				}
			case c.targetAtom:
				buf := make([]byte, len(c.targetAtoms)*4)
				for i, atom := range c.targetAtoms {
					xgb.Put32(buf[i*4:], uint32(atom))
				}

				cpc := xproto.ChangePropertyChecked(
					c.x,
					xproto.PropModeReplace,
					e.Requestor,
					e.Property,
					c.atomAtom,
					32,
					uint32(len(c.targetAtoms)),
					buf,
				)
				if cpc.Check() == nil {
					c.sendSelectionNotify(e)
				} else {
					fmt.Fprintln(os.Stderr, err)
				}
			default:
				log.Printf("unknown target: %v", e.Target)
			}
		default:
			log.Printf("unknown event: %v", e)
		}
	}
}

func (c *Clipboard) sendSelectionNotify(e xproto.SelectionRequestEvent) {
	sn := xproto.SelectionNotifyEvent{
		Time:      e.Time,
		Requestor: e.Requestor,
		Selection: e.Selection,
		Target:    e.Target,
		Property:  e.Property}
	sec := xproto.SendEventChecked(c.x, false, e.Requestor, xproto.EventMaskNoEvent, string(sn.Bytes()))
	err := sec.Check()
	if err != nil {
		fmt.Println(err)
	}
}

func internAtom(conn *xgb.Conn, n string) xproto.Atom {
	iac := xproto.InternAtom(conn, true, uint16(len(n)), n)
	iar, err := iac.Reply()
	if err != nil {
		panic(err)
	}
	return iar.Atom
}
