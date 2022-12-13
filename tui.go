package ndog

import (
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/isobit/tview"
)

type TUI struct {
	delegate StreamFactory
	app      *tview.Application
	treeRoot *tview.TreeNode
	logView  *tview.TextView
	input    *tview.InputField
}

func NewTUI(delegate StreamFactory) *TUI {
	treeRoot := tview.NewTreeNode("all") // TODO set reference for multi writer
	tree := tview.NewTreeView().
		SetRoot(treeRoot).
		SetCurrentNode(treeRoot)
	tree.
		SetTitle("streams").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	logView := tview.NewTextView()
	logView.
		SetTitle("log").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	input := tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorBlack)
	input.
		SetTitle("input").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(
			tview.NewFlex().
				AddItem(tree, 0, 1, true).
				AddItem(logView, 0, 3, true),
			0, 1, true,
		).
		AddItem(
			input,
			3, 1, true,
		)

	app := tview.NewApplication().SetRoot(flex, true).SetFocus(flex)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		k := event.Key()
		r := event.Rune()
		switch {
		case k == tcell.KeyTab:
			switch app.GetFocus() {
			case logView:
				app.SetFocus(tree)
			case tree:
				app.SetFocus(logView)
			}
		case r == 'q':
			app.Stop()
		default:
			return event
		}
		return nil
	})

	logView.SetChangedFunc(func() {
		app.Draw()
	})

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		app.SetFocus(input)
	})

	getCurrentWriterCloser := func() (io.WriteCloser, bool) {
		node := tree.GetCurrentNode()
		if node == nil {
			return nil, false
		}
		rc, ok := node.GetReference().(io.WriteCloser)
		return rc, ok
	}

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			if writer, writerOk := getCurrentWriterCloser(); writerOk {
				text := input.GetText()
				// Logf(10, "input done; text=%s, ok=%v", text, writerOk)
				io.WriteString(writer, text+"\n")
			} else {
				Logf(-1, "failed to send input, no writer reference on node item")
			}
		}
		input.SetText("")
		if key == tcell.KeyEscape {
			app.SetFocus(tree)
		}
	})
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlD {
			if writer, writerOk := getCurrentWriterCloser(); writerOk {
				writer.Close()
				input.SetText("")
				app.SetFocus(tree)
			} else {
				Logf(-1, "failed to close writer, no writer reference on node item")
			}
		}
		return event
	})

	// // If a directory was selected, open it.
	// tree.SetSelectedFunc(func(node *tview.TreeNode) {
	// 	reference := node.GetReference()
	// 	if reference == nil {
	// 		return // Selecting the root node does nothing.
	// 	}
	// 	children := node.GetChildren()
	// 	if len(children) == 0 {
	// 		// Load and show files in this directory.
	// 		path := reference.(string)
	// 		add(node, path)
	// 	} else {
	// 		// Collapse if visible, expand if collapsed.
	// 		node.SetExpanded(!node.IsExpanded())
	// 	}
	// })
	return &TUI{
		delegate: delegate,
		app:      app,
		treeRoot: treeRoot,
		logView:  logView,
		input:    input,
	}
}

func (tui *TUI) NewStream(name string) Stream {
	node := tview.NewTreeNode(name).
		SetSelectable(true)

	var res Stream
	if tui.delegate != nil {
		stream := tui.delegate.NewStream(name)
		res = genericStream{
			Reader:          stream,
			Writer:          stream,
			CloseWriterFunc: stream.CloseWriter,
			CloseFunc: func() error {
				tui.app.QueueUpdateDraw(func() {
					tui.treeRoot.RemoveChild(node)
				})
				return stream.Close()
			},
		}
	} else {
		r, w := io.Pipe()
		node.SetReference(w)
		res = genericStream{
			Reader: r,
			Writer: io.Discard,
			CloseFunc: func() error {
				tui.app.QueueUpdateDraw(func() {
					tui.treeRoot.RemoveChild(node)
				})
				return nil
			},
		}
	}
	tui.app.QueueUpdateDraw(func() {
		tui.treeRoot.AddChild(node)
	})
	return res
}

func (tui *TUI) Logf(level int, format string, v ...interface{}) (int, error) {
	// if level > LogLevel {
	// 	return 0, nil
	// }
	if len(format) > 0 && format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	return fmt.Fprintf(tui.logView, format, v...)
}

func (tui *TUI) Run() error {
	return tui.app.Run()
}
