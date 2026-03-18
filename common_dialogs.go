package vtui

// SelectDirDialog creates a standard directory selection dialog.
func SelectDirDialog(title string, initialPath string, vfs VFS) *Dialog {
	width := 50
	height := 18
	scrW := FrameManager.GetScreenSize()
	x1 := (scrW - width) / 2
	y1 := 4

	dlg := NewDialog(x1, y1, x1+width-1, y1+height-1, title)
	dlg.ShowClose = true

	pathEdit := NewEdit(x1+2, y1+2, width-4, initialPath)
	dlg.AddItem(pathEdit)

	// List of directories
	var items []string
	updateList := func(p string) {
		entries, _ := vfs.ReadDir(p)
		items = []string{".."}
		for _, e := range entries {
			if e.IsDir {
				items = append(items, e.Name)
			}
		}
	}
	updateList(vfs.GetPath())

	lb := NewListBox(x1+2, y1+4, width-4, height-8, items)
	dlg.AddItem(lb)

	lb.OnChange = func(idx int) {
		if idx < 0 || idx >= len(items) { return }
		selected := items[idx]
		var newPath string
		if selected == ".." {
			newPath = vfs.Dir(vfs.GetPath())
		} else {
			newPath = vfs.Join(vfs.GetPath(), selected)
		}

		if err := vfs.SetPath(newPath); err == nil {
			updateList(vfs.GetPath())
			lb.Items = items
			lb.SelectPos = 0
			lb.TopPos = 0
			pathEdit.SetText(vfs.GetPath())
		}
	}

	btnOk := NewButton(x1+10, y1+height-2, "&Ok")
	btnOk.OnClick = func() { dlg.SetExitCode(1) }
	dlg.AddItem(btnOk)

	btnCancel := NewButton(x1+width-20, y1+height-2, "&Cancel")
	btnCancel.OnClick = func() { dlg.SetExitCode(-1) }
	dlg.AddItem(btnCancel)

	FrameManager.Push(dlg)
	return dlg
}